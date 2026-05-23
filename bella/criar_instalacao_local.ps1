$destino = "C:\BELLA"
New-Item -ItemType Directory -Force -Path $destino | Out-Null
New-Item -ItemType Directory -Force -Path "$destino\.bella\logs" | Out-Null
Copy-Item ".\Bella.exe" "$destino\Bella.exe" -Force
Copy-Item ".\config.installed.json" "$destino\config.json" -Force
Write-Host "Instalação local criada em $destino"
Write-Host "Edite $destino\config.json e troque gitUserEmail pelo seu email do GitHub."
