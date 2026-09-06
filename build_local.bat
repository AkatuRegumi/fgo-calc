@echo off
setlocal
cd /d "%~dp0backend"

rem v8 Local-only cleanup: remove obsolete upstream account/auth sources before build.
for %%F in (
  "internal\repository\user.go"
  "internal\repository\user_test.go"
  "internal\util\jwt.go"
  "internal\model\User.go"
) do (
  if exist "%%~F" del /q "%%~F" >nul 2>nul
)
if exist "..\data\users.db" del /q "..\data\users.db" >nul 2>nul
if exist "..\data\users.db-shm" del /q "..\data\users.db-shm" >nul 2>nul
if exist "..\data\users.db-wal" del /q "..\data\users.db-wal" >nul 2>nul
if exist "users.db" del /q "users.db" >nul 2>nul
if exist "users.db-shm" del /q "users.db-shm" >nul 2>nul
if exist "users.db-wal" del /q "users.db-wal" >nul 2>nul

where go >nul 2>nul
if errorlevel 1 (
  echo [ERROR] Go was not found in PATH.
  echo Install Go 1.25.1 or newer, then run this file again.
  pause
  exit /b 1
)

echo [INFO] Synchronizing Go module metadata...
go mod tidy
if errorlevel 1 (
  echo [ERROR] go mod tidy failed.
  echo Check your GOPROXY/network settings, then retry.
  pause
  exit /b 1
)

echo [INFO] Building FGO Calc Local-only...
go build -mod=readonly -o fgo-calc-local.exe .
if errorlevel 1 (
  echo [ERROR] Build failed.
  pause
  exit /b 1
)

> .local_build_version echo v9-upstream-rebase
echo [OK] Built backend\fgo-calc-local.exe
endlocal
