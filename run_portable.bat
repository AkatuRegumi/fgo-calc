@echo off
setlocal
cd /d "%~dp0backend"

if not exist "fgo-calc-local.exe" (
  echo [ERROR] fgo-calc-local.exe is missing.
  echo Please download and extract the complete Windows portable package again.
  pause
  exit /b 1
)

if not exist "config.portable.json" (
  echo [ERROR] config.portable.json is missing.
  echo Please download and extract the complete Windows portable package again.
  pause
  exit /b 1
)

echo [INFO] FGO Calc Local Box starting at http://127.0.0.1:30006
echo [INFO] Keep this window open while using FGO Calc. Close it to stop the local server.
start "" powershell.exe -NoProfile -WindowStyle Hidden -Command "Start-Sleep -Seconds 2; Start-Process 'http://127.0.0.1:30006'"
"fgo-calc-local.exe" -config config.portable.json

if errorlevel 1 (
  echo.
  echo [ERROR] FGO Calc exited with an error.
  pause
)
endlocal
