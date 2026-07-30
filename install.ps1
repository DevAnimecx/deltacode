<#
.SYNOPSIS
    Δ Delta Code — One-Line Installer
.DESCRIPTION
    Downloads and installs Delta Code in one command.
    
    One-liner:
        irm https://raw.githubusercontent.com/DevAnimecx/deltacode/master/install.ps1 | iex
    
    Or download and run:
        git clone https://github.com/DevAnimecx/deltacode.git
        cd deltacode
        .\run.ps1
.LINK
    https://github.com/DevAnimecx/deltacode
#>

$ErrorActionPreference = "Stop"
$Repo = "DevAnimecx/deltacode"
$Branch = "master"
$InstallDir = "$env:USERPROFILE\.delta-bin"
$DeltaExe = "$InstallDir\delta.exe"

$C = @{ Cyan = "Cyan"; Green = "Green"; Yellow = "Yellow"; Red = "Red"; Magenta = "Magenta" }

function Write-Logo {
    Clear-Host
    Write-Host @"
      ██████╗ ███████╗██╗  ████████╗ █████╗
      ██╔══██╗██╔════╝██║  ╚══██╔══╝██╔══██╗
      ██║  ██║█████╗  ██║     ██║   ███████║
      ██║  ██║██╔══╝  ██║     ██║   ██╔══██║
      ██████╔╝███████╗███████╗██║   ██║  ██║
      ╚═════╝ ╚══════╝╚══════╝╚═╝   ╚═╝  ╚═╝
"@ -ForegroundColor $C.Cyan
}

function Install-Delta {
    Write-Logo
    Write-Host "╔══════════════════════════════════════════════╗" -ForegroundColor $C.Cyan
    Write-Host "║       Δ Delta Code — One-Click Install       ║" -ForegroundColor $C.Cyan
    Write-Host "╚══════════════════════════════════════════════╝" -ForegroundColor $C.Cyan
    Write-Host ""

    # Step 1: Check prerequisites
    Write-Host "  [1/5] Checking prerequisites..." -ForegroundColor $C.Yellow
    
    $hasGit = $null -ne (Get-Command "git" -ErrorAction SilentlyContinue)
    $hasGo = $null -ne (Get-Command "go" -ErrorAction SilentlyContinue)
    
    if (-not $hasGit -and -not $hasGo) {
        Write-Host "  ⚠ Neither Git nor Go found." -ForegroundColor $C.Yellow
        Write-Host "  Downloading pre-built binary instead..." -ForegroundColor $C.Cyan
        Install-PreBuilt
        return
    }
    Write-Host "  ✓ Git: $(if($hasGit) {'found'} else {'not found'}) | Go: $(if($hasGo) {'found'} else {'not found'})" -ForegroundColor $C.Green

    # Step 2: Create install directory
    Write-Host "  [2/5] Creating install directory..." -ForegroundColor $C.Yellow
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Write-Host "  ✓ $InstallDir" -ForegroundColor $C.Green

    # Step 3: Download
    Write-Host "  [3/5] Downloading Delta Code..." -ForegroundColor $C.Yellow
    $ZipUrl = "https://github.com/$Repo/archive/refs/heads/$Branch.zip"
    $ZipPath = "$env:TEMP\delta-code.zip"
    
    try {
        Invoke-WebRequest -Uri $ZipUrl -OutFile $ZipPath -UseBasicParsing
        Write-Host "  ✓ Downloaded" -ForegroundColor $C.Green
        Expand-Archive -Path $ZipPath -DestinationPath "$env:TEMP\delta-code-extract" -Force
        $BuildDir = Get-ChildItem "$env:TEMP\delta-code-extract" -Directory | Select-Object -First 1 -ExpandProperty FullName
    } catch {
        Write-Host "  ✗ Download failed. Cloning instead..." -ForegroundColor $C.Yellow
        git clone --depth 1 "https://github.com/$Repo.git" "$env:TEMP\delta-code-src" 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Write-Host "  ✗ Clone failed. Install Go from https://go.dev/dl/ then re-run." -ForegroundColor $C.Red
            exit 1
        }
        $BuildDir = "$env:TEMP\delta-code-src"
    }

    # Step 4: Build
    Write-Host "  [4/5] Building binary..." -ForegroundColor $C.Yellow
    
    if (Test-Path $DeltaExe) { Remove-Item $DeltaExe -Force }
    
    Push-Location $BuildDir
    go build -o "$InstallDir\delta.exe" . 2>&1
    Pop-Location
    
    # Cleanup temp files
    Remove-Item "$env:TEMP\delta-code.zip" -ErrorAction SilentlyContinue
    Remove-Item "$env:TEMP\delta-code-extract" -Recurse -ErrorAction SilentlyContinue
    Remove-Item "$env:TEMP\delta-code-src" -Recurse -ErrorAction SilentlyContinue

    if (-not (Test-Path $DeltaExe)) {
        Write-Host "`r  ✗ Build failed. Install Go from https://go.dev/dl/ and re-run." -ForegroundColor $C.Red
        exit 1
    }
    
    $size = (Get-Item $DeltaExe).Length / 1MB
    Write-Host "`r  ✓ Built: $([math]::Round($size, 1)) MB" -ForegroundColor $C.Green

    # Step 5: Install
    Install-ToPath
}

