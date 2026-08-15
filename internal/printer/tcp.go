package printer

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

// TCPPrinter fala ESC/POS puro sobre uma conexão TCP crua - o modelo
// mais comum de impressora térmica em rede (porta 9100, "raw
// printing"/JetDirect, sem protocolo por cima). Testável sem hardware
// físico via um listener TCP simples (ver tcp_test.go).
type TCPPrinter struct {
	Address    string
	Port       int
	PaperWidth string

	conn net.Conn
}

func NewTCPPrinter(address string, port int, paperWidth string) *TCPPrinter {
	return &TCPPrinter{Address: address, Port: port, PaperWidth: paperWidth}
}

func (p *TCPPrinter) Connect() error {
	addr := net.JoinHostPort(p.Address, strconv.Itoa(p.Port))

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return fmt.Errorf("falha ao conectar na impressora %s: %w", addr, err)
	}

	p.conn = conn

	return nil
}

func (p *TCPPrinter) Print(receipt Receipt) error {
	if p.conn == nil {
		if err := p.Connect(); err != nil {
			return err
		}
	}

	_, err := p.conn.Write(Render(receipt, p.PaperWidth))

	return err
}

func (p *TCPPrinter) Cut() error {
	if p.conn == nil {
		return fmt.Errorf("impressora não conectada")
	}

	_, err := p.conn.Write(CutCommand())

	return err
}

func (p *TCPPrinter) Close() error {
	if p.conn == nil {
		return nil
	}

	err := p.conn.Close()
	p.conn = nil

	return err
}
