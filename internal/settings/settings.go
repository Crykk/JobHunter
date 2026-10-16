package settings

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/Crykk/JobHunter/internal/openrouter"
)

// DefaultFile is the working-directory settings JSON (gitignored).
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

// Load reads DefaultFile, or Default if missing/invalid.
func Load() File {
	return LoadPath(DefaultFile)
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

// Save writes DefaultFile.
func Save(f File) error {
	return SavePath(DefaultFile, f)
}

// SavePath writes path.
func SavePath(path string, f File) error {
	f = Normalize(f)
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
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
