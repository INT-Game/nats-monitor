@echo off
cd /d "%~dp0"
nats-monitor.exe -config config.json
