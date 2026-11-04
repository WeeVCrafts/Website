@echo off
setlocal

echo ========================================================
echo  Starting WeevCrafts Marketplace (Windows)
echo ========================================================

cd /d "%~dp0"

if not exist ".env" (
    echo [+] Creating .env configuration from .env.example...
    copy .env.example .env >nul
)

docker info >nul 2>&1
if %errorlevel% neq 0 (
    echo.
    echo [-] Error: Docker Desktop is not running!
    echo     Please launch Docker Desktop and run start.bat again.
    pause
    exit /b 1
)

echo [+] Starting services (PostgreSQL, Redis, Search, Garage S3, Web)...
docker compose up -d --build
if %errorlevel% neq 0 exit /b 1

echo [+] Existing accounts and catalog retained. No seeding or simulated payments.

echo.
echo ========================================================
echo  WeevCrafts is LIVE!
echo  Opening http://localhost:8080 in your browser...
echo ========================================================
start http://localhost:8080
pause
