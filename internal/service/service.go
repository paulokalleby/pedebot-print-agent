// Package service orquestra o ciclo de vida do Agent: conectar, parear,
// reconectar, sincronizar jobs pendentes e processar jobs recebidos via
// WebSocket. Nenhuma regra de negócio do PedeBot aqui - só orquestração
// de I/O (ver SPEC-025, "O Agent NÃO deve conter regra de negócio").
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"
	"time"

	"pedebot-print-agent/internal/api"
	"pedebot-print-agent/internal/autostart"
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

// JobEvent é a visão da fila que a tela mostra ao vivo - não confundir
// com queue.Entry (persistência/retry) nem com o PrintJob da API: é só
// um resumo pra UI, mantido em memória, que desaparece a cada restart
// do Agent (a fonte da verdade de tentativas/retry continua sendo
// queue.Queue e o backend).
type JobEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // order/reprint/test
	Label     string    `json:"label"`
	Status    string    `json:"status"` // queued/printing/printed/failed
	Error     string    `json:"error,omitempty"`
	Attempts  int       `json:"attempts,omitempty"`
	WillRetry bool      `json:"will_retry,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// maxJobHistory limita a lista em memória - é só uma visão recente pra
// UI, não um histórico de auditoria (isso já existe no backend).
const maxJobHistory = 30

type Service struct {
	mu     sync.Mutex
	cfg    *config.Config
	reverb ReverbConfig
	client *api.Client
	ws     *websocket.Client
	queue  *queue.Queue
	status Status
	// stopCh é fechado por Unpair() pra sinalizar connect()/
	// processQueueLoop()/heartbeatLoop() da geração atual de pareamento
	// pra parar - sem isso, despareado o Agent ficaria com goroutines
	// zumbis tentando reconectar/mandar heartbeat com client nil pra
	// sempre (pânico) ou reautenticando com um token já removido.
	stopCh chan struct{}
	// jobHistory - uma entrada por PrintJob (upsert por ID, não um log
	// que cresce a cada transição), tamanho limitado por maxJobHistory.
	// A tela atualiza o status ao vivo (queued -> printing ->
	// printed/failed) na mesma linha, como uma fila de impressão de
	// verdade (pedido explícito: "apresentar o que está sendo imprimido
	// e o que deu falha em tempo real, igual uma fila de impressão").
	jobHistory []JobEvent

	OnStatusChange func(Status)
	OnJobEvent     func(JobEvent)
}

func New(cfg *config.Config, dir string) (*Service, error) {
	q, err := queue.New(dir)
	if err != nil {
		return nil, err
	}

	s := &Service{
		cfg:    cfg,
		queue:  q,
		status: Status{Paired: cfg.Paired()},
	}

	if cfg.Paired() {
		s.client = api.New(cfg.ServerURL, cfg.Token)
	}

	return s, nil
}

// printerFor monta a impressora de verdade a partir dos dados que a API
// devolveu pra este job específico - cada job pode, em tese, ir pra uma
// impressora diferente da mesma estação (ver `printer_id` em
// `print_jobs`), então a resolução é por job, não fixa no Service.
func (s *Service) printerFor(data map[string]any) (printer.Printer, error) {
	return printer.FromJobPrinter(data)
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

// RecentJobs devolve o snapshot atual da fila (mais recente primeiro) -
// usado pela tela ao abrir/recarregar, pra não depender só dos eventos
// que chegarem depois disso.
func (s *Service) RecentJobs() []JobEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]JobEvent, len(s.jobHistory))
	copy(out, s.jobHistory)

	return out
}

// recordJob faz upsert por ID em jobHistory (mesmo job muda de status
// na mesma linha, não vira uma entrada nova) e notifica a tela via
// OnJobEvent. Jobs novos entram no topo; o histórico é limitado a
// maxJobHistory (só uma visão recente, não auditoria).
func (s *Service) recordJob(event JobEvent) {
	event.UpdatedAt = time.Now()

	s.mu.Lock()
	found := false
	for i := range s.jobHistory {
		if s.jobHistory[i].ID == event.ID {
			s.jobHistory[i] = event
			found = true

			break
		}
	}
	if !found {
		s.jobHistory = append([]JobEvent{event}, s.jobHistory...)
		if len(s.jobHistory) > maxJobHistory {
			s.jobHistory = s.jobHistory[:maxJobHistory]
		}
	}
	s.mu.Unlock()

	if s.OnJobEvent != nil {
		s.OnJobEvent(event)
	}
}

// Pair troca o código por um token, persiste e já inicia a conexão -
// não exige reiniciar o Agent depois de parear. serverURL é só a URL
// da API (ex: https://api.pedebot.com.br) - os dados de conexão do
// Reverb são buscados no próprio servidor (ver fetchReverbConfig), o
// Agent nunca tem host/porta/chave do Reverb cravados no binário.
func (s *Service) Pair(serverURL, code string) error {
	reverb, err := fetchReverbConfig(serverURL)
	if err != nil {
		s.setStatus(func(st *Status) { st.LastError = err.Error() })

		return fmt.Errorf("falha ao buscar configuração do servidor: %w", err)
	}

	cfg, err := pairing.Pair(serverURL, code)
	if err != nil {
		s.setStatus(func(st *Status) { st.LastError = err.Error() })

		return err
	}

	stop := make(chan struct{})

	s.mu.Lock()
	s.cfg = cfg
	s.reverb = reverb
	s.client = api.New(cfg.ServerURL, cfg.Token)
	s.stopCh = stop
	s.mu.Unlock()

	s.setStatus(func(st *Status) { *st = Status{Paired: true} })

	// Pareamento é o sinal de "isto é uma instalação de produção de
	// verdade" - habilita autostart aqui, não em algum outro momento
	// (ver internal/autostart, "operador nunca deveria precisar de
	// lançamento manual"). Best-effort - falhar em habilitar autostart
	// nunca deve fazer o pareamento em si falhar.
	if err := autostart.Enable(); err != nil {
		log.Printf("falha ao habilitar autostart (não bloqueia o pareamento): %v", err)
	}

	go s.connect(stop)
	go s.processQueueLoop(stop)

	return nil
}

// Unpair limpa a configuração local (token, estação) e para as
// goroutines de conexão/heartbeat/fila da geração atual - usado quando
// o token guardado não serve mais (estação revogada/excluída no
// painel) e a reconexão automática nunca ia se resolver sozinha. Depois
// de chamado, a tela volta a aceitar um novo código de pareamento.
func (s *Service) Unpair() error {
	s.mu.Lock()
	stop := s.stopCh
	ws := s.ws
	s.stopCh = nil
	s.ws = nil
	s.client = nil
	s.cfg = &config.Config{}
	s.mu.Unlock()

	if stop != nil {
		close(stop)
	}
	if ws != nil {
		_ = ws.Close()
	}

	if err := s.cfg.Save(); err != nil {
		return err
	}

	s.setStatus(func(st *Status) { *st = Status{} })

	return nil
}

// Start conecta se já estiver pareado - chamado uma vez na
// inicialização do Agent. O processamento da fila roda independente da
// conexão WebSocket estar de pé - offline vira só mais um motivo de
// falha que entra no mesmo backoff (ver processQueueLoop), sem
// tratamento especial.
func (s *Service) Start() {
	if !s.cfg.Paired() {
		return
	}

	stop := make(chan struct{})
	s.mu.Lock()
	s.stopCh = stop
	s.mu.Unlock()

	go s.startConnectionLoop(stop)
	go s.processQueueLoop(stop)
}

// fetchReverbConfig busca host/porta/scheme/chave do Reverb em
// `GET /print-agent/config` no servidor apontado por serverURL - nunca
// cravado no Agent (ver SPEC-025), assim reconectar sempre usa a
// configuração atual do servidor, não uma cópia salva que pode ficar
// desatualizada.
func fetchReverbConfig(serverURL string) (ReverbConfig, error) {
	reverb, err := api.New(serverURL, "").FetchReverbConfig()
	if err != nil {
		return ReverbConfig{}, err
	}

	return ReverbConfig{
		Scheme: reverb.Scheme,
		Host:   reverb.Host,
		Port:   reverb.Port,
		AppKey: reverb.Key,
	}, nil
}

// startConnectionLoop busca a configuração do Reverb (com retry - a
// rede pode estar indisponível bem no boot do Agent) e só então conecta
// - chamado por Start() ao reiniciar já pareado (Pair() já tem a
// configuração em mãos na hora, não passa por aqui).
func (s *Service) startConnectionLoop(stop chan struct{}) {
	for {
		s.mu.Lock()
		serverURL := s.cfg.ServerURL
		s.mu.Unlock()

		reverb, err := fetchReverbConfig(serverURL)
		if err != nil {
			s.setStatus(func(st *Status) { st.Connected = false; st.LastError = err.Error() })

			select {
			case <-stop:
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}

		s.mu.Lock()
		s.reverb = reverb
		s.mu.Unlock()

		s.connect(stop)

		return
	}
}

func (s *Service) connect(stop chan struct{}) {
	authFunc := func(channel, socketID string) (string, error) {
		return s.client.BroadcastAuth(channel, socketID)
	}

	ws := websocket.New(s.reverb.Scheme, s.reverb.Host, s.reverb.Port, s.reverb.AppKey, authFunc)

	s.mu.Lock()
	s.ws = ws
	s.mu.Unlock()

	ws.OnConnected = func() {
		channel := "private-print-station." + s.cfg.PrintStationID
		if err := ws.Subscribe(channel); err != nil {
			log.Printf("falha ao assinar canal: %v", err)
			s.setStatus(func(st *Status) { st.LastError = err.Error() })

			return
		}

		s.setStatus(func(st *Status) { st.Connected = true; st.LastError = "" })

		go s.reconcilePending()
		go s.heartbeatLoop(stop)
	}

	ws.OnDisconnected = func(err error) {
		select {
		case <-stop:
			// Desparelado enquanto conectado - não reconecta, não pisa
			// no status (Unpair já zerou pra {}).
			return
		default:
		}

		s.setStatus(func(st *Status) {
			st.Connected = false
			if err != nil {
				st.LastError = err.Error()
			}
		})

		// Reconexão simples com espera fixa - backoff exponencial de
		// verdade fica pra quando isso for testado contra quedas de
		// rede reais, não antes.
		select {
		case <-stop:
			return
		case <-time.After(5 * time.Second):
			s.connect(stop)
		}
	}

	ws.OnEvent = func(event, data string) {
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
		s.recordJob(JobEvent{ID: payload.PrintJobID, Label: "Novo pedido", Status: "queued"})
	}

	if err := ws.Connect(); err != nil {
		log.Printf("falha ao conectar no Reverb: %v", err)
		s.setStatus(func(st *Status) { st.Connected = false; st.LastError = err.Error() })

		select {
		case <-stop:
			return
		case <-time.After(5 * time.Second):
			go s.connect(stop)
		}
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
		s.recordJob(JobEvent{ID: job.ID, Type: job.Type, Label: jobLabel(&job), Status: "queued"})
	}
}

// processQueueLoop tenta os jobs prontos (queue.Due()) a cada poucos
// segundos - continua rodando mesmo com o WebSocket desconectado
// (offline vira só uma falha de tentativa como outra qualquer, sujeita
// ao mesmo backoff, sem tratamento especial). Loop de vida do processo,
// não retorna sozinho.
func (s *Service) processQueueLoop(stop chan struct{}) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			for _, id := range s.queue.Due() {
				s.attemptPrintJob(id)
			}
		}
	}
}

func (s *Service) attemptPrintJob(id string) {
	label := "Pedido"
	jobType := ""

	fail := func(reason, errMsg string) {
		attempts, gaveUp := s.queue.RecordFailure(id)
		if gaveUp {
			log.Printf("job %s desistido após esgotar tentativas (%s)", id, reason)
		} else {
			log.Printf("job %s falhou, tentando de novo com backoff (%s)", id, reason)
		}
		s.recordJob(JobEvent{
			ID: id, Type: jobType, Label: label, Status: "failed",
			Error: errMsg, Attempts: attempts, WillRetry: !gaveUp,
		})
	}

	s.mu.Lock()
	client := s.client
	s.mu.Unlock()
	if client == nil {
		// Despareado entre o job ficar `Due()` e esta tentativa rodar -
		// não há client pra falar com a API, não conta como falha do job
		// (ele continua na fila local, tenta de novo se/quando parear).
		return
	}

	job, err := client.GetPrintJob(id)
	if err != nil {
		fail("busca do job: "+err.Error(), err.Error())

		return
	}

	jobType = job.Type
	label = jobLabel(job)
	s.recordJob(JobEvent{ID: id, Type: jobType, Label: label, Status: "printing"})

	if err := client.UpdatePrintJobStatus(id, "printing", ""); err != nil {
		if errors.Is(err, api.ErrJobConflict) {
			// O backend recusou (409): outro processo já reivindicou este
			// job. Aborta SEM imprimir - insistir aqui é exatamente o
			// cenário de impressão física duplicada que essa checagem
			// existe pra evitar. Remove da fila local, não é um retry.
			log.Printf("job %s já foi reivindicado por outro processo - abortando sem imprimir", id)
			_ = s.queue.Remove(id)
			s.recordJob(JobEvent{
				ID: id, Type: jobType, Label: label, Status: "failed",
				Error: "job já processado por outro processo (pareamento duplicado?)",
			})

			return
		}
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

	dev, err := s.printerFor(job.Printer)
	if err != nil {
		_ = client.UpdatePrintJobStatus(id, "failed", err.Error())
		fail("impressora: "+err.Error(), err.Error())

		return
	}
	defer dev.Close()

	if err := dev.Print(receipt); err != nil {
		_ = client.UpdatePrintJobStatus(id, "failed", err.Error())
		fail("impressão: "+err.Error(), err.Error())

		return
	}

	if err := dev.Cut(); err != nil {
		// Corte falhar não invalida o cupom já impresso - loga e segue,
		// não faz o job inteiro falhar por causa disso.
		log.Printf("job %s: falha ao cortar o papel: %v", id, err)
	}

	if err := client.UpdatePrintJobStatus(id, "printed", ""); err != nil {
		fail("confirmação de status: "+err.Error(), err.Error())

		return
	}

	_ = s.queue.Remove(id)
	s.recordJob(JobEvent{ID: id, Type: jobType, Label: label, Status: "printed"})
}

// jobLabel deriva um texto curto pra mostrar na fila da tela - usa a
// primeira linha do cupom (que já traz "Pedido #123" pros tipos
// order/reprint, ou "TESTE DE IMPRESSÃO" pro tipo test - ver
// `PrintJob::receiptPayload()` no backend) antes de cair pro título
// genérico, já que o `title` do cupom é só o nome do Tenant (repetido
// em todo job, pouco útil pra diferenciar linhas da fila).
func jobLabel(job *api.PrintJob) string {
	if job.Receipt != nil {
		if lines, ok := job.Receipt["lines"].([]any); ok && len(lines) > 0 {
			if first, ok := lines[0].(string); ok && first != "" {
				return first
			}
		}
		if title, ok := job.Receipt["title"].(string); ok && title != "" {
			return title
		}
	}

	return "Pedido " + job.ID
}

func (s *Service) heartbeatLoop(stop chan struct{}) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	send := func() {
		s.mu.Lock()
		client := s.client
		s.mu.Unlock()
		if client == nil {
			return
		}

		hostname, _ := os.Hostname()
		if err := client.Heartbeat(agentVersion, runtime.GOOS, hostname); err != nil {
			log.Printf("heartbeat falhou: %v", err)
		}
	}

	send()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			send()
		}
	}
}
