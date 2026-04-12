@echo off
cd /d "%~dp0"
go generate ./... && go build -o AutoSwitcher.exe . && start AutoSwitcher.exe
