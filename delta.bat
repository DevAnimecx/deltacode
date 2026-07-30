@echo off
REM delta.bat — Delta Code Launcher for CMD
REM Usage: delta <command> [args]

set DELTA_DIR=C:\Users\india\OneDrive\Documents\Delta Code
set DELTA_EXE=%DELTA_DIR%\delta.exe

if not exist "%DELTA_EXE%" (
    echo Delta Code binary not found at %DELTA_EXE%
    echo Run 'go build -o delta.exe .' in %DELTA_DIR% first.
    exit /b 1
)

"%DELTA_EXE%" %*
