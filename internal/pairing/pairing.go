// Package pairing orquestra a troca de um código de pareamento por um
// token - nunca pede/mostra o token pro humano (ver SPEC-025).
package pairing

import (
	"pedebot-print-agent/internal/api"
	"pedebot-print-agent/internal/config"
)

// Pair troca o código, salva o token/print_station_id na config local e
// devolve a config já atualizada.
func Pair(serverURL, code string) (*config.Config, error) {
	client := api.New(serverURL, "")

	result, err := client.Exchange(code)
	if err != nil {
		return nil, err
	}

	cfg := &config.Config{
		ServerURL:      serverURL,
		PrintStationID: result.PrintStation.ID,
		Token:          result.Token,
	}

	if err := cfg.Save(); err != nil {
		return nil, err
	}

	return cfg, nil
}
