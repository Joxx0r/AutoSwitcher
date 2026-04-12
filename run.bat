@echo off
cd /d "%~dp0"
go generate ./... && go build -ldflags="-H windowsgui" -o AutoSwitcher.exe . && start "" AutoSwitcher.exe
