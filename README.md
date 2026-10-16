# JobHunter

A terminal command: after install you type `jobhunter`. In a terminal it opens a **same-tab UI** (menu, mouse, query box). It searches [Duunitori](https://duunitori.fi/tyopaikat) from **your** IP, filters listings with OpenRouter Luna Decisions, then ranks survivors with Gemini 4 Argon.

Agents and contributors: read [AGENTS.md](AGENTS.md) for what/how/why and constraints.

Install is **one line**. No Node or Python runtime.

## Install

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/Crykk/JobHunter/main/install.ps1 | iex
```

macOS / Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/Crykk/JobHunter/main/install.sh | sh
```

The scripts download the matching GitHub Release binary onto PATH (`%LOCALAPPDATA%\jobhunter` on Windows, `~/.local/bin` on Unix). Reinstall replaces only `jobhunter` / `jobhunter.exe`. Your API key, model choice, and `profile.txt` stay in the app data folder.

Override the GitHub repo if you forked:

```bash
JOBHUNTER_REPO=you/JobHunter curl -fsSL https://raw.githubusercontent.com/you/JobHunter/main/install.sh | sh
```

Or, with Go installed:

```bash
go install github.com/Crykk/JobHunter/cmd/jobhunter@latest
```

## Setup

1. On first run (and on Windows install) JobHunter creates `profile.txt` if it is missing: `%LOCALAPPDATA%\jobhunter\profile.txt` on Windows, `~/.local/share/jobhunter/profile.txt` on Unix. If `./profile.txt` already exists, that file is used instead. Open it from **Settings → Open in editor** (Notepad/default app) or **Open folder**.

2. OpenRouter API key: type `jobhunter` and **paste the key** when asked (hidden). It is saved in `%LOCALAPPDATA%\jobhunter\jobhunter.settings.json` on Windows or `~/.local/share/jobhunter/jobhunter.settings.json` on Unix (gitignored). Change it later under **Settings**. Optional: set `OPENROUTER_API_KEY` to override. A new install does not overwrite this file.

## Usage

In Windows Terminal, PowerShell, or any TTY, type `jobhunter` (no flags). The UI stays in **that** tab: click **New search**, type a query, Enter. Click a result to open it in your default browser. **Settings** (`ctrl+,`) chooses the first-pass model (Luna 6 Decisions or Jev); the second pass is Gemini 4 Argon. `ctrl+q` quits. Choices and the API key are saved in the app data `jobhunter.settings.json` (gitignored) and survive upgrades.

Scripts and pipes still use the non-interactive command:

```bash
jobhunter search --query "python" --area "Helsinki" --pages 3
```

If stdout is not a terminal, `jobhunter` with no args prints help instead of the UI.

`search` flags:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--query` / `-q` | (required) | Duunitori `haku=` |
| `--area` / `-a` | empty | Duunitori `alue=` |
| `--pages` / `-p` | `3` | Max list pages (20 jobs per page, `sivu`) |
| `--threshold` | `0.55` | Minimum Luna `noul` score to keep a job |
| `--profile` | `profile.txt` | Candidate profile |
| `--out` | `output/matches.json` | Ranked JSON |

Progress goes to stderr. A ranked table is printed on stdout. JSON is written to `--out`.

Pipeline:

1. `GET https://duunitori.fi/tyopaikat?haku=...&alue=...&sivu=N` — parse list cards only (id, title, company, location, posted, url). Dedup by numeric id from `/tyopaikat/tyo/{slug}-{id}`. Stop on last page (`ceil(total/20)`) or HTTP 404.
2. Stage 1 — `POST https://openrouter.ai/api/alpha/decisions` with `openai/gpt-6-luna-decisions` or `typesafe/jev-1.14` (Settings). One `noul` question per job (`suitable_{id}`).
3. Fetch detail pages for surviving IDs; strip HTML to text.
4. Stage 2 — `POST https://openrouter.ai/api/v1/chat/completions` with model `google/gemini-4-argon`. JSON ranking (`score` 0–100 + `why`).

Requests use plain `net/http` from the machine that runs the CLI (honors `HTTP_PROXY` / `HTTPS_PROXY`). There is a ~0.8s delay between Duunitori fetches and a browser-like User-Agent. Cloudflare / HTTP 403 / 429 fail with: run from a Finnish/Nordic home IP. That is expected on many cloud VMs.

## Disclaimer

Users must follow [Duunitori’s terms of service](https://duunitori.fi/) and applicable law. Duunitori `robots.txt` currently has `User-agent: *` / `Disallow: /`. This tool is for personal use from your own residential connection. Do not run it against Duunitori from datacenter IPs, do not hammer the site, and do not use it if you are not allowed to fetch those pages.

## Development

Requires Go 1.24+.

```bash
go test ./...
go build -o jobhunter ./cmd/jobhunter
```

Parser tests use anonymized HTML in `testdata/` (no live Duunitori in CI). OpenRouter clients are covered with `httptest`.

Releases: push a `v*` tag; `.github/workflows/release.yml` runs GoReleaser for Windows/macOS/Linux amd64 and arm64.

## License

MIT
