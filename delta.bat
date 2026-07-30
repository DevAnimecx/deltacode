@echo off
REM ============================================================================
REM  Δ Delta Code — Launcher for Windows CMD
REM  Auto-builds if binary doesn't exist. First run launches setup wizard.
REM ============================================================================
setlocal enabledelayedexpansion

set DELTA_DIR=%~dp0
set DELTA_EXE=%DELTA_DIR%delta.exe

REM Auto-build if binary is missing
if not exist "%DELTA_EXE%" (
    echo [Δ] Building Delta Code...
    if not exist "%DELTA_DIR%go.mod" (
        echo [✗] Error: Not a Go project directory. Clone the repo first:
        echo     git clone https://github.com/DevAnimecx/deltacode.git
        pause
        exit /b 1
    )
    cd /d "%DELTA_DIR%"
    go build -o delta.exe . >nul 2>&1
    if !errorlevel! neq 0 (
        echo [✗] Build failed. Install Go from https://go.dev/dl/
        pause
        exit /b 1
    )
    echo [✓] Build complete!
)

REM Run Delta
"%DELTA_EXE%" %*
exit /b %errorlevel%
