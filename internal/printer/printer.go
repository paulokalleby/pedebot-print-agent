// Package printer implementa a abstração de impressora (ver SPEC-025,
// "Estratégia ESC/POS"): TCPPrinter (impressora de rede, porta 9100,
// testável sem hardware via um listener TCP simples) e USBPrinter
// (device file, só Linux por enquanto - ver usb.go). Nenhuma das duas
// foi validada contra uma impressora térmica física real ainda - os
// comandos ESC/POS e a code page CP860 seguem a especificação/tabela
// oficial, mas variação de modelo real (largura de coluna exata,
// comando de corte suportado) só se confirma com hardware em mãos.
package printer

import "log"

type Receipt struct {
	Title string   `json:"title"`
	Lines []string `json:"lines"`
}

type Printer interface {
	Connect() error
	Print(receipt Receipt) error
	Cut() error
	Close() error
}

// LogPrinter só imprime no terminal - usada até a Fase 5 existir de
// verdade, e útil como fallback de debug depois disso também.
type LogPrinter struct{}

func (LogPrinter) Connect() error { return nil }

func (LogPrinter) Print(receipt Receipt) error {
	log.Printf("[LogPrinter] %s", receipt.Title)
	for _, line := range receipt.Lines {
		log.Printf("[LogPrinter] %s", line)
	}

	return nil
}

func (LogPrinter) Cut() error  { return nil }
func (LogPrinter) Close() error { return nil }
