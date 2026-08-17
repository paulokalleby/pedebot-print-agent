package queue

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddIsDueImmediately(t *testing.T) {
	q, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if err := q.Add("job-1"); err != nil {
		t.Fatal(err)
	}

	due := q.Due()
	if len(due) != 1 || due[0] != "job-1" {
		t.Fatalf("esperava [job-1] due imediatamente, recebeu %v", due)
	}
}

func TestAddIsIdempotent(t *testing.T) {
	q, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	_ = q.Add("job-1")
	_, _ = q.RecordFailure("job-1") // avança pro attempt=1, next_attempt_at no futuro
	_ = q.Add("job-1")              // não deveria resetar o backoff

	if due := q.Due(); len(due) != 0 {
		t.Fatalf("Add duplicado resetou o backoff - due=%v", due)
	}
}

func TestRecordFailureAppliesBackoff(t *testing.T) {
	q, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	_ = q.Add("job-1")

	attempts, gaveUp := q.RecordFailure("job-1")
	if gaveUp {
		t.Fatal("não deveria desistir na primeira falha")
	}
	if attempts != 1 {
		t.Fatalf("esperava attempts=1 na primeira falha, recebeu %d", attempts)
	}

	if due := q.Due(); len(due) != 0 {
		t.Fatalf("job não deveria estar due logo após a primeira falha (backoff), due=%v", due)
	}
}

func TestRecordFailureGivesUpAfterMaxAttempts(t *testing.T) {
	q, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	_ = q.Add("job-1")

	var gaveUp bool
	for i := 0; i < MaxAttempts; i++ {
		_, gaveUp = q.RecordFailure("job-1")
	}

	if !gaveUp {
		t.Fatalf("deveria ter desistido após %d tentativas", MaxAttempts)
	}

	if list := q.List(); len(list) != 0 {
		t.Fatalf("job deveria ter sido removido da fila após desistir, list=%v", list)
	}
}

func TestRemoveClearsEntry(t *testing.T) {
	q, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	_ = q.Add("job-1")
	_ = q.Remove("job-1")

	if due := q.Due(); len(due) != 0 {
		t.Fatalf("job removido não deveria aparecer em Due(), due=%v", due)
	}
}

func TestQueuePersistsAcrossRestart(t *testing.T) {
	dir := t.TempDir()

	q1, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = q1.Add("job-1")
	_, _ = q1.RecordFailure("job-1")

	q2, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}

	list := q2.List()
	if len(list) != 1 || list[0] != "job-1" {
		t.Fatalf("esperava job-1 sobrevivendo ao restart, recebeu %v", list)
	}
}

func TestRecordFailureOnUnknownJobGivesUp(t *testing.T) {
	q, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	if _, gaveUp := q.RecordFailure("never-added"); !gaveUp {
		t.Fatal("job desconhecido deveria devolver gaveUp=true, nada a incrementar")
	}
}

func TestSaveNeverLeavesATempFileBehind(t *testing.T) {
	dir := t.TempDir()

	q, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = q.Add("job-1")

	if _, err := os.Stat(filepath.Join(dir, "queue.json.tmp")); !os.IsNotExist(err) {
		t.Fatalf("esperava que o arquivo temporário não sobrevivesse ao save(), stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "queue.json")); err != nil {
		t.Fatalf("esperava queue.json persistido, stat err=%v", err)
	}
}

func TestNewToleratesACorruptedQueueFileInsteadOfFailingToBoot(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "queue.json"), []byte("{isso não é json válido"), 0o600); err != nil {
		t.Fatal(err)
	}

	q, err := New(dir)
	if err != nil {
		t.Fatalf("um queue.json corrompido não deveria impedir o Agent de iniciar, recebeu erro: %v", err)
	}

	if list := q.List(); len(list) != 0 {
		t.Fatalf("esperava fila vazia após recuperar de um arquivo corrompido, recebeu %v", list)
	}

	// Confirma que a fila volta a funcionar normalmente depois - não é
	// só "não travar", precisa aceitar novos jobs e persistir de novo.
	if err := q.Add("job-1"); err != nil {
		t.Fatalf("fila recuperada deveria aceitar novos jobs, erro: %v", err)
	}
	if due := q.Due(); len(due) != 1 || due[0] != "job-1" {
		t.Fatalf("esperava [job-1] due após recuperar da corrupção, recebeu %v", due)
	}
}
