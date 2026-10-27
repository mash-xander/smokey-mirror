param([string]$Executable = (Join-Path $PSScriptRoot '../dist/smokey.exe'))
$ErrorActionPreference = 'Stop'
$Executable = (Resolve-Path -LiteralPath $Executable).Path
$testDir = Join-Path ([System.IO.Path]::GetTempPath()) ('SmokeyMirror-smoke-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testDir | Out-Null
try {
    $source = Join-Path $testDir 'sample file.bin'
    $key = Join-Path $testDir 'test.smokey-key'
    $restored = Join-Path $testDir 'restored.bin'
    $bytes = New-Object byte[] (3 * 1024 * 1024 + 17)
    [System.Security.Cryptography.RandomNumberGenerator]::Fill($bytes)
    [System.IO.File]::WriteAllBytes($source, $bytes)
    & $Executable version
    if ($LASTEXITCODE) { throw 'Version command failed' }
    & $Executable keygen $key
    if ($LASTEXITCODE) { throw 'Key generation failed' }
    & $Executable encrypt $source --key-file $key
    if ($LASTEXITCODE) { throw 'Encryption failed' }
    & $Executable decrypt ($source + '.smoke') --key-file $key -o $restored
    if ($LASTEXITCODE) { throw 'Decryption failed' }
    if ((Get-FileHash -LiteralPath $source).Hash -ne (Get-FileHash -LiteralPath $restored).Hash) { throw 'Round trip mismatch' }
    & $Executable encrypt $source --key-file $key 2>&1 | Out-Null
    if ($LASTEXITCODE -eq 0) { throw 'Existing output was accepted' }
    $ciphertext = [System.IO.File]::ReadAllBytes($source + '.smoke')
    $ciphertext[$ciphertext.Length - 2] = $ciphertext[$ciphertext.Length - 2] -bxor 1
    $damaged = Join-Path $testDir 'damaged.smoke'
    $rejected = Join-Path $testDir 'must-not-exist.bin'
    [System.IO.File]::WriteAllBytes($damaged, $ciphertext)
    & $Executable decrypt $damaged -k $key -o $rejected 2>&1 | Out-Null
    if ($LASTEXITCODE -eq 0 -or (Test-Path -LiteralPath $rejected)) { throw 'Tampered data was published' }
    if (Get-ChildItem -LiteralPath $testDir -Filter '.smokey-*.tmp') { throw 'Temporary files leaked' }
    Write-Output 'PASS: executable round trip, no-overwrite, integrity failure, and cleanup.'
} finally {
    $resolved = [System.IO.Path]::GetFullPath($testDir)
    $tempRoot = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath()).TrimEnd('\') + '\'
    if ($resolved.StartsWith($tempRoot, [System.StringComparison]::OrdinalIgnoreCase) -and (Split-Path -Leaf $resolved).StartsWith('SmokeyMirror-smoke-')) {
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}
