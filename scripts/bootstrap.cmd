@echo off
setlocal EnableExtensions

set "ROOT_DIR=%~dp0.."
for %%I in ("%ROOT_DIR%") do set "ROOT_DIR=%%~fI"
set "BIN_PATH=%ROOT_DIR%\deeph.exe"

where go >nul 2>nul
if errorlevel 1 (
  echo error: Go not found. Install Go 1.24+ first.
  exit /b 1
)

echo Building deeph.exe...
go build -o "%BIN_PATH%" "%ROOT_DIR%\cmd\deeph"
if errorlevel 1 exit /b 1

echo.
echo Built: %BIN_PATH%
echo.
echo Quick start ^(Windows CMD^):
echo   mkdir C:\Users\%%USERNAME%%\deeph-workspace
echo   cd C:\Users\%%USERNAME%%\deeph-workspace
echo   "%BIN_PATH%" quickstart --deepseek
echo   set DEEPSEEK_API_KEY=sk-...your_real_key...
echo   "%BIN_PATH%" run guide "hello"
echo.
echo Tip: use scripts\quickstart.cmd for a one-command workspace bootstrap.

endlocal
