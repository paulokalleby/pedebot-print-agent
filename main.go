package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "PedeBot Print Agent",
		Width:  420,
		Height: 560,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup:        app.startup,
		// Fechar a janela (botão X) nunca deve matar o processo - é
		// aqui que o WebSocket e a fila de impressão vivem. Ver
		// App.onSecondInstanceLaunch pra como o usuário volta a ver a
		// janela depois (substitui a necessidade de bandeja do sistema,
		// ver SPEC-025/memória - 3 tentativas de bandeja já travaram).
		HideWindowOnClose: true,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "br.com.pedebot.printagent",
			OnSecondInstanceLaunch: app.onSecondInstanceLaunch,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
