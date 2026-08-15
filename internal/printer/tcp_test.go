package printer

import (
	"bytes"
	"net"
	"strconv"
	"testing"
	"time"
)

// startMockPrinter sobe um listener TCP real na loopback - não é uma
// impressora física, mas exercita o transporte TCP de verdade (dial,
// write, close), diferente de um teste que só chama Render() em
// isolamento.
func startMockPrinter(t *testing.T) (addr string, port int, received chan []byte) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("falha ao subir listener de teste: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	received = make(chan []byte, 1)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		received <- buf[:n]

		// Segunda leitura, pro teste de Cut() separado.
		n2, _ := conn.Read(buf)
		if n2 > 0 {
			received <- buf[:n2]
		}
	}()

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ = strconv.Atoi(portStr)

	return host, port, received
}

func TestTCPPrinterSendsRenderedReceipt(t *testing.T) {
	addr, port, received := startMockPrinter(t)

	p := NewTCPPrinter(addr, port, "80mm")
	defer p.Close()

	if err := p.Print(Receipt{Title: "PEDEBOT", Lines: []string{"linha 1"}}); err != nil {
		t.Fatalf("Print() falhou: %v", err)
	}

	select {
	case data := <-received:
		if !bytes.Contains(data, []byte("PEDEBOT")) || !bytes.Contains(data, []byte("linha 1")) {
			t.Fatalf("dados recebidos não batem com o esperado: %v", data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout esperando a impressora mock receber dados")
	}
}

func TestTCPPrinterCutSendsPartialCut(t *testing.T) {
	addr, port, received := startMockPrinter(t)

	p := NewTCPPrinter(addr, port, "80mm")
	defer p.Close()

	if err := p.Print(Receipt{Title: "T", Lines: []string{"x"}}); err != nil {
		t.Fatalf("Print() falhou: %v", err)
	}
	<-received // consome o print, isola o Cut() na segunda leitura

	if err := p.Cut(); err != nil {
		t.Fatalf("Cut() falhou: %v", err)
	}

	select {
	case data := <-received:
		if !bytes.Contains(data, cmdPartialCut) {
			t.Fatalf("Cut() deveria mandar o comando de corte, recebeu %v", data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout esperando o comando de corte")
	}
}

func TestTCPPrinterCutWithoutConnectionFails(t *testing.T) {
	p := NewTCPPrinter("127.0.0.1", 1, "80mm") // nunca conectado

	if err := p.Cut(); err == nil {
		t.Fatal("Cut() sem conexão deveria falhar")
	}
}

func TestTCPPrinterConnectFailureIsReported(t *testing.T) {
	// Porta improvável de ter algo escutando - dial deve falhar rápido
	// o bastante pra não travar o teste.
	p := NewTCPPrinter("127.0.0.1", 1, "80mm")

	if err := p.Connect(); err == nil {
		t.Fatal("esperava falha ao conectar numa porta sem listener")
	}
}
