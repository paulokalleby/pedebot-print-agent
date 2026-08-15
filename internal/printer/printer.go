// Package printer define a abstração de impressora (ver SPEC-025,
// "Estratégia ESC/POS") - só a interface e um stub de log por enquanto.
// A implementação real ESC/POS (USB/TCP, code page, corte de papel) é
// Fase 5, ainda não começou - registrar aqui só o suficiente pra provar
// o resto do pipeline (pareamento -> WebSocket -> busca do job -> status)
// sem fingir que a impressão física já funciona.
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
