// Package api é o cliente HTTP fino pro backend do PedeBot - nenhuma
// regra de negócio aqui, só transporte (ver SPEC-025, "O Agent NÃO deve
// conter regra de negócio do PedeBot").
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

type ExchangeResult struct {
	Token        string `json:"token"`
	PrintStation struct {
		ID string `json:"id"`
	} `json:"print_station"`
}

// Exchange troca um código de pareamento por um token - única chamada
// que não precisa de token prévio (ver SPEC-025, "Estratégia de pairing").
func (c *Client) Exchange(code string) (*ExchangeResult, error) {
	var result struct {
		Data ExchangeResult `json:"data"`
	}

	// A resposta do backend não usa envelope "data" pra este endpoint
	// específico (retorna {token, print_station} direto) - tentamos os
	// dois formatos pra não travar se um dia isso mudar.
	body, status, err := c.do("POST", "/print-agent/pairing/exchange", map[string]string{"code": code}, false)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("pareamento falhou (status %d): %s", status, string(body))
	}

	var direct ExchangeResult
	if err := json.Unmarshal(body, &direct); err == nil && direct.Token != "" {
		return &direct, nil
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("resposta inesperada do servidor: %w", err)
	}

	return &result.Data, nil
}

type PrintJob struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Status  string         `json:"status"`
	Printer map[string]any `json:"printer"`
	Receipt map[string]any `json:"receipt"`
}

func (c *Client) PendingPrintJobs() ([]PrintJob, error) {
	var result struct {
		Data []PrintJob `json:"data"`
	}

	body, status, err := c.do("GET", "/print-agent/print-jobs/pending", nil, true)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("status %d ao buscar jobs pendentes: %s", status, string(body))
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func (c *Client) GetPrintJob(id string) (*PrintJob, error) {
	var result struct {
		Data PrintJob `json:"data"`
	}

	body, status, err := c.do("GET", "/print-agent/print-jobs/"+id, nil, true)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("status %d ao buscar job %s: %s", status, id, string(body))
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result.Data, nil
}

func (c *Client) UpdatePrintJobStatus(id, status, errorMessage string) error {
	payload := map[string]string{"status": status}
	if errorMessage != "" {
		payload["error_message"] = errorMessage
	}

	body, respStatus, err := c.do("PATCH", "/print-agent/print-jobs/"+id+"/status", payload, true)
	if err != nil {
		return err
	}
	if respStatus != http.StatusOK {
		return fmt.Errorf("status %d ao atualizar job %s: %s", respStatus, id, string(body))
	}

	return nil
}

// ReverbConfig é a resposta de `GET /print-agent/config` - dados de
// conexão do Reverb do servidor apontado por `baseURL`. Buscado em vez
// de cravado no Agent (ver SPEC-025) - assim trocar de ambiente (local/
// produção) é só trocar a URL da API, o resto vem junto sozinho.
type ReverbConfig struct {
	Scheme string `json:"scheme"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Key    string `json:"key"`
}

func (c *Client) FetchReverbConfig() (*ReverbConfig, error) {
	var result struct {
		Reverb ReverbConfig `json:"reverb"`
	}

	body, status, err := c.do("GET", "/print-agent/config", nil, false)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("status %d ao buscar configuração do Reverb: %s", status, string(body))
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result.Reverb, nil
}

func (c *Client) Heartbeat(agentVersion, os, hostname string) error {
	payload := map[string]string{
		"agent_version": agentVersion,
		"os":            os,
		"hostname":      hostname,
	}

	_, status, err := c.do("POST", "/print-agent/heartbeat", payload, true)
	if err != nil {
		return err
	}
	if status != http.StatusNoContent && status != http.StatusOK {
		return fmt.Errorf("heartbeat falhou (status %d)", status)
	}

	return nil
}

// BroadcastAuth chama o endpoint padrão do Laravel Broadcasting
// (`POST /broadcasting/auth`) pra autorizar a assinatura de um canal
// privado do Reverb - o mesmo endpoint que o Echo do painel Vue usa,
// só que aqui autenticado pelo token Sanctum da PrintStation em vez de
// um usuário (ver EnsurePrintStationToken/routes/channels.php).
func (c *Client) BroadcastAuth(channel, socketID string) (string, error) {
	var result struct {
		Auth string `json:"auth"`
	}

	body, status, err := c.do("POST", "/broadcasting/auth", map[string]string{
		"channel_name": channel,
		"socket_id":    socketID,
	}, true)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("status %d ao autorizar canal %s: %s", status, channel, string(body))
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	return result.Auth, nil
}

func (c *Client) do(method, path string, payload any, authenticated bool) ([]byte, int, error) {
	var reader io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if authenticated {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	return body, resp.StatusCode, nil
}
