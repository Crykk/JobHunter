package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/Crykk/JobHunter/internal/openrouter"
	"github.com/Crykk/JobHunter/internal/profile"
)

// DefaultFile is the settings JSON name (gitignored; lives in the app data dir).
const DefaultFile = "jobhunter.settings.json"

// File is persisted model choices and optional API key (gitignored).
type File struct {
	FilterModel string `json:"filter_model"`
	RankModel   string `json:"rank_model"`
	APIKey      string `json:"api_key,omitempty"`
}

// Default is Luna Decisions + Gemini 4 Argon.
func Default() File {
	return File{
		FilterModel: openrouter.LunaModel,
		RankModel:   openrouter.ArgonModel,
	}
}

// Path is the local settings file. Upgrades replace only the binary, not this file.
func Path() string {
	if p := strings.TrimSpace(os.Getenv("JOBHUNTER_SETTINGS")); p != "" {
		return p
	}
	return filepath.Join(profile.Dir(), DefaultFile)
}

// Load reads Path(), or Default if missing/invalid.
func Load() File {
	p := Path()
	if _, err := os.Stat(p); err == nil {
		return LoadPath(p)
	}
	if migrated := migrateCwd(p); migrated {
		return LoadPath(p)
	}
	return Default()
}

func migrateCwd(dest string) bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	old := filepath.Join(cwd, DefaultFile)
	if old == dest {
		return false
	}
	raw, err := os.ReadFile(old)
	if err != nil || len(raw) == 0 {
		return false
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return false
	}
	if err := os.WriteFile(dest, raw, 0o600); err != nil {
		return false
	}
	return true
}

// LoadPath reads path.
func LoadPath(path string) File {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Default()
	}
	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		return Default()
	}
	return Normalize(f)
}

// Save writes Path(). An empty API key does not wipe a stored key.
func Save(f File) error {
	return SavePath(Path(), f)
}

// SavePath writes path. Creates the parent dir. Does not replace a stored API key with empty.
func SavePath(path string, f File) error {
	f = Normalize(f)
	if f.APIKey == "" {
		if existing := LoadPath(path); existing.APIKey != "" {
			f.APIKey = existing.APIKey
		}
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

// Normalize pins known aliases and forces rank to Argon for now.
func Normalize(f File) File {
	f.FilterModel = NormalizeFilter(f.FilterModel)
	f.RankModel = openrouter.ArgonModel
	f.APIKey = strings.TrimSpace(f.APIKey)
	return f
}

// ResolveAPIKey returns OPENROUTER_API_KEY if set, else the saved settings key.
func ResolveAPIKey() string {
	if k := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY")); k != "" {
		return k
	}
	return strings.TrimSpace(Load().APIKey)
}

// NormalizeFilter maps a user/id alias to an OpenRouter Decisions model.
func NormalizeFilter(id string) string {
	s := strings.ToLower(strings.TrimSpace(id))
	switch s {
	case "", "luna", "luna-6", "luna-decisions", "gpt-6-luna-decisions", openrouter.LunaModel:
		return openrouter.LunaModel
	case "jev", "jev-1.14", "jev-latest", "typesafe/jev-latest", "~typesafe/jev-latest", openrouter.JevModel:
		return openrouter.JevModel
	default:
		return openrouter.LunaModel
	}
}

// FilterLabel is the short UI name for a first-pass model.
func FilterLabel(id string) string {
	if NormalizeFilter(id) == openrouter.JevModel {
		return "Jev"
	}
	return "Luna 6 Decisions"
}

// RankLabel is the short UI name for the second-pass model.
func RankLabel() string {
	return "Gemini 4 Argon"
}

// FilterLogName is a short log prefix.
func FilterLogName(id string) string {
	if NormalizeFilter(id) == openrouter.JevModel {
		return "jev"
	}
	return "luna"
}
