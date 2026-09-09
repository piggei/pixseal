@echo off
setlocal
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
if not exist dist mkdir dist
go test ./... || exit /b 1
go build -trimpath -ldflags="-s -w" -o dist\pixseal-windows-amd64.exe .\cmd\pixseal || exit /b 1
echo Created dist\pixseal-windows-amd64.exe
endlocal
