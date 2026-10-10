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
    "jqlang.jq"  # claude status line
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

# --- developer mode ---
# lets unelevated processes (git, mainly) create symlinks. HKLM, so this one
# step elevates; it's skipped once set
$devModeKey = "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock"
if ((Get-ItemProperty $devModeKey -ErrorAction SilentlyContinue).AllowDevelopmentWithoutDevLicense -eq 1) {
    Write-Host "developer mode: on"
} else {
    Write-Host "developer mode: enabling (UAC prompt)..."
    # not New-Item -Force: on the registry that recreates an existing key, values and all
    $cmd = "if (-not (Test-Path '$devModeKey')) { New-Item -Path '$devModeKey' | Out-Null }; " +
           "Set-ItemProperty -Path '$devModeKey' -Name AllowDevelopmentWithoutDevLicense -Value 1 -Type DWord"
    Start-Process powershell -Verb RunAs -Wait -ArgumentList "-NoProfile", "-Command", $cmd
    if ((Get-ItemProperty $devModeKey -ErrorAction SilentlyContinue).AllowDevelopmentWithoutDevLicense -ne 1) {
        throw "developer mode wasn't enabled (UAC declined?)"
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
# windows-only: keep files byte-for-byte as committed (git for windows' system
# config defaults to autocrlf=true, which breaks shell scripts) and check out
# real symlinks (needs developer mode, below)
git config --global core.autocrlf false
git config --global core.symlinks true
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
# the status line script is shared with claude.nix; claude runs it under Git Bash
$statusScript = (Join-Path $dots "modules\home\claude-statusline.sh") -replace '\\', '/'
Merge-Object $settings ([pscustomobject]@{ statusLine = [pscustomobject]@{ type = "command"; command = "bash `"$statusScript`"" } })
[IO.File]::WriteAllText($settingsPath, ($settings | ConvertTo-Json -Depth 100), (New-Object Text.UTF8Encoding $false))
Write-Host "claude: merged settings into $settingsPath"

# --- font ---
# the same Nerd Font stylix uses on the nix hosts (JetBrainsMonoNL Nerd Font
# Mono), from the official release, pinned by version and sha256. Installed
# per user, so no admin; re-run after bumping the version.
$fontVersion = "v3.5.1"
$fontSha256 = "fab782a66f7d3019da64f6572db9fc5d3a4bcb19f9fa13e2d8a62e3693d6396e"
$fontFamily = "JetBrainsMonoNL Nerd Font Mono"
# the name Windows registers (legacy family name); the long typographic name
# above is what stylix uses, but terminal font lookup goes by this one
$fontFace = "JetBrainsMonoNL NFM"
$fontDir = Join-Path $env:LOCALAPPDATA "Microsoft\Windows\Fonts"
$fontMarker = Join-Path $fontDir ".jetbrainsmono-nerdfont-version"
$fontRegKey = "HKCU:\Software\Microsoft\Windows NT\CurrentVersion\Fonts"
if ((Test-Path $fontMarker) -and ((Get-Content $fontMarker -Raw).Trim() -eq $fontVersion)) {
    Write-Host "font: $fontFamily $fontVersion installed"
} else {
    Write-Host "font: installing $fontFamily $fontVersion..."
    $zip = Join-Path $env:TEMP "JetBrainsMono-$fontVersion.zip"
    $extract = Join-Path $env:TEMP "JetBrainsMono-$fontVersion"
    Invoke-WebRequest "https://github.com/ryanoasis/nerd-fonts/releases/download/$fontVersion/JetBrainsMono.zip" -OutFile $zip -UseBasicParsing
    if ((Get-FileHash $zip -Algorithm SHA256).Hash.ToLower() -ne $fontSha256) { throw "font zip sha256 mismatch" }
    if (Test-Path $extract) { Remove-Item $extract -Recurse -Force }
    Expand-Archive $zip -DestinationPath $extract
    New-Item -ItemType Directory -Path $fontDir -Force | Out-Null
    if (-not (Test-Path $fontRegKey)) { New-Item -Path $fontRegKey | Out-Null }
    foreach ($ttf in Get-ChildItem $extract -Filter "JetBrainsMonoNLNerdFontMono-*.ttf") {
        $dest = Join-Path $fontDir $ttf.Name
        Copy-Item $ttf.FullName $dest -Force
        Set-ItemProperty -Path $fontRegKey -Name "$($ttf.BaseName) (TrueType)" -Value $dest
    }
    Set-Content -Path $fontMarker -Value $fontVersion -Encoding ascii
    Remove-Item $zip, $extract -Recurse -Force
    Write-Host "font: installed (restart apps to see it)"
}

# Windows Terminal: use it for every profile. settings.json is JSON with
# comments allowed; skip rather than mangle it if it doesn't parse
$wtSettings = Join-Path $env:LOCALAPPDATA "Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json"
if (Test-Path $wtSettings) {
    try {
        $wt = [IO.File]::ReadAllText($wtSettings) | ConvertFrom-Json
        if (-not $wt.profiles.defaults) { $wt.profiles | Add-Member -NotePropertyName defaults -NotePropertyValue ([pscustomobject]@{}) -Force }
        Merge-Object $wt.profiles.defaults ([pscustomobject]@{ font = [pscustomobject]@{ face = $fontFace } })
        [IO.File]::WriteAllText($wtSettings, ($wt | ConvertTo-Json -Depth 100), (New-Object Text.UTF8Encoding $false))
        Write-Host "terminal: font set to $fontFace"
    } catch {
        Write-Host "terminal: settings.json didn't parse as plain JSON; set the font to '$fontFace' by hand"
    }
}

# --- auth ---
$ErrorActionPreference = "Continue" # gh reports logged-out on stderr
gh auth status *> $null
$ghStatus = $LASTEXITCODE
$ErrorActionPreference = "Stop"
if ($ghStatus -ne 0) { Write-Host "`ngh isn't logged in: run 'gh auth login'" }

Write-Host "`nDone."
