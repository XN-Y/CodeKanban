@echo off
setlocal enabledelayedexpansion
set SCRIPT_DIR=%~dp0
set BUNDLE_DIR=%SCRIPT_DIR%
set CODEX_HOME_DIR=%CODEX_HOME%
if "%CODEX_HOME_DIR%"=="" set CODEX_HOME_DIR=%USERPROFILE%\.codex
set SKILLS_DIR=%CODEX_HOME_DIR%\skills

echo Installing codekanban-cli from the offline bundle...
call npm install -g --no-fund --no-audit "%BUNDLE_DIR%npm\codekanban-cli-__CLI_VERSION__.tgz"
if errorlevel 1 exit /b 1

echo Installing Codex skills into %SKILLS_DIR% ...
if not exist "%SKILLS_DIR%" mkdir "%SKILLS_DIR%"
xcopy /E /I /Y "%BUNDLE_DIR%skills\*" "%SKILLS_DIR%\" >nul
if errorlevel 1 exit /b 1

echo.
echo Installation complete. Restart Codex to discover the new skills.
