package main

import (
	"context"

	"pedebot-print-agent/internal/config"
	"pedebot-print-agent/internal/service"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// DefaultServerURL é só o valor pré-preenchido na tela de pareamento -
// o operador raramente precisa trocar (só em desenvolvimento, apontando
// pro backend local). Os dados de conexão do Reverb nunca ficam aqui,
// são buscados no próprio servidor (ver internal/service.Pair).
const DefaultServerURL = "https://api.pedebot.com.br"

// App é a ponte entre a janela (frontend) e o Service - fica fino de
// propósito, toda a orquestração real mora em internal/service.
type App struct {
	ctx context.Context
	svc *service.Service
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	cfg, err := config.Load()
	if err != nil {
		wailsRuntime.LogError(ctx, "falha ao carregar config: "+err.Error())
		cfg = &config.Config{}
	}

	dir, err := config.Dir()
	if err != nil {
		wailsRuntime.LogError(ctx, "falha ao resolver diretório de config: "+err.Error())
	}

	svc, err := service.New(cfg, dir)
	if err != nil {
		wailsRuntime.LogError(ctx, "falha ao iniciar o serviço: "+err.Error())

		return
	}

	a.svc = svc
	a.svc.OnStatusChange = func(status service.Status) {
		wailsRuntime.EventsEmit(a.ctx, "status", status)
	}
	a.svc.OnJobEvent = func(job service.JobEvent) {
		wailsRuntime.EventsEmit(a.ctx, "print-job", job)
	}
	a.svc.Start()
}

// GetDefaultServerURL preenche o campo "Servidor" da tela de
// pareamento - o operador normalmente só confirma, só troca em
// desenvolvimento/teste.
func (a *App) GetDefaultServerURL() string {
	return DefaultServerURL
}

// Pair é chamado pelo frontend com a URL da API e o código digitado
// pelo usuário (formato PDB-XXXX-XXXX) - a configuração do Reverb
// (host/porta/chave) é buscada nesse mesmo servidor, nunca escolhida
// separadamente (ver internal/service.Pair).
func (a *App) Pair(serverURL, code string) string {
	if err := a.svc.Pair(serverURL, code); err != nil {
		return err.Error()
	}

	return ""
}

func (a *App) GetStatus() service.Status {
	if a.svc == nil {
		return service.Status{}
	}

	return a.svc.Status()
}

// GetRecentJobs devolve o snapshot atual da fila - a tela chama isso ao
// abrir, pra não depender só dos eventos que chegarem dali pra frente.
func (a *App) GetRecentJobs() []service.JobEvent {
	if a.svc == nil {
		return nil
	}

	return a.svc.RecentJobs()
}

// Unpair é chamado pelo frontend quando o usuário pede pra parear de
// novo (token guardado localmente não serve mais - estação revogada/
// excluída no painel, por exemplo) - devolve a tela ao estado "digite o
// código de pareamento".
func (a *App) Unpair() string {
	if a.svc == nil {
		return ""
	}

	if err := a.svc.Unpair(); err != nil {
		return err.Error()
	}

	return ""
}
