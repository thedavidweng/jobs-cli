# jobs-cli installer for Windows
# Usage: powershell -ExecutionPolicy ByPass -c "irm https://raw.githubusercontent.com/thedavidweng/jobs-cli/main/install.ps1 | iex"
# Uninstall: powershell -ExecutionPolicy ByPass -c "& ([scriptblock]::Create((irm https://raw.githubusercontent.com/thedavidweng/jobs-cli/main/install.ps1))) uninstall"

$ErrorActionPreference = "Stop"
$Repo = "thedavidweng/jobs-cli"
$Binary = "jobs-cli"
$DefaultInstallDir = Join-Path $env:LOCALAPPDATA "jobs-cli\bin"

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

function Get-InstallDir {
    if ($env:JOBS_INSTALL_DIR) { $env:JOBS_INSTALL_DIR } else { $DefaultInstallDir }
}

function Test-PathEntry($pathValue, $dir) {
    $dir = $dir.TrimEnd("\")
    foreach ($entry in ("$pathValue" -split ";")) {
        if ($entry -and [Environment]::ExpandEnvironmentVariables($entry).TrimEnd("\") -eq $dir) { return $true }
    }
    return $false
}

# The user Path goes through the registry directly because
# [Environment]::SetEnvironmentVariable expands %VARIABLES% and saves the
# value as REG_SZ.
function Get-UserPath {
    $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey("Environment")
    if ($null -eq $key) { return "" }
    try {
        [string]($key.GetValue("Path", "", [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames))
    } finally {
        $key.Close()
    }
}

function Set-UserPath($value) {
    $key = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey("Environment")
    try {
        if (-not $value) {
            $key.DeleteValue("Path", $false)
        } else {
            $kind = if ($key.GetValueNames() -contains "Path") {
                $key.GetValueKind("Path")
            } else {
                [Microsoft.Win32.RegistryValueKind]::ExpandString
            }
            $key.SetValue("Path", $value, $kind)
        }
    } finally {
        $key.Close()
    }

    # Tell Explorer the environment changed, so terminals opened from now on
    # get the new Path without signing out.
    if (-not ("JobsCli.NativeMethods" -as [Type])) {
        Add-Type -Namespace JobsCli -Name NativeMethods -MemberDefinition @'
[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]
public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);
'@
    }
    $HWND_BROADCAST = [IntPtr]0xffff
    $WM_SETTINGCHANGE = 0x1a
    $SMTO_ABORTIFHUNG = 2
    $result = [UIntPtr]::Zero
    [JobsCli.NativeMethods]::SendMessageTimeout($HWND_BROADCAST, $WM_SETTINGCHANGE, [UIntPtr]::Zero, "Environment", $SMTO_ABORTIFHUNG, 5000, [ref]$result) | Out-Null
}

# Main install
function Install-JobsCli {
    # Windows PowerShell downloads noticeably slower while drawing a progress bar.
    $ProgressPreference = "SilentlyContinue"
    Step "Installing jobs-cli ($platformLabel)"

    $version = Resolve-Version
    Step "Latest version: $version"

    $asset = "${Binary}_windows_${arch}.zip"
    $url = "https://github.com/$Repo/releases/download/$version/$asset"

    $installDir = (New-Item -ItemType Directory -Path (Get-InstallDir) -Force).FullName
    $tmpDir = Join-Path $env:TEMP "jobs-cli-install-$([guid]::NewGuid().ToString('N').Substring(0,8))"
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

    $needsRestart = $false
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

        # Like install.sh, leave PATH alone when the directory is already on it.
        if (-not (Test-PathEntry $env:Path $installDir)) {
            $userPath = Get-UserPath
            if (-not (Test-PathEntry $userPath $installDir)) {
                Set-UserPath $(if ($userPath) { "$installDir;$userPath" } else { $installDir })
                Step "Added $installDir to your user PATH"
            }
            $env:Path = "$installDir;$env:Path"
            $needsRestart = $true
        }

        $versionOutput = & (Join-Path $installDir "${Binary}.exe") --version 2>$null
        Step "Installed $versionOutput"
    } finally {
        Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }

    Write-Host ""
    if ($needsRestart) {
        Step "Restart your terminal to use jobs-cli."
    }
    Step "Run 'jobs-cli doctor' to check your setup."
    Step "Run 'jobs-cli --help' to see available commands."
}

function Uninstall-JobsCli {
    $installDir = Get-InstallDir
    $exe = Join-Path $installDir "${Binary}.exe"
    if (Test-Path $exe) {
        Step "Removing $exe"
        Remove-Item $exe -Force
    }
    # Only the default directory belongs to jobs-cli. A custom JOBS_INSTALL_DIR
    # may hold other tools, so its PATH entry and folder stay.
    if (-not $env:JOBS_INSTALL_DIR) {
        $userPath = Get-UserPath
        if (Test-PathEntry $userPath $installDir) {
            Set-UserPath ((($userPath -split ";") | Where-Object { -not (Test-PathEntry $_ $installDir) }) -join ";")
            Step "Removed $installDir from your user PATH"
        }
        foreach ($dir in $installDir, (Split-Path $installDir)) {
            if ((Test-Path $dir) -and -not (Get-ChildItem -Force $dir)) {
                Remove-Item $dir -Force
            }
        }
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
}
