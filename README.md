# PedeBot Print Agent

Agente local (Go + Wails v2) que roda no computador do estabelecimento,
recebe pedidos via WebSocket (Reverb) e imprime em impressoras térmicas
ESC/POS (USB ou TCP/IP). Ver `.sdd/specs/ACTIVE/SPEC-025-impressao-
automatica-print-agent.md` em `pedebot-api` para o desenho completo.

## Pré-requisitos

- Go 1.24+ e Node 20+.
- CLI oficial do Wails (não confundir com `go build` direto - o Wails
  injeta flags de build/linker específicas por plataforma):
  ```
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0
  ```
  Isso instala o binário em `$(go env GOPATH)/bin` (normalmente
  `~/go/bin`) - garanta que essa pasta esteja no `PATH` do seu shell
  (`export PATH="$PATH:$(go env GOPATH)/bin"` no `~/.zshrc`/`~/.bashrc`).
- Rode `wails doctor` pra conferir se falta alguma dependência da
  plataforma atual (no macOS: Xcode Command Line Tools).

## Rodar em desenvolvimento

```
wails dev
```

Sobe com hot-reload do frontend (Vite) e permite chamar os métodos Go
do devtools do navegador em `http://localhost:34115`.

## Build de produção

**macOS** (gera `.app` em `build/bin/`):
```
wails build
```

**Windows** (gera `.exe` em `build/bin/`) - **compila cruzado direto do
macOS/Linux**, sem precisar de uma máquina Windows só pra isso:
```
wails build -platform windows/amd64
```
**Testado ao vivo numa máquina Windows real (2026-08-17)**: pareamento
e conexão WebSocket confirmados funcionando a partir do `.exe`
cross-compilado. O Windows 10/11 já vem com o runtime WebView2 (via
Edge); em versões mais antigas pode ser necessário instalá-lo à parte.
Impressão em si (USB ou TCP) ainda não foi validada contra hardware
real - ver a seção de impressoras abaixo.

**Instalador Windows (NSIS)** - gera `pedebot-print-agent-amd64-
installer.exe` em `build/bin/`, com atalho no menu Iniciar e entrada
em Adicionar/Remover Programas:
```
brew install makensis   # uma vez, só no macOS - já vem em qualquer distro Linux comum
wails build -platform windows/amd64 -nsis
```
**Ainda não assinado** (nem o `.exe` solto, nem o instalador) - o
Windows SmartScreen vai avisar "aplicativo desconhecido" até isso
existir. Assinatura de código exige um certificado Authenticode
(pessoa jurídica, ~R$500-2000/ano dependendo da CA) emitido pra uma
entidade real - a Publisher já configurada no instalador (`Kall
Labs`, ver `wails.json`) é quem precisaria solicitar o certificado.
Sem isso, distribuir o instalador já é uma melhoria de UX sobre o
`.exe` solto, mas não remove o aviso do SmartScreen.

## Rodar o binário já compilado

- **macOS**: abra `build/bin/pedebot-print-agent.app` (duplo clique ou
  `open build/bin/pedebot-print-agent.app`).
- **Windows**: rode o instalador (`pedebot-print-agent-amd64-
  installer.exe`) ou dê duplo clique direto em
  `pedebot-print-agent.exe`.

Ainda não existe configuração de iniciar com o sistema (autostart) -
depende de uma bandeja do sistema funcionando primeiro (ver
`.sdd/README.md`/memória da SPEC-025 - duas tentativas já travaram).
Por enquanto o Agent precisa ser aberto manualmente a cada boot.

## Limitação conhecida: impressora USB não funciona no macOS

USB via `winspool.drv` funciona no Windows (`internal/printer/
usb_windows.go`) e via device file no Linux (`usb_linux.go`). No
**macOS** ainda não (`usb_other.go`, erro explícito) - exigiria IOKit
via cgo, sem impressora física real pra validar contra ainda.
Impressora de rede (TCP/IP) funciona nas três plataformas.

## Assinatura de código - status e o que falta

Nenhum binário é assinado hoje, nas duas plataformas:

- **macOS**: o `.app` é só auto-assinado ad-hoc (`wails build` já faz
  isso por padrão, é o que permite ele abrir localmente sem o Gatekeeper
  bloquear na própria máquina de build) - **não é o mesmo que uma
  assinatura real**. Distribuir pra fora exige uma conta Apple
  Developer Program (US$99/ano, pessoa jurídica - Kall Labs) + um
  certificado "Developer ID Application" + notarização via `xcrun
  notarytool` depois do build. Sem isso, o Gatekeeper bloqueia o app
  com "não é possível verificar o desenvolvedor" em qualquer Mac que
  não seja o de build.
- **Windows**: precisa de um certificado Authenticode (ver seção do
  instalador acima) + assinar com `signtool` (Windows) ou
  `osslsigncode` (Linux/macOS, alternativa que assina `.exe`/`.msi` sem
  precisar do SDK da Microsoft).

Ambos dependem de comprar um certificado/conta em nome de uma pessoa
jurídica real - não é algo que dá pra "implementar" sem essa decisão de
negócio ser tomada primeiro.

## Pareamento

A autenticação usa um código de pareamento temporário e de uso único
(`PDB-XXXX-XXXX`), gerado no painel (Configurações → Impressão → Nova
estação). Nunca é preciso colar um token manualmente.
