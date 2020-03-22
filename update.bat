@echo off
echo Updating version, please wait a few seconds
timeout /t 5 /nobreak >nul
del run_flexx.exe
rename run_flexx.latest.exe run_flexx.exe
run_flexx.exe