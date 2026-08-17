// Package queue é a fila local persistente do Print Agent (ver SPEC-025,
// seção 8 - "não utilizar apenas memória RAM", "implementar retry com
// backoff", "evitar loops infinitos agressivos"). Decisão final
// (2026-08-17): JSON simples, não SQLite. Uma estação de impressão
// nunca acumula mais que algumas dezenas de jobs pendentes por vez
// (`MaxAttempts=5` descarta o resto) - não há consulta nem
// concorrência de escrita entre processos que justifique um banco
// embutido (e SQLite puro-Go/CGO traria dependência e complexidade sem
// benefício real nesse volume). O que de fato importava resolver era
// durabilidade: `save()` escreve num arquivo temporário e troca via
// `os.Rename` (atômico no mesmo volume, evita um `queue.json` truncado
// se o processo for encerrado no meio de uma escrita - realista num PC
// de operador, que sofre queda de energia/desligamento forçado sem
// aviso) e `load()` trata um arquivo corrompido como fila vazia em vez
// de falhar o boot inteiro do Agent (ver o bug real que isso evitava em
// app.go `Pair()`).
package queue

import (
	"encoding/json"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MaxAttempts - depois disso, GiveUp() remove o job da fila local (o
// backend já sabe que falhou via UpdatePrintJobStatus - reimprimir
// depois de esgotar as tentativas é uma ação manual do usuário, não
// algo que o Agent deve insistir sozinho pra sempre).
const MaxAttempts = 5

type Entry struct {
	ID            string    `json:"id"`
	Attempts      int       `json:"attempts"`
	NextAttemptAt time.Time `json:"next_attempt_at"`
}

type Queue struct {
	mu      sync.Mutex
	path    string
	entries map[string]*Entry
}

func New(dir string) (*Queue, error) {
	q := &Queue{path: filepath.Join(dir, "queue.json")}

	if err := q.load(); err != nil {
		return nil, err
	}

	return q, nil
}

func (q *Queue) load() error {
	data, err := os.ReadFile(q.path)
	if os.IsNotExist(err) {
		q.entries = map[string]*Entry{}

		return nil
	}
	if err != nil {
		return err
	}

	var list []*Entry
	if err := json.Unmarshal(data, &list); err != nil {
		// Arquivo corrompido (ex: gravação interrompida por queda de
		// energia/kill do processo) não pode derrubar o boot do Agent
		// inteiro - ver o comentário do pacote. Perder a fila de retry
		// local é aceitável (o backend continua sendo a fonte da
		// verdade dos PrintJob, ver PrintJobService::pendingFor);
		// falhar em abrir o app inteiro por isso não é.
		log.Printf("queue: %s corrompido, iniciando fila vazia: %v", q.path, err)
		q.entries = map[string]*Entry{}

		return nil
	}

	q.entries = map[string]*Entry{}
	for _, e := range list {
		q.entries[e.ID] = e
	}

	return nil
}

// save escreve num arquivo temporário no mesmo diretório e troca via
// os.Rename - atômico no mesmo volume (POSIX e Windows), evita um
// queue.json parcialmente escrito se o processo for encerrado no meio
// do os.WriteFile direto no arquivo final (ver comentário do pacote).
func (q *Queue) save() error {
	list := make([]*Entry, 0, len(q.entries))
	for _, e := range q.entries {
		list = append(list, e)
	}

	data, err := json.Marshal(list)
	if err != nil {
		return err
	}

	tmp := q.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}

	return os.Rename(tmp, q.path)
}

// Add é idempotente - um job já presente na fila não reseta a contagem
// de tentativas (evita que um evento duplicado via WebSocket reinicie o
// backoff de um job que já está tentando de novo).
func (q *Queue) Add(printJobID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, exists := q.entries[printJobID]; exists {
		return nil
	}

	q.entries[printJobID] = &Entry{ID: printJobID, NextAttemptAt: time.Now()}

	return q.save()
}

func (q *Queue) Remove(printJobID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.entries, printJobID)

	return q.save()
}

// Due devolve os jobs prontos pra tentar agora (NextAttemptAt já
// passou) - quem chama decide a ordem de processamento.
func (q *Queue) Due() []string {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	var due []string
	for id, e := range q.entries {
		if !e.NextAttemptAt.After(now) {
			due = append(due, id)
		}
	}

	return due
}

// RecordFailure aplica backoff exponencial (3^tentativas segundos,
// limitado a 5 minutos) e devolve o número de tentativas já feitas e
// se o job esgotou as tentativas - quem chama é responsável por já ter
// marcado o job como `failed` no backend antes de decidir descartar
// localmente.
func (q *Queue) RecordFailure(printJobID string) (attempts int, gaveUp bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	e, exists := q.entries[printJobID]
	if !exists {
		return 0, true
	}

	e.Attempts++
	if e.Attempts >= MaxAttempts {
		attempts = e.Attempts
		delete(q.entries, printJobID)
		_ = q.save()

		return attempts, true
	}

	backoff := time.Duration(math.Pow(3, float64(e.Attempts))) * time.Second
	if backoff > 5*time.Minute {
		backoff = 5 * time.Minute
	}
	e.NextAttemptAt = time.Now().Add(backoff)
	_ = q.save()

	return e.Attempts, false
}

func (q *Queue) List() []string {
	q.mu.Lock()
	defer q.mu.Unlock()

	out := make([]string, 0, len(q.entries))
	for id := range q.entries {
		out = append(out, id)
	}

	return out
}
