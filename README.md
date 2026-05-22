# B.E.L.L.A.

**Backup Engenhoso Ligeiramente Leve e Automático**

A B.E.L.L.A. é uma aplicação em Go para sincronizar uma pasta local, um repositório privado no GitHub e um pendrive. A primeira versão roda no Windows via terminal, sem interface gráfica e sem suporte Android.

## Ideia principal

O GitHub é o centro da verdade:

```txt
Pasta local -> GitHub -> Pendrive
Pendrive -> GitHub -> Pasta local
```

A B.E.L.L.A. não faz cópia direta simples entre pasta local e pendrive. Ela usa Git.

## Requisitos

- Windows
- Go instalado
- Git instalado
- Repositório GitHub privado criado, por exemplo `drive_bar`
- Git autenticado no GitHub

## Estrutura

```txt
bella/
  cmd/
    bella/
      main.go
  internal/
    config/
      config.go
    logger/
      logger.go
    git/
      git.go
    ignore/
      ignore.go
    watcher/
      watcher.go
    sync/
      sync.go
    usb/
      usb.go
    startup/
      startup.go
  config.example.json
  README.md
  go.mod
```

## Configuração

Copie:

```txt
config.example.json
```

E crie:

```txt
config.json
```

Exemplo:

```json
{
  "localFolder": "C:\\Users\\gusta\\OneDrive\\Área de Trabalho\\BELLA_UP",
  "repoName": "drive_bar",
  "repoUrl": "https://github.com/SEU_USUARIO/drive_bar.git",
  "branch": "main",
  "autoCommitDelaySeconds": 30,
  "usbDriveLetter": "D:",
  "usbFolder": "D:\\BELLA_UP",
  "maxFileSizeMB": 100,
  "gitPath": "git",
  "ignorePatterns": [
    "*.tmp",
    "*.log",
    "*.cache",
    "*.zip",
    "*.rar",
    "*.iso",
    "*.mp4",
    "node_modules",
    "dist",
    "build"
  ],
  "autoStartWithWindows": false,
  "askBeforeUsbSync": true
}
```

## Executar em desenvolvimento

```bash
go mod tidy
go run ./cmd/bella
```

## Gerar Bella.exe

```bash
go build -o Bella.exe ./cmd/bella
```

Coloque `Bella.exe` e `config.json` na mesma pasta.

## Comandos disponíveis

```txt
sync
```

Sincroniza a pasta local com o GitHub.

```txt
usb
```

Sincroniza o pendrive com o GitHub e depois tenta atualizar a pasta local.

```txt
status
```

Mostra estado interno da B.E.L.L.A.

```txt
unlock
```

Remove bloqueio após conflito Git resolvido manualmente.

```txt
install-startup
```

Configura a B.E.L.L.A. para iniciar junto com o Windows.

```txt
uninstall-startup
```

Remove a B.E.L.L.A. da inicialização do Windows.

```txt
close
```

Encerra a aplicação.

## Pendrive

A v0.1 detecta o pendrive por letra fixa, por exemplo:

```json
"usbDriveLetter": "D:",
"usbFolder": "D:\\BELLA_UP"
```

Quando o pendrive for conectado, a B.E.L.L.A. pergunta:

```txt
Deseja sincronizar agora? [s/n]
```

O pendrive não executa a B.E.L.L.A. sozinho. O correto é deixar `Bella.exe` iniciar com o Windows usando `install-startup`.

## Logs

Os logs ficam ao lado do executável:

```txt
.bella/logs/sync.log
```

## Arquivos ignorados

A B.E.L.L.A. cria/atualiza `.gitignore` na pasta monitorada e ignora:

```txt
.bella/
*.tmp
*.log
*.cache
*.zip
*.rar
*.iso
*.mp4
node_modules/
dist/
build/
```

## Arquivos grandes

Arquivos maiores que `maxFileSizeMB` bloqueiam o ciclo de sincronização. A B.E.L.L.A. avisa no terminal e registra no log.

## Conflitos

Se houver conflito no `git pull --rebase`, a B.E.L.L.A.:

- bloqueia novas sincronizações
- registra no log
- não faz push
- espera resolução manual

Depois de resolver manualmente, use:

```txt
unlock
```

## Limitações da v0.1

Ainda não possui:

- interface gráfica
- Android
- aplicativo mobile
- criptografia avançada
- resolução automática de conflitos
- múltiplos pendrives
- detecção por nome do volume
- suporte a outros serviços além do GitHub
