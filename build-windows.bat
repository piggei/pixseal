@echo off
setlocal
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
if not exist dist mkdir dist
go test ./cmd/pixseal ./internal/buildinfo -count=1 || exit /b 1
go test ./watermark -run "Test(V3EncoderGoldenFingerprint|StrengthRejectsNonFiniteValues|WorkingImageLimitRejectsBeforePixelPlaneAllocation|WorkingImageLimitRejectsIntegerOverflow|AnalyzerUsesSameWhiteAlphaFlatteningAsEncoder|IsotropicScaleSearchIsFixed|HammingCorrectsSingleBit|ProfileSelectionThresholds|ExplicitProfileCapacityErrors|V3ProfileRoundTrips|V3TransformsByProfile|V3AutoProfileExtraction|WrongKeyAndUnmarkedImageAreBounded|AnalyzeImageMatchesProfileMath|V3FrameIgnoresTrailingPaddingButAuthenticatesHeader|V3SyncPatternObservationCounts|V3TileMappingObservationCounts)$" -count=1 || exit /b 1
go build -trimpath -ldflags="-s -w" -o dist\pixseal-windows-amd64.exe .\cmd\pixseal || exit /b 1
echo Created dist\pixseal-windows-amd64.exe
endlocal
