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
Importante: cross-compilar produz um `.exe` que builda limpo, mas
**ainda não foi executado/testado numa máquina Windows real** - antes
de distribuir pra um estabelecimento, valide numa VM ou máquina Windows
de verdade. O Windows 10/11 já vem com o runtime WebView2 (via Edge);
em versões mais antigas pode ser necessário instalá-lo à parte.

## Rodar o binário já compilado

- **macOS**: abra `build/bin/pedebot-print-agent.app` (duplo clique ou
  `open build/bin/pedebot-print-agent.app`).
- **Windows**: dê duplo clique em `pedebot-print-agent.exe`.

Ainda não existe instalador nem configuração de iniciar com o sistema
(autostart) - isso é trabalho em andamento da SPEC-025. Por enquanto o
Agent precisa ser aberto manualmente a cada boot.

## Limitação conhecida: impressora USB só funciona no Linux

`internal/printer/usb.go` hoje só implementa impressão USB de verdade
no Linux (usa o device file `/dev/usb/lpN`). No macOS e **no Windows -
onde está a maioria dos clientes -** uma impressora conectada via USB
ainda não imprime; só impressoras de rede (TCP/IP) funcionam nessas
duas plataformas hoje. Suporte a USB no Windows precisaria de uma
implementação própria (ex: via spooler do Windows ou WinUSB) - ainda
não escopado.

## Pareamento

A autenticação usa um código de pareamento temporário e de uso único
(`PDB-XXXX-XXXX`), gerado no painel (Configurações → Impressão → Nova
estação). Nunca é preciso colar um token manualmente.
