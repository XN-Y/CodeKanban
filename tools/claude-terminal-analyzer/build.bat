@echo off
echo Building Claude Terminal Analyzer...
go build -o claude-terminal-analyzer.exe -ldflags="-s -w" .
if %errorlevel% == 0 (
    echo Build successful! Executable: claude-terminal-analyzer.exe
) else (
    echo Build failed!
    exit /b 1
)
