# JobHunter — agent context

Read this before changing the repo. Human-facing install/run docs are in `README.md`.

## What this is

JobHunter is a **local CLI** (`jobhunter`) that opens a Bubble Tea TUI in the current terminal when you type `jobhunter` with no args (TTY). `jobhunter search` stays non-interactive. The pipeline:

1. Fetches Finnish job **list cards** from [Duunitori](https://duunitori.fi/tyopaikat) over plain HTTP.
2. Cheap-filters titles against `profile.txt` with OpenRouter **Luna Decisions**.
3. Fetches **full ads** only for survivors.
4. Ranks them with OpenRouter **Gemini 4 Argon**.

It is personal, MIT, open-source. It is **not** a web app, SaaS, scraper-as-a-service, or browser automation.

Public GitHub (when published): https://github.com/Crykk/JobHunter  
This Project also has a private Origin copy; GitHub is the public home.

## Why these choices

- **Go, single binary** — install with `irm | iex` / `curl | sh` and no Node/Python runtime.
- **User’s own IP** — Duunitori often Cloudflare-challenges datacenter and non-FI IPs. No bundled proxy. Honor `HTTP_PROXY`/`HTTPS_PROXY` if the user sets them. Fail 403/429/CF HTML with a clear “run from a Finnish/Nordic home IP” error; **do not** solve Cloudflare challenges.
- **Two AI stages** — list pages are large and cheap to filter; full HTML is expensive. Stage 1 is OpenRouter Decisions: `openai/gpt-6-luna-decisions` or `typesafe/jev-1.14` (TUI Settings). Stage 2 is `google/gemini-4-argon` (chat JSON: score + why). One OpenRouter key, no local models. The TUI asks you to paste the key if missing; env `OPENROUTER_API_KEY` overrides. Persist models + key in `jobhunter.settings.json` (gitignored).
- **`profile.txt`** — created on first run in the app data dir (`%LOCALAPPDATA%\jobhunter` / `~/.local/share/jobhunter`) from `profile.example.txt`. `./profile.txt` wins if present. TUI Settings opens it in the OS editor. Gitignore local copies; commit only `profile.example.txt`.
- **robots.txt** — Duunitori `User-agent: *` / `Disallow: /`. Keep the README disclaimer. Do not add aggressive parallelism or unlimited crawl.

## How it is implemented

| Path | Role |
| --- | --- |
| `cmd/jobhunter` | Binary entry |
| `internal/cli` | Cobra root (TUI on TTY) + `search` (`--query`, `--area`, `--pages`, `--threshold`, `--profile`, `--out`) |
| `internal/pipeline` | Shared Duunitori → Luna → Argon run (CLI + TUI) |
| `internal/tui` | Bubble Tea UI in the current tab (mouse, menu, settings, results) |
| `internal/settings` | `jobhunter.settings.json` — first-pass Luna/Jev, second-pass Argon, optional API key |
| `internal/profile` | Create/open `profile.txt` (app data dir + Settings) |
| `internal/httpx` | `net/http` client, ~0.8s delay, browser-like User-Agent |
| `internal/scraper` | Search pagination (`sivu=`, 20/page) and detail fetch |
| `internal/parse` | goquery list cards + detail text; Cloudflare HTML detection |
| `internal/openrouter` | Decisions + chat HTTP |
| `internal/filter` | Luna: `state` = profile + jobs; one `noul` per job `suitable_{id}`; keep if `noul >= --threshold` (default 0.55); batch ≤ ~100 jobs |
| `internal/rank` | Argon JSON: `score` 0–100 and short `why` |
| `internal/output` | stderr progress, stdout table, `--out` JSON (default `output/matches.json`) |
| `install.ps1` / `install.sh` | Release asset, else `go install` |
| `testdata/` | Anonymized HTML fixtures — **no live Duunitori in CI** |

Job IDs come from `/tyopaikat/tyo/{slug}-{id}`. Dedup across pages.

## Do not

- Add a browser/Playwright/Chrome scraper.
- Add a proxy vendor SDK.
- Add Ink, a second TUI toolkit, or a UI that opens a new window. The default `jobhunter` command is already Bubble Tea in the current tab.
- Add auth, a database, or extra services.
- Hit Duunitori from tests or CI.
- Commit real `profile.txt`, `jobhunter.settings.json`, or API keys.

## Commands

```bash
export OPENROUTER_API_KEY=sk-or-...
cp profile.example.txt profile.txt
go test ./...
go run ./cmd/jobhunter
go run ./cmd/jobhunter search --query "python" --area "Helsinki" --pages 3
```

Module path: `github.com/Crykk/JobHunter`.
