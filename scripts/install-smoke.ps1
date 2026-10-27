$ErrorActionPreference = 'Stop'
$portable = Join-Path $PSScriptRoot '../dist/smokey.exe'
$appDir = Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'Programs\Smokey Mirror'
$installedExe = Join-Path $appDir 'smokey.exe'
$shortcut = Join-Path ([Environment]::GetFolderPath('StartMenu')) 'Programs\Smokey Mirror.lnk'
$registryPaths = @(
    'HKCU:\Software\SmokeyMirror',
    'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\SmokeyMirror',
    'HKCU:\Software\Classes\*\shell\SmokeyMirror.Encrypt',
    'HKCU:\Software\Classes\*\shell\SmokeyMirror.Decrypt',
    'HKCU:\Software\Classes\SmokeyMirror.File'
)
foreach ($path in (@($appDir, $shortcut) + $registryPaths)) {
    if (Test-Path -LiteralPath $path) { throw "Existing installation or registration found; test refused: $path" }
}
$environmentKey = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
$pathBefore = $environmentKey.GetValue('Path', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
$environmentKey.Close()
$installed = $false
try {
    & $portable install
    if ($LASTEXITCODE) { throw 'Installation failed' }
    $installed = $true
    if (-not (Test-Path -LiteralPath $installedExe) -or -not (Test-Path -LiteralPath $shortcut)) { throw 'Installed files missing' }
    foreach ($path in $registryPaths) {
        if (-not (Test-Path -LiteralPath $path)) { throw "Registration missing: $path" }
    }
    & $installedExe version
    if ($LASTEXITCODE) { throw 'Installed executable failed' }
    & $installedExe uninstall
    if ($LASTEXITCODE) { throw 'Uninstall failed' }
    $deadline = [DateTime]::UtcNow.AddSeconds(15)
    while ((Test-Path -LiteralPath $installedExe) -and [DateTime]::UtcNow -lt $deadline) { Start-Sleep -Milliseconds 200 }
    foreach ($path in (@($appDir, $shortcut) + $registryPaths)) {
        if (Test-Path -LiteralPath $path) { throw "Uninstall left an artifact: $path" }
    }
    $installed = $false
    $environmentKey = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('Environment')
    $pathAfter = $environmentKey.GetValue('Path', $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
    $environmentKey.Close()
    if ($pathBefore -cne $pathAfter) { throw 'User PATH was not restored exactly' }
    Write-Output 'PASS: per-user installation, installed executable, self-removal, shell registration cleanup, and exact PATH restoration.'
} finally {
    if ($installed) { & $portable uninstall }
}
