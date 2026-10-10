# windows dev base: git, gh, claude code, and the git/claude config shared
# with the nix hosts. idempotent; re-run to apply changes.
#
#   irm https://raw.githubusercontent.com/ojaskavathe/dots/master/windows/setup.ps1 | iex
#
# run from a clone it uses that clone; piped in, it clones to ~/dots first.
# no admin needed (winget prompts for UAC itself where an installer wants it).
$ErrorActionPreference = "Stop"

$packages = @(
    "Git.Git"
    "GitHub.cli"
)

function Update-Path {
    $env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" +
                [Environment]::GetEnvironmentVariable("Path", "User")
}

# deep-merge $over into $base (PSCustomObjects from ConvertFrom-Json), like
# jq's `.[0] * .[1]`: nested objects merge, everything else is replaced
function Merge-Object($base, $over) {
    foreach ($p in $over.PSObject.Properties) {
        $existing = $base.PSObject.Properties[$p.Name]
        if ($existing -and $existing.Value -is [pscustomobject] -and $p.Value -is [pscustomobject]) {
            Merge-Object $existing.Value $p.Value
        } else {
            $base | Add-Member -NotePropertyName $p.Name -NotePropertyValue $p.Value -Force
        }
    }
}

# --- packages ---
foreach ($id in $packages) {
    winget list --id $id -e --accept-source-agreements *> $null
    if ($LASTEXITCODE -eq 0) {
        Write-Host "${id}: installed"
        continue
    }
    Write-Host "${id}: installing..."
    winget install --id $id -e --silent --accept-package-agreements --accept-source-agreements
    if ($LASTEXITCODE -ne 0) { throw "winget install $id failed (exit $LASTEXITCODE)" }
}
Update-Path

# claude code uses the native installer rather than winget: it self-updates
# and winget's manifest lags behind
if (Get-Command claude -ErrorAction SilentlyContinue) {
    Write-Host "claude: installed"
} else {
    Write-Host "claude: installing..."
    Invoke-RestMethod https://claude.ai/install.ps1 | Invoke-Expression
    Update-Path
}

# --- dots ---
if ($PSScriptRoot -and (Test-Path (Join-Path $PSScriptRoot "..\flake.nix"))) {
    $dots = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
} else {
    $dots = Join-Path $HOME "dots"
    if (-not (Test-Path $dots)) {
        # https so a fresh machine can clone before it has an ssh key; origin is
        # then pointed at ssh for pushing once the key is on github
        git clone https://github.com/ojaskavathe/dots.git $dots
        if ($LASTEXITCODE -ne 0) { throw "git clone failed" }
        git -C $dots remote set-url origin git@github.com:ojaskavathe/dots.git
    }
}
Write-Host "dots: $dots"

# --- git ---
# ~/.gitconfig stays a normal mutable file; it just includes the shared one
$gitInclude = (Join-Path $dots "modules\home\gitconfig") -replace '\\', '/'
# git errors (on stderr, which 5.1 turns into a terminating error) on a missing
# global config, so make sure there is one
$gitconfig = Join-Path $HOME ".gitconfig"
if (-not (Test-Path $gitconfig)) { New-Item -ItemType File -Path $gitconfig | Out-Null }
$includes = @(git config --global --get-all include.path)
if ($includes -contains $gitInclude) {
    Write-Host "git: include already set"
} else {
    git config --global --add include.path $gitInclude
    Write-Host "git: included $gitInclude"
}

# --- claude ---
# same merge claude.nix does on activation: shared keys win, everything claude
# wrote at runtime is kept
$claudeDir = Join-Path $HOME ".claude"
$settingsPath = Join-Path $claudeDir "settings.json"
New-Item -ItemType Directory -Path $claudeDir -Force | Out-Null

# ReadAllText/WriteAllText rather than Get-Content/Set-Content: 5.1 reads
# BOM-less files as ANSI and writes UTF-8 with a BOM
$settings = [pscustomobject]@{}
if ((Test-Path $settingsPath) -and (Get-Item $settingsPath).Length -gt 0) {
    $settings = [IO.File]::ReadAllText($settingsPath) | ConvertFrom-Json
}
$managed = [IO.File]::ReadAllText((Join-Path $dots "modules\home\claude-settings.json")) | ConvertFrom-Json
Merge-Object $settings $managed
[IO.File]::WriteAllText($settingsPath, ($settings | ConvertTo-Json -Depth 100), (New-Object Text.UTF8Encoding $false))
Write-Host "claude: merged settings into $settingsPath"

# --- auth ---
$ErrorActionPreference = "Continue" # gh reports logged-out on stderr
gh auth status *> $null
$ghStatus = $LASTEXITCODE
$ErrorActionPreference = "Stop"
if ($ghStatus -ne 0) { Write-Host "`ngh isn't logged in: run 'gh auth login'" }

Write-Host "`nDone."
