@echo off
chcp 65001 >nul
powershell -ExecutionPolicy Bypass -NoProfile -File "%~dp0bin\start.ps1" %*
exit /b %ERRORLEVEL%