@echo off
setlocal
cd /d "%~dp0"

set "PY_CMD="
where python >nul 2>nul
if not errorlevel 1 set "PY_CMD=python"
if not defined PY_CMD (
  where py >nul 2>nul
  if not errorlevel 1 set "PY_CMD=py -3"
)
if not defined PY_CMD (
  echo [ERROR] Python 3 was not found in PATH.
  echo Data Sync needs Python 3.10+ and Git.
  pause
  exit /b 1
)

where git >nul 2>nul
if errorlevel 1 (
  echo [ERROR] Git was not found in PATH.
  echo Install Git for Windows, then retry.
  pause
  exit /b 1
)

echo [INFO] Syncing FGO data from Atlas Academy + Chaldea Data...
%PY_CMD% update_data.py
if errorlevel 1 (
  echo [ERROR] Data Sync failed. Existing generated JSON files were kept where possible.
  pause
  exit /b 1
)

echo [OK] Data Sync completed. If the local server is already running, use the in-app updater instead so data reloads immediately.
pause
endlocal
