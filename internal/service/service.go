// Package service orquestra o ciclo de vida do Agent: conectar, parear,
// reconectar, sincronizar jobs pendentes e processar jobs recebidos via
// WebSocket. Nenhuma regra de negócio do PedeBot aqui - só orquestração
// de I/O (ver SPEC-025, "O Agent NÃO deve conter regra de negócio").
package service

import (
	"encoding/json"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"pedebot-print-agent/internal/api"
	"pedebot-print-agent/internal/config"
	"pedebot-print-agent/internal/pairing"
	"pedebot-print-agent/internal/printer"
	"pedebot-print-agent/internal/queue"
	"pedebot-print-agent/internal/websocket"
)

const agentVersion = "0.1.0-dev"

// ReverbConfig é fixo por enquanto (aponta pro dev local) - vira
// configurável (ou descoberto via um endpoint do backend) antes de
// qualquer distribuição real pra fora da máquina de desenvolvimento.
type ReverbConfig struct {
	Scheme string
	Host   string
	Port   int
	AppKey string
}

type Status struct {
	Paired    bool   `json:"paired"`
	Connected bool   `json:"connected"`
	LastError string `json:"last_error,omitempty"`
}

type Service struct {
	mu     sync.Mutex
	cfg    *config.Config
	reverb ReverbConfig
	client *api.Client
	ws     *websocket.Client
	queue  *queue.Queue
	printer printer.Printer
	status Status

	OnStatusChange func(Status)
}

func New(cfg *config.Config, reverb ReverbConfig, dir string) (*Service, error) {
	q, err := queue.New(dir)
	if err != nil {
		return nil, err
	}

	s := &Service{
		cfg:     cfg,
		reverb:  reverb,
		queue:   q,
		printer: printer.LogPrinter{},
		status:  Status{Paired: cfg.Paired()},
	}

	if cfg.Paired() {
		s.client = api.New(cfg.ServerURL, cfg.Token)
	}

	return s, nil
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.status
}

func (s *Service) setStatus(mutate func(*Status)) {
	s.mu.Lock()
	mutate(&s.status)
	current := s.status
	s.mu.Unlock()

	if s.OnStatusChange != nil {
		s.OnStatusChange(current)
	}
}

// Pair troca o código por um token, persiste e já inicia a conexão -
// não exige reiniciar o Agent depois de parear.
func (s *Service) Pair(serverURL, code string) error {
	cfg, err := pairing.Pair(serverURL, code)
	if err != nil {
		s.setStatus(func(st *Status) { st.LastError = err.Error() })

		return err
	}

	s.mu.Lock()
	s.cfg = cfg
	s.client = api.New(cfg.ServerURL, cfg.Token)
	s.mu.Unlock()

	s.setStatus(func(st *Status) { st.Paired = true; st.LastError = "" })

	go s.connect()
	go s.processQueueLoop()

	return nil
}

// Start conecta se já estiver pareado - chamado uma vez na
// inicialização do Agent. O processamento da fila roda independente da
// conexão WebSocket estar de pé - offline vira só mais um motivo de
// falha que entra no mesmo backoff (ver processQueueLoop), sem
// tratamento especial.
func (s *Service) Start() {
	if s.cfg.Paired() {
		go s.connect()
		go s.processQueueLoop()
	}
}

