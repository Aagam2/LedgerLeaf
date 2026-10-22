$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$hashes = @()
foreach ($target in @(@('x86','i686-w64-mingw32-objdump'), @('x64','x86_64-w64-mingw32-objdump'))) {
  $path = Join-Path $root "release/BillBook-Windows-$($target[0]).exe"
  if (-not (Test-Path $path)) { throw "Missing executable: $path" }
  $imports = & ($target[1]) -p $path
  if ($LASTEXITCODE -ne 0) { throw 'PE inspection failed' }
  $imports | Set-Content (Join-Path $root "release/imports-$($target[0]).txt")
  foreach ($line in ($imports | Select-String 'DLL Name:')) {
    if ($line -notmatch 'DLL Name: (KERNEL32.dll|msvcrt.dll)\s*$') { throw "Unexpected dependency: $line" }
  }
  $hashes += "$((Get-FileHash $path -Algorithm SHA256).Hash.ToLower())  $([IO.Path]::GetFileName($path))"
}
$hashes | Set-Content (Join-Path $root 'release/SHA256SUMS')
