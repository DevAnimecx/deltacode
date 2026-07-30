# run.ps1 — Delta Code Helper Script
# Usage: .\run.ps1 <command> [args]

param(
    [Parameter(Position=0)]
    [string]$Command = "help",
    
    [Parameter(ValueFromRemainingArguments=$true)]
    [string[]]$Args
)

$deltaDir = "C:\Users\india\OneDrive\Documents\Delta Code"
$deltaExe = Join-Path $deltaDir "delta.exe"

if (!(Test-Path $deltaExe)) {
    Write-Host "Delta Code binary not found at $deltaExe" -ForegroundColor Red
    Write-Host "Run 'cd $deltaDir && go build -o delta.exe .' first" -ForegroundColor Yellow
    exit 1
}

# Build command line
$allArgs = @($Command) + $Args
$joinedArgs = $allArgs -join ' '

# Run delta
& $deltaExe $allArgs

# Helpful exit
if ($LASTEXITCODE -ne 0 -and $Command -eq "help") {
    Write-Host ""
    Write-Host "Quick commands:" -ForegroundColor Cyan
    Write-Host "  .\run.ps1 doctor                    # Check system health"
    Write-Host "  .\run.ps1 provider add              # Add an AI provider"
    Write-Host "  .\run.ps1 run ""write hello world""  # Generate code"
    Write-Host "  .\run.ps1 fix ""bug description""    # Autonomous fix"
    Write-Host ""
    Write-Host "Or add delta to PATH and just run: delta <command>" -ForegroundColor Green
}
