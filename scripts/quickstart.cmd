@echo off
setlocal EnableExtensions

set "ROOT_DIR=%~dp0.."
for %%I in ("%ROOT_DIR%") do set "ROOT_DIR=%%~fI"
set "BIN_PATH=%ROOT_DIR%\deeph.exe"

if "%~1"=="" (
  set "WORKSPACE=%USERPROFILE%\deeph-workspace"
) else (
  set "WORKSPACE=%~1"
)

where go >nul 2>nul
if errorlevel 1 (
  echo error: Go not found. Install Go 1.24+ first.
  exit /b 1
)

echo Building deeph.exe...
go build -o "%BIN_PATH%" "%ROOT_DIR%\cmd\deeph"
if errorlevel 1 exit /b 1

if not exist "%WORKSPACE%" mkdir "%WORKSPACE%"
if errorlevel 1 exit /b 1

echo.
echo Bootstrapping workspace: %WORKSPACE%
pushd "%WORKSPACE%"
"%BIN_PATH%" quickstart --workspace "%WORKSPACE%" --deepseek
if errorlevel 1 (
  popd
  exit /b 1
)
popd

echo.
echo Done.
echo Next:
echo   cd "%WORKSPACE%"
echo   set DEEPSEEK_API_KEY=sk-...your_real_key...
echo   "%BIN_PATH%" run guide "hello"

endlocal
