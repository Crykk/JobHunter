# Install jobhunter onto PATH (Windows).
# Usage: irm https://raw.githubusercontent.com/<org>/jobhunter/main/install.ps1 | iex
$ErrorActionPreference = "Stop"

$Repo = if ($env:JOBHUNTER_REPO) { $env:JOBHUNTER_REPO } else { "Crykk/JobHunter" }
$InstallDir = if ($env:JOBHUNTER_INSTALL_DIR) { $env:JOBHUNTER_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "jobhunter" }
$Module = if ($env:JOBHUNTER_MODULE) { $env:JOBHUNTER_MODULE } else { "github.com/Crykk/JobHunter/cmd/jobhunter" }

switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { $Arch = "x86_64" }
    "ARM64" { $Arch = "arm64" }
    default { throw "unsupported arch: $($env:PROCESSOR_ARCHITECTURE)" }
}

$Asset = "jobhunter_Windows_${Arch}.zip"
$Api = "https://api.github.com/repos/$Repo/releases/latest"

function Install-FromGo {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "No GitHub Release found for $Repo and Go is not on PATH. Create a release or install Go and retry."
    }
    Write-Host "No GitHub Release asset $Asset; falling back to go install ${Module}@latest"
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $env:GOBIN = $InstallDir
    & go install "${Module}@latest"
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

try {
    $release = Invoke-RestMethod -Uri $Api -Headers @{ "User-Agent" = "jobhunter-install" }
    $url = ($release.assets | Where-Object { $_.name -eq $Asset } | Select-Object -First 1).browser_download_url
    if (-not $url) { throw "asset missing" }
    $zip = Join-Path $env:TEMP $Asset
    Write-Host "Downloading $url"
    Invoke-WebRequest -Uri $url -OutFile $zip -UseBasicParsing
    $extract = Join-Path $env:TEMP "jobhunter-extract"
    if (Test-Path $extract) { Remove-Item -Recurse -Force $extract }
    Expand-Archive -Path $zip -DestinationPath $extract -Force
    $exe = Get-ChildItem -Path $extract -Recurse -Filter "jobhunter.exe" | Select-Object -First 1
    if (-not $exe) { throw "zip did not contain jobhunter.exe" }
    # Replace only the binary. Never overwrite profile.txt or jobhunter.settings.json.
    Copy-Item $exe.FullName (Join-Path $InstallDir "jobhunter.exe") -Force
    $example = Get-ChildItem -Path $extract -Recurse -Filter "profile.example.txt" | Select-Object -First 1
    $profile = Join-Path $InstallDir "profile.txt"
    if ($example -and -not (Test-Path $profile)) {
        Copy-Item $example.FullName $profile
        Write-Host "Created $profile — edit it in JobHunter Settings"
    }
    elseif (Test-Path $profile) {
        Write-Host "Kept existing $profile"
    }
    $settings = Join-Path $InstallDir "jobhunter.settings.json"
    if (Test-Path $settings) {
        Write-Host "Kept existing $settings"
    }
}
catch {
    Write-Host $_.Exception.Message
    Install-FromGo
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (-not $userPath) { $userPath = "" }
$parts = $userPath -split ";" | Where-Object { $_ -ne "" }
if ($parts -notcontains $InstallDir) {
    [Environment]::SetEnvironmentVariable("Path", ($userPath.TrimEnd(";") + ";" + $InstallDir), "User")
    $env:Path = $env:Path + ";" + $InstallDir
    Write-Host "Added $InstallDir to your user PATH. Open a new terminal if jobhunter is not found."
}

Write-Host "Installed $(Join-Path $InstallDir 'jobhunter.exe')"
Write-Host 'Then run: jobhunter   (same tab UI). Scripts: jobhunter search -q "python" -a "Helsinki"'
