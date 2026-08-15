package main

import (
	"context"

	"pedebot-print-agent/internal/config"
	"pedebot-print-agent/internal/service"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App é a ponte entre a janela (frontend) e o Service - fica fino de
// propósito, toda a orquestração real mora em internal/service.
type App struct {
	ctx     context.Context
	svc     *service.Service
	reverb  service.ReverbConfig
}

func NewApp() *App {
	return &App{
		// Dev local, mesmos valores de pedebot-api/.env - vira
		// configurável antes de qualquer distribuição real.
		reverb: service.ReverbConfig{
			Scheme: "http",
			Host:   "localhost",
			Port:   8080,
			AppKey: "mluuypmnconyrt2e9g1j",
		},
	}
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

	svc, err := service.New(cfg, a.reverb, dir)
	if err != nil {
		wailsRuntime.LogError(ctx, "falha ao iniciar o serviço: "+err.Error())

		return
	}

	a.svc = svc
	a.svc.OnStatusChange = func(status service.Status) {
		wailsRuntime.EventsEmit(a.ctx, "status", status)
	}
	a.svc.Start()
}

// Pair é chamado pelo frontend com o código digitado pelo usuário
// (formato PDB-XXXX-XXXX). serverURL é fixo por enquanto (aponta pro
// backend local) - vira um campo editável antes de qualquer
// distribuição fora da máquina de desenvolvimento.
func (a *App) Pair(code string) string {
	serverURL := "http://localhost:8000"

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
