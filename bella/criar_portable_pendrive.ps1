param([string]$drive = "D:")
$base = "$drive\BELLA_PORTABLE"
New-Item -ItemType Directory -Force -Path $base | Out-Null
New-Item -ItemType Directory -Force -Path "$base\drive_bar" | Out-Null
New-Item -ItemType Directory -Force -Path "$base\.bella\logs" | Out-Null
New-Item -ItemType Directory -Force -Path "$base\PortableGit" | Out-Null
Copy-Item ".\Bella.exe" "$base\Bella.exe" -Force
Copy-Item ".\config.portable.json" "$base\config.json" -Force
Copy-Item ".\BELLA_START.bat" "$base\BELLA_START.bat" -Force
Copy-Item ".\BELLA_SYNC_NOW.bat" "$base\BELLA_SYNC_NOW.bat" -Force
@"
@echo off
explorer "%~dp0BELLA_PORTABLE\drive_bar"
"@ | Set-Content -Encoding ASCII "$drive\Abrir Drive Bar.bat"
@"
@echo off
cd /d "%~dp0BELLA_PORTABLE"
Bella.exe
pause
"@ | Set-Content -Encoding ASCII "$drive\Iniciar BELLA.bat"
Write-Host "B.E.L.L.A. Portable criada em $base"
Write-Host "Edite $base\config.json e troque gitUserEmail pelo seu email do GitHub."
