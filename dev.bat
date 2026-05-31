@echo off
setlocal EnableExtensions
chcp 65001 >nul

set "ROOT_DIR=%~dp0"
cd /d "%ROOT_DIR%"

if /I "%~1"=="help" goto :usage
if /I "%~1"=="-h" goto :usage
if /I "%~1"=="--help" goto :usage

if not exist "%ROOT_DIR%bat\dev.bat" (
    echo [ERROR] Missing bat\dev.bat
    if /I not "%NO_PAUSE%"=="1" if /I not "%CI%"=="1" pause
    exit /b 1
)

call "%ROOT_DIR%bat\dev.bat" %*
exit /b %ERRORLEVEL%

:usage
echo Usage:
echo   dev.bat [stable^|live^|limited] [--no-pause]
echo.
echo Modes:
echo   stable   Default. Build frontend static assets and start Wails without Vite dev server.
echo   live     Start the frontend dev server and connect Wails to it.
echo   limited  Same as live, with Windows memory limits for the frontend watcher.
echo.
echo Examples:
echo   dev.bat
echo   dev.bat live
echo   dev.bat limited --no-pause
exit /b 0
