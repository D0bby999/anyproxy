# SPDX-License-Identifier: AGPL-3.0-or-later
# Downloads WinDivert 2.2.2 and compiles ProxyBridgeCore.dll beside AnyProxy.exe.
param(
    [string]$OutDir = "build/bin",
    [string]$WinDivertDir = "$env:TEMP\WinDivert-2.2.2-A"
)
$ErrorActionPreference = "Stop"

if (-not (Test-Path "$WinDivertDir\include\windivert.h")) {
    $zip = "$env:TEMP\WinDivert-2.2.2-A.zip"
    Invoke-WebRequest -Uri "https://reqrypt.org/download/WinDivert-2.2.2-A.zip" -OutFile $zip
    Expand-Archive -Path $zip -DestinationPath $env:TEMP -Force
}

New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
$sources = Get-ChildItem -Path "windows/core/*.c" | ForEach-Object { $_.FullName }

& gcc -shared -O2 -s -D_WIN32_WINNT=0x0601 -DPROXYBRIDGE_EXPORTS -D__forceinline=inline `
    -I"$WinDivertDir\include" `
    @sources `
    -L"$WinDivertDir\x64" -lWinDivert -lws2_32 -liphlpapi `
    -o "$OutDir\ProxyBridgeCore.dll"

if ($LASTEXITCODE -ne 0) {
    throw "gcc failed compiling ProxyBridgeCore.dll"
}

Copy-Item "$WinDivertDir\x64\WinDivert.dll" "$OutDir\WinDivert.dll" -Force
Copy-Item "$WinDivertDir\x64\WinDivert64.sys" "$OutDir\WinDivert64.sys" -Force
Write-Host "Built $OutDir\ProxyBridgeCore.dll + WinDivert runtime"
