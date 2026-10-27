param(
    [string]$Version = '0.1.0',
    [switch]$Resources
)
$ErrorActionPreference = 'Stop'
if ($Version -notmatch '^\d+\.\d+\.\d+([-.][a-zA-Z0-9.]+)?$') { throw 'Invalid version' }
$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    if ($Resources) {
        go run ./tools/icon
        if ($LASTEXITCODE) { throw 'Icon generation failed' }
        go run github.com/akavel/rsrc@v0.10.2 -manifest assets/app.manifest -ico assets/icon.ico -o cmd/smokey/rsrc_windows_amd64.syso
        if ($LASTEXITCODE) { throw 'Resource generation failed' }
        Copy-Item -LiteralPath cmd/smokey/rsrc_windows_amd64.syso -Destination cmd/setup/rsrc_windows_amd64.syso
    }
    go test ./...
    if ($LASTEXITCODE) { throw 'Tests failed' }
    go vet ./...
    if ($LASTEXITCODE) { throw 'Static checks failed' }
    New-Item -ItemType Directory -Force -Path dist,cmd/setup/payload | Out-Null
    go build -trimpath -ldflags "-s -w -X main.version=$Version" -o dist/smokey.exe ./cmd/smokey
    if ($LASTEXITCODE) { throw 'Portable build failed' }
    Copy-Item -LiteralPath dist/smokey.exe -Destination cmd/setup/payload/smokey.exe
    go build -tags installer -trimpath -ldflags "-s -w -H windowsgui -X main.version=$Version" -o dist/SmokeyMirror-Setup.exe ./cmd/setup
    if ($LASTEXITCODE) { throw 'Installer build failed' }
    go version -m dist/smokey.exe | Set-Content -LiteralPath dist/build-info.txt -Encoding utf8
    $hashes = Get-FileHash -LiteralPath dist/smokey.exe,dist/SmokeyMirror-Setup.exe -Algorithm SHA256
    $hashes | ForEach-Object { $_.Hash.ToLowerInvariant() + '  ' + (Split-Path -Leaf $_.Path) } | Set-Content -LiteralPath dist/SHA256SUMS.txt -Encoding ascii
    Write-Output 'Ready: dist/smokey.exe and dist/SmokeyMirror-Setup.exe'
} finally { Pop-Location }
