// Package config lida com a configuração local persistente do Print
// Agent - nunca em texto puro num lugar óbvio do repositório, sempre no
// diretório de config do próprio usuário do SO (ver SPEC-025, seção
// "Configuração local").
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const dirName = "PedeBot Print Agent"

// Config é o estado persistido entre execuções do Agent. Token é o
// Sanctum token da PrintStation, obtido uma única vez via
// pairing.Exchange - nunca digitado/colado manualmente pelo usuário.
type Config struct {
	ServerURL     string `json:"server_url"`
	PrintStationID string `json:"print_station_id"`
	Token         string `json:"token"`
}

func (c *Config) Paired() bool {
	return c.Token != ""
}

// Dir retorna o diretório de config do Agent (criado se não existir),
// usando o diretório de config padrão do SO (%AppData% no Windows,
// ~/Library/Application Support no macOS, ~/.config no Linux via
// os.UserConfigDir()).
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(base, dirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	return dir, nil
}

func filePath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "config.json"), nil
}

// Load nunca falha por "arquivo não existe" - devolve uma Config vazia
// (não pareada ainda), é o estado normal na primeira execução.
func Load() (*Config, error) {
	path, err := filePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Save() error {
	path, err := filePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	// 0600 - só o usuário dono do processo lê/escreve (contém o token).
	return os.WriteFile(path, data, 0o600)
}
