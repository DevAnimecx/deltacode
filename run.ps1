<#
.SYNOPSIS
    Δ Delta Code — PowerShell Launcher
.DESCRIPTION
    Auto-builds if binary missing. First run launches the interactive setup wizard.
    Use this script to run Delta Code from anywhere.
.EXAMPLE
    .\run.ps1                       # Launch TUI or setup wizard
    .\run.ps1 run "hello world"     # Generate code
    .\run.ps1 doctor                # System check
    .\run.ps1 provider add          # Add an AI provider
#>

param(
    [Parameter(Position=0)]
    [string]$Command = "",
    
    [Parameter(ValueFromRemainingArguments=$true)]
    [string[]]$Arguments
)

$DeltaDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$DeltaExe = Join-Path $DeltaDir "delta.exe"

# Colors
$Cyan = "Cyan"
$Green = "Green"
$Yellow = "Yellow"
$Red = "Red"

# Auto-build if binary is missing
if (-not (Test-Path $DeltaExe)) {
    Write-Host "Δ Building Delta Code..." -ForegroundColor $Cyan
    
    if (-not (Test-Path (Join-Path $DeltaDir "go.mod"))) {
        Write-Host "✗ Error: Not a Go project directory." -ForegroundColor $Red
        Write-Host "  Clone the repo first:" -ForegroundColor $Yellow
        Write-Host "  git clone https://github.com/DevAnimecx/deltacode.git" -ForegroundColor White
        pause
        exit 1
    }
    
    Push-Location $DeltaDir
    $build = Start-Process -FilePath "go" -ArgumentList "build -o delta.exe ." -NoNewWindow -Wait -PassThru
    Pop-Location
    
    if (-not (Test-Path $DeltaExe)) {
        Write-Host "✗ Build failed. Install Go from https://go.dev/dl/" -ForegroundColor $Red
        pause
        exit 1
    }
    Write-Host "✓ Build complete!" -ForegroundColor $Green
}

# Show help if no args
if ([string]::IsNullOrEmpty($Command)) {
    & $DeltaExe
    exit $LASTEXITCODE
}

# Run Delta with arguments
$allArgs = @($Command) + $Arguments
& $DeltaExe $allArgs
exit $LASTEXITCODE
