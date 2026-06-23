@echo off
chcp 65001 >/dev/null
powershell -ExecutionPolicy Bypass -NoProfile -File "%~dp0restart.ps1" %*
exit /b %ERRORLEVEL%
