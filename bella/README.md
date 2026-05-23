# B.E.L.L.A. v0.2.1 Clean Install

**B.E.L.L.A.** significa **Backup Engenhoso Ligeiramente Leve e Automático**.

Esta versão possui dois modos:

- **installed**: roda no computador, monitora uma pasta local e sincroniza com GitHub e pendrive.
- **portable**: roda direto do pendrive e sincroniza a pasta `drive_bar` do próprio pendrive com o GitHub.

O GitHub continua sendo o centro da verdade.

## Funções principais

- Verifica alterações locais antes de sincronizar.
- Verifica alterações remotas no GitHub antes de sincronizar.
- Verifica commits locais ainda não enviados.
- Evita commits vazios.
- Usa `fetch + rebase origin/main`, evitando o erro `Cannot rebase onto multiple branches`.
- Ignora arquivos grandes automaticamente.
- Adiciona arquivos grandes ao `.gitignore`.
- Se arquivo grande já estiver rastreado, usa `git rm --cached` sem apagar do disco.
- Mostra relatório dos arquivos grandes ignorados.
- Monitora pasta local, pendrive ou modo portable.
- Registra logs em `.bella/logs/sync.log`.
- Registra estado em `.bella/state.json`.

## Comandos da B.E.L.L.A.

Digite dentro do terminal da B.E.L.L.A.:

- `sync` — sincroniza a pasta principal do modo atual.
- `usb` — sincroniza o pendrive no modo installed.
- `retry` — tenta novamente a última sincronização que falhou.
- `safe` — configura `safe.directory` para o repositório atual.
- `status` — mostra estado interno da B.E.L.L.A.
- `check` — verifica alterações locais/remotas sem sincronizar.
- `unlock` — libera bloqueio depois de conflito resolvido manualmente.
- `config` — mostra configurações principais.
- `install-startup` — inicia com Windows, apenas no modo installed.
- `uninstall-startup` — remove inicialização automática.
- `close` — encerra.

## Compilar

Na pasta do projeto, onde está o `go.mod`:

```powershell
go build -o Bella.exe ./cmd/bella
```

## Instalar no computador

1. Compile o executável:

```powershell
go build -o Bella.exe ./cmd/bella
```

2. Rode o script:

```powershell
.\criar_instalacao_local.ps1
```

3. Edite:

```txt
C:\BELLA\config.json
```

4. Troque:

```json
"gitUserEmail": "SEU_EMAIL_DO_GITHUB"
```

pelo seu email do GitHub.

5. Abra:

```powershell
cd C:\BELLA
.\Bella.exe
```

6. Teste:

```txt
check
sync
status
```

7. Para iniciar com Windows:

```txt
install-startup
```

## Instalar no pendrive

1. Compile:

```powershell
go build -o Bella.exe ./cmd/bella
```

2. Rode o script, trocando `D:` pela letra real do pendrive:

```powershell
.\criar_portable_pendrive.ps1 -drive "D:"
```

3. Edite:

```txt
D:\BELLA_PORTABLE\config.json
```

4. Troque:

```json
"gitUserEmail": "SEU_EMAIL_DO_GITHUB"
```

pelo seu email do GitHub.

5. Abra no pendrive:

```txt
D:\Iniciar BELLA.bat
```

ou:

```txt
D:\BELLA_PORTABLE\BELLA_START.bat
```

6. Os arquivos sincronizados do pendrive ficam aqui:

```txt
D:\BELLA_PORTABLE\drive_bar
```

7. Para abrir direto os arquivos:

```txt
D:\Abrir Drive Bar.bat
```

## Corrigir Git no pendrive

Se aparecer `safe.directory`, rode:

```powershell
.\corrigir_git_pendrive.ps1 -repo "D:\BELLA_PORTABLE\drive_bar" -email "SEU_EMAIL_DO_GITHUB"
```

Esse script configura:

- `safe.directory`
- `user.name`
- `user.email`
- upstream da branch `main`

## Sobre arquivos grandes

Se a B.E.L.L.A. encontrar arquivo acima de `maxFileSizeMB`, ela não trava mais.

Ela faz:

1. Identifica o arquivo.
2. Adiciona o caminho dele ao `.gitignore`.
3. Se ele já estiver no Git, roda `git rm --cached`.
4. Continua a sincronização.
5. Relata ao usuário no final.

O arquivo continua no dispositivo, mas não é enviado ao GitHub.

## Estrutura local recomendada

```txt
C:\BELLA\
  Bella.exe
  config.json
  .bella\
    logs\
    state.json
```

Pasta sincronizada local:

```txt
C:\Users\gusta\OneDrive\Área de Trabalho\BELLA_UP
```

## Estrutura portable recomendada

```txt
D:\
  Abrir Drive Bar.bat
  Iniciar BELLA.bat
  BELLA_PORTABLE\
    Bella.exe
    config.json
    BELLA_START.bat
    BELLA_SYNC_NOW.bat
    drive_bar\
    PortableGit\
    .bella\
      logs\
      state.json
```

## Limitações da v0.2.1

- Não possui interface gráfica.
- Não resolve conflitos automaticamente.
- Não faz auto-update ainda.
- Não criptografa arquivos.
- Não detecta pendrive por nome do volume, apenas por letra no modo installed.