func (s *Service) connect() {
	authFunc := func(channel, socketID string) (string, error) {
		return s.client.BroadcastAuth(channel, socketID)
	}

	s.ws = websocket.New(s.reverb.Scheme, s.reverb.Host, s.reverb.Port, s.reverb.AppKey, authFunc)

	s.ws.OnConnected = func() {
		channel := "private-print-station." + s.cfg.PrintStationID
		if err := s.ws.Subscribe(channel); err != nil {
			log.Printf("falha ao assinar canal: %v", err)
			s.setStatus(func(st *Status) { st.LastError = err.Error() })

			return
		}

		s.setStatus(func(st *Status) { st.Connected = true; st.LastError = "" })

		go s.reconcilePending()
		go s.heartbeatLoop()
	}

	s.ws.OnDisconnected = func(err error) {
		s.setStatus(func(st *Status) {
			st.Connected = false
			if err != nil {
				st.LastError = err.Error()
			}
		})

		// Reconexão simples com espera fixa - backoff exponencial de
		// verdade fica pra quando isso for testado contra quedas de
		// rede reais, não antes.
		time.Sleep(5 * time.Second)
		s.connect()
	}

	s.ws.OnEvent = func(event, data string) {
		if event != "print.job.created" {
			return
		}

		var payload struct {
			PrintJobID string `json:"print_job_id"`
		}
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			log.Printf("payload inesperado em print.job.created: %v", err)

			return
		}

		_ = s.queue.Add(payload.PrintJobID)
	}

	if err := s.ws.Connect(); err != nil {
		log.Printf("falha ao conectar no Reverb: %v", err)
		s.setStatus(func(st *Status) { st.Connected = false; st.LastError = err.Error() })

		time.Sleep(5 * time.Second)
		go s.connect()
	}
}

// reconcilePending é a garantia de entrega de verdade (WebSocket é só
// notificação em tempo real, nunca a fonte da verdade - ver SPEC-025,
// "Não confiar exclusivamente no WebSocket"). Só enfileira - quem
// processa de fato é processQueueLoop.
func (s *Service) reconcilePending() {
	jobs, err := s.client.PendingPrintJobs()
	if err != nil {
		log.Printf("falha ao sincronizar jobs pendentes: %v", err)

		return
	}

	for _, job := range jobs {
		_ = s.queue.Add(job.ID)
	}
}

// processQueueLoop tenta os jobs prontos (queue.Due()) a cada poucos
// segundos - continua rodando mesmo com o WebSocket desconectado
// (offline vira só uma falha de tentativa como outra qualquer, sujeita
// ao mesmo backoff, sem tratamento especial). Loop de vida do processo,
// não retorna sozinho.
func (s *Service) processQueueLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		for _, id := range s.queue.Due() {
			s.attemptPrintJob(id)
		}
	}
}

func (s *Service) attemptPrintJob(id string) {
	fail := func(reason string) {
		if gaveUp := s.queue.RecordFailure(id); gaveUp {
			log.Printf("job %s desistido após esgotar tentativas (%s)", id, reason)
		} else {
			log.Printf("job %s falhou, tentando de novo com backoff (%s)", id, reason)
		}
	}

	job, err := s.client.GetPrintJob(id)
	if err != nil {
		fail("busca do job: " + err.Error())

		return
	}

	if err := s.client.UpdatePrintJobStatus(id, "printing", ""); err != nil {
		log.Printf("falha ao marcar job %s como printing: %v", id, err)
	}

	receipt := printer.Receipt{}
	if job.Receipt != nil {
		if title, ok := job.Receipt["title"].(string); ok {
			receipt.Title = title
		}
		if lines, ok := job.Receipt["lines"].([]any); ok {
			for _, l := range lines {
				if s, ok := l.(string); ok {
					receipt.Lines = append(receipt.Lines, s)
				}
			}
		}
	}

	if err := s.printer.Print(receipt); err != nil {
		_ = s.client.UpdatePrintJobStatus(id, "failed", err.Error())
		fail("impressão: " + err.Error())

		return
	}

	if err := s.client.UpdatePrintJobStatus(id, "printed", ""); err != nil {
		fail("confirmação de status: " + err.Error())

		return
	}

	_ = s.queue.Remove(id)
}

func (s *Service) heartbeatLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	send := func() {
		hostname, _ := os.Hostname()
		if err := s.client.Heartbeat(agentVersion, runtime.GOOS, hostname); err != nil {
			log.Printf("heartbeat falhou: %v", err)
		}
	}

	send()
	for range ticker.C {
		send()
	}
}
