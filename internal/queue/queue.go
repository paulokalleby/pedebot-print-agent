// Package queue é a fila local persistente do Print Agent (ver SPEC-025,
// seção 8 - "não utilizar apenas memória RAM"). Implementação provisória
// em arquivo JSON simples: o formato final (SQLite embutido foi
// cogitado, ver a Spec) ainda não foi validado - o que importa agora é
// que a lista de jobs pendentes sobrevive a um restart do processo,
// não a tecnologia de armazenamento exata por trás disso.
package queue

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type Queue struct {
	mu   sync.Mutex
	path string
	ids  []string
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
		q.ids = []string{}

		return nil
	}
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &q.ids)
}

func (q *Queue) save() error {
	data, err := json.Marshal(q.ids)
	if err != nil {
		return err
	}

	return os.WriteFile(q.path, data, 0o600)
}

func (q *Queue) Add(printJobID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, id := range q.ids {
		if id == printJobID {
			return nil
		}
	}

	q.ids = append(q.ids, printJobID)

	return q.save()
}

func (q *Queue) Remove(printJobID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	filtered := q.ids[:0]
	for _, id := range q.ids {
		if id != printJobID {
			filtered = append(filtered, id)
		}
	}
	q.ids = filtered

	return q.save()
}

func (q *Queue) List() []string {
	q.mu.Lock()
	defer q.mu.Unlock()

	out := make([]string, len(q.ids))
	copy(out, q.ids)

	return out
}
