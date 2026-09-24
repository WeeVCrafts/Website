@echo off
setlocal
cd /d "%~dp0"
echo Stopping WeeVCrafts services...
docker compose down
echo All services stopped.
pause
