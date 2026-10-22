$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')
if (-not $env:GO_BIN) { $env:GO_BIN = 'go' }
function Check-Exit { if ($LASTEXITCODE -ne 0) { throw "Build command failed: $LASTEXITCODE" } }
if ((& $env:GO_BIN version) -notmatch '^go version go1\.20\.14 ') { throw 'Go 1.20.14 is required.' }
npm ci --prefix frontend --ignore-scripts --no-audit --no-fund; Check-Exit
npm run build --prefix frontend; Check-Exit
npm test --prefix frontend; Check-Exit
& $env:GO_BIN test ./...; Check-Exit
& $env:GO_BIN vet ./...; Check-Exit
New-Item -ItemType Directory -Force release | Out-Null
$env:CGO_ENABLED = '1'
$env:GOOS = 'windows'
try {
  foreach ($target in @(@('386','x86','i686-w64-mingw32-gcc'), @('amd64','x64','x86_64-w64-mingw32-gcc'))) {
    $env:GOARCH = $target[0]
    $env:CC = $target[2]
    & $env:GO_BIN build -buildvcs=false -trimpath '-ldflags=-s -w -extldflags=-static' -o "release/BillBook-Windows-$($target[1]).exe" ./cmd/billbook
    Check-Exit
  }
} finally {
  Remove-Item Env:GOOS,Env:GOARCH,Env:CC,Env:CGO_ENABLED -ErrorAction SilentlyContinue
}
& "$PSScriptRoot/verify-build.ps1"
node scripts/manifest.mjs; Check-Exit
Write-Output 'Windows runtime and browser workflow tests must still be executed on target machines.'
