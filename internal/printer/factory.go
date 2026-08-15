package printer

import "fmt"

// FromJobPrinter monta a implementação certa a partir dos dados de
// `printer` que vêm no PrintJobResource (ver GetPrintJob no cliente da
// API) - a coluna `address` faz papel duplo por design: IP quando
// `connection_type=tcp`, device path (ex: `/dev/usb/lp0`) quando
// `connection_type=usb`. Reaproveitar a mesma coluna evita uma
// migration nova só pra isso (ver `printers` table, SPEC-025 Fase 1).
func FromJobPrinter(data map[string]any) (Printer, error) {
	connectionType, _ := data["connection_type"].(string)
	paperWidth, _ := data["paper_width"].(string)
	address, _ := data["address"].(string)

	switch connectionType {
	case "tcp":
		port := 9100
		if p, ok := data["port"].(float64); ok {
			port = int(p)
		}

		return NewTCPPrinter(address, port, paperWidth), nil
	case "usb":
		return NewUSBPrinter(address, paperWidth), nil
	default:
		return nil, fmt.Errorf("tipo de conexão de impressora desconhecido: %q", connectionType)
	}
}
