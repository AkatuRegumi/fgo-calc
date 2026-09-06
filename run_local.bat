@echo off
setlocal EnableDelayedExpansion
cd /d "%~dp0backend"

set NEED_BUILD=0
if not exist "fgo-calc-local.exe" set NEED_BUILD=1
if not exist ".local_build_version" set NEED_BUILD=1
if exist ".local_build_version" (
  set /p LOCAL_BUILD_VERSION=<.local_build_version
  if /i not "!LOCAL_BUILD_VERSION!"=="v9-upstream-rebase" set NEED_BUILD=1
)
rem An overlay from the old online-account branch can leave removed auth sources on disk.
if exist "internal\repository\user.go" set NEED_BUILD=1
if exist "internal\util\jwt.go" set NEED_BUILD=1

if "%NEED_BUILD%"=="1" (
  echo [INFO] Local-only backend needs a fresh build...
  call "%~dp0build_local.bat"
  if errorlevel 1 exit /b 1
  cd /d "%~dp0backend"
)

echo [INFO] FGO Calc Local-only starting at http://127.0.0.1:30006
start "" cmd /c "timeout /t 2 /nobreak ^>nul ^& start \"\" http://127.0.0.1:30006"
"fgo-calc-local.exe" -config config.dev.json
endlocal
