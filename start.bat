@echo off
setlocal

echo ========================================================
echo  Starting Weevcrafts Marketplace (Windows)
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

echo [+] Starting services (PostgreSQL, Redis, Search, Garage S3, Mailpit, Adminer, Uptime Kuma, Web)...
docker compose up -d --build
if %errorlevel% neq 0 exit /b 1

echo [+] Waiting for local monitoring setup...
docker compose wait uptime-kuma-setup
if %errorlevel% neq 0 exit /b 1

echo.
echo [+] Seeding demo marketplace accounts and products...
docker compose --profile demo run --rm seed-demo

echo.
echo ========================================================
echo  Weevcrafts is LIVE!
echo  Adminer: http://localhost:8082
echo  Uptime Kuma: http://localhost:3001
echo  Opening http://localhost:8080 in your browser...
echo ========================================================
start http://localhost:8080
pause