function Install-PreBuilt {
    $InstallDir = "$env:USERPROFILE\.delta-bin"
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    
    $OsPlatform = "windows"
    $Arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
    
    Write-Host "  [3/5] Downloading binary for ${OsPlatform}-${Arch}..." -ForegroundColor $C.Yellow
    $BinaryUrl = "https://github.com/$Repo/releases/latest/download/delta-${OsPlatform}-${Arch}.exe"
    
    try {
        Invoke-WebRequest -Uri $BinaryUrl -OutFile $DeltaExe -UseBasicParsing
        Write-Host "  ✓ Downloaded" -ForegroundColor $C.Green
    } catch {
        Write-Host "  ✗ Binary download failed. Try installing Go: https://go.dev/dl/" -ForegroundColor $C.Red
        Write-Host "  Then re-run this installer." -ForegroundColor $C.Yellow
        exit 1
    }
    
    Install-ToPath
}

function Install-ToPath {
    Write-Host "  [5/5] Adding to PATH..." -ForegroundColor $C.Yellow
    
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$InstallDir*") {
        $newPath = "$userPath;$InstallDir"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        $env:Path = "$env:Path;$InstallDir"
    }
    Write-Host "  ✓ Added to PATH" -ForegroundColor $C.Green
    
    Write-Host ""
    Write-Host "╔══════════════════════════════════════════════╗" -ForegroundColor $C.Green
    Write-Host "║       🎉 Delta Code is installed!            ║" -ForegroundColor $C.Green
    Write-Host "╚══════════════════════════════════════════════╝" -ForegroundColor $C.Green
    Write-Host ""
    Write-Host "  Binary: $DeltaExe" -ForegroundColor $C.Cyan
    Write-Host ""
    Write-Host "  Quick start:" -ForegroundColor $C.Yellow
    Write-Host "    delta                      # Setup wizard (first run)" -ForegroundColor $C.White
    Write-Host "    delta run 'hello world'    # Generate code" -ForegroundColor $C.White
    Write-Host "    delta doctor               # System check" -ForegroundColor $C.White
    Write-Host "    delta fix 'sort is broken' # Autonomous fix" -ForegroundColor $C.White
    Write-Host ""
    Write-Host "  Restart your terminal or run:" -ForegroundColor $C.Yellow
    Write-Host "    $env:Path = `"$env:Path;$InstallDir`"" -ForegroundColor $C.White
    Write-Host ""
}

Install-Delta
