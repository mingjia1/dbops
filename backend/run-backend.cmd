@echo off
rem Backend launcher: load .env then run platform.exe
for /f "usebackq tokens=1,* delims==" %%a in (`findstr /r "^[A-Za-z_][A-Za-z0-9_]*=" "D:\test_tmple\new_dbops\.env"`) do set "%%a=%%b"
cd /d "D:\test_tmple\new_dbops\backend"
bin\platform.exe >> "D:\test_tmple\new_dbops\db\backend_stdout.log" 2>&1
