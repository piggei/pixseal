@echo off
setlocal
if not exist dist mkdir dist
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go test ./... || exit /b 1
go build -trimpath -ldflags="-s -w" -o dist\pixseal-windows-amd64.exe .\cmd\pixseal || exit /b 1
echo Created dist\pixseal-windows-amd64.exe
endlocal
