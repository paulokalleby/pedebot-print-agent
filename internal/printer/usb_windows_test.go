//go:build windows

package printer

import "testing"

// Só roda de verdade numa máquina Windows (build tag) - exercita a API
// real do winspool.drv (não tem como simular numa impressora física
// sem hardware), mas cobre o caminho de erro: um nome de fila que não
// existe deve falhar de forma descritiva, nunca travar/panicar.
func TestUSBPrinterConnectToUnknownQueueFails(t *testing.T) {
	p := NewUSBPrinter("Impressora Que Não Existe De Verdade PedeBot Teste", "80mm")

	if err := p.Connect(); err == nil {
		t.Fatal("esperava falha ao conectar numa fila de impressão inexistente")
	}
}
