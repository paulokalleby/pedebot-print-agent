package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdatePrintJobStatusReturnsErrJobConflictOn409(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"job já reivindicado por outro processo"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token")

	err := c.UpdatePrintJobStatus("job-1", "printing", "")
	if !errors.Is(err, ErrJobConflict) {
		t.Fatalf("esperava ErrJobConflict, recebeu: %v", err)
	}
}

func TestUpdatePrintJobStatusSucceedsOn200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token")

	if err := c.UpdatePrintJobStatus("job-1", "printing", ""); err != nil {
		t.Fatalf("não esperava erro, recebeu: %v", err)
	}
}

func TestUpdatePrintJobStatusReturnsGenericErrorOnOtherStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token")

	err := c.UpdatePrintJobStatus("job-1", "printing", "")
	if err == nil || errors.Is(err, ErrJobConflict) {
		t.Fatalf("esperava um erro genérico (não ErrJobConflict), recebeu: %v", err)
	}
}
