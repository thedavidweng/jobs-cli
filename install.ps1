# jobs-cli installer for Windows
# Usage: powershell -ExecutionPolicy ByPass -c "irm https://raw.githubusercontent.com/thedavidweng/jobs-cli/main/install.ps1 | iex"

$ErrorActionPreference = "Stop"
$Repo = "thedavidweng/jobs-cli"
$Binary = "jobs-cli"

function Step($msg)  { Write-Host "==> $msg" }
function Die($msg)   { Write-Error "ERROR: $msg"; exit 1 }

# Detect architecture
$arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "x86_64" }
} else {
    Die "32-bit Windows is not supported."
}

$platformLabel = "windows/$arch"

# Resolve latest version
function Resolve-Version {
    $resp = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    return $resp.tag_name
}

# Main install
function Install-JobsCli {
    Step "Installing jobs-cli ($platformLabel)"

    $version = Resolve-Version
    Step "Latest version: $version"

    $asset = "${Binary}_windows_${arch}.zip"
    $url = "https://github.com/$Repo/releases/download/$version/$asset"

    $installDir = if ($env:JOBS_INSTALL_DIR) { $env:JOBS_INSTALL_DIR } else {
        Join-Path $env:LOCALAPPDATA "jobs-cli\bin"
    }

    $tmpDir = Join-Path $env:TEMP "jobs-cli-install-$([guid]::NewGuid().ToString('N').Substring(0,8))"
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null

    try {
        Step "Downloading $asset"
        $zipPath = Join-Path $tmpDir $asset
        Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing

        Step "Installing to $installDir"
        Expand-Archive -Path $zipPath -DestinationPath $tmpDir -Force
        $exe = Join-Path $tmpDir "${Binary}.exe"
        if (-not (Test-Path $exe)) {
            Die "Could not find $Binary.exe in archive."
        }
        Copy-Item $exe (Join-Path $installDir "${Binary}.exe") -Force

        # Add to PATH if needed
        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if ($userPath -notlike "*$installDir*") {
            [Environment]::SetEnvironmentVariable("Path", "$installDir;$userPath", "User")
            $env:Path = "$installDir;$env:Path"
            Step "Added $installDir to user PATH"
        }

        $versionOutput = & (Join-Path $installDir "${Binary}.exe") --version 2>$null
        Step "Installed $versionOutput"
    } finally {
        Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Uninstall-JobsCli {
    $installDir = if ($env:JOBS_INSTALL_DIR) { $env:JOBS_INSTALL_DIR } else {
        Join-Path $env:LOCALAPPDATA "jobs-cli\bin"
    }
    $exe = Join-Path $installDir "${Binary}.exe"
    if (Test-Path $exe) {
        Step "Removing $exe"
        Remove-Item $exe -Force
    }
    $configDir = if ($env:JOBS_CONFIG_DIR) { $env:JOBS_CONFIG_DIR } else {
        Join-Path $env:APPDATA "jobs-cli"
    }
    Step "Uninstalled. You may also remove jobs-cli config and sessions from $configDir\"
}

# Entry point
if ($args.Count -gt 0 -and $args[0] -eq "uninstall") {
    Uninstall-JobsCli
} else {
    Install-JobsCli
    Write-Host ""
    Step "Run 'jobs-cli doctor' to check your setup."
    Step "Run 'jobs-cli --help' to see available commands."
}
