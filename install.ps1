# Butler installer for Windows — detects architecture, downloads binary, adds to PATH.
# Usage: irm https://raw.githubusercontent.com/cytivibe/butler/main/install.ps1 | iex

$ErrorActionPreference = "Stop"

$repo = "cytivibe/butler"
$installDir = "$env:LOCALAPPDATA\butler"

# Detect architecture
$binary = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "butler-windows-arm64.exe" } else { "butler-windows-x64.exe" }
} else {
    Write-Error "32-bit Windows is not supported."
    exit 1
}

# Get latest release tag
$release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
$latest = $release.tag_name
if (-not $latest) {
    Write-Error "Could not determine latest release."
    exit 1
}

$url = "https://github.com/$repo/releases/download/$latest/$binary"

# Download
Write-Host "Downloading butler $latest ($binary)..."
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
$dest = "$installDir\butler.exe"

# Kill any running butler processes so the file isn't locked
$procs = Get-Process -Name "butler" -ErrorAction SilentlyContinue
if ($procs) {
    Write-Host "Stopping running butler processes..."
    $procs | Stop-Process -Force
    Start-Sleep -Seconds 2
}

# Retry download in case file lock takes a moment to release
$attempts = 0
$maxAttempts = 3
while ($attempts -lt $maxAttempts) {
    try {
        Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing
        break
    } catch [System.IO.IOException] {
        $attempts++
        if ($attempts -eq $maxAttempts) {
            Write-Error "Cannot write to $dest — file is locked. Close any applications using butler and try again."
            exit 1
        }
        Write-Host "File locked, retrying in 3 seconds..."
        Start-Sleep -Seconds 3
    }
}

# Add to PATH if not already there
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
    Write-Host "Added $installDir to user PATH."
}

Write-Host "butler $latest installed to $dest"
Write-Host ""
Write-Host "To set up with Claude Code, run:"
Write-Host "  claude mcp add butler --scope user -- cmd /c `"$dest`" serve"
Write-Host ""
Write-Host "Restart your terminal for PATH changes to take effect."
