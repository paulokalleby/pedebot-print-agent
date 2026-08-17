//go:build !linux && !windows

package printer

import (
	"fmt"
	"runtime"
)

// USBPrinter ainda não é suportado nesta plataforma (macOS e outras) -
// USB raw aqui exigiria IOKit (via cgo), sem impressora física real
// pra validar contra (ver SPEC-025, "sem impressora física ainda" -
// mesma disciplina de não implementar às cegas o que não dá pra
// testar). Linux (device file) e Windows (spooler/winspool.drv) já
// funcionam - ver usb_linux.go/usb_windows.go.
type USBPrinter struct {
	DevicePath string
	PaperWidth string
}

func NewUSBPrinter(devicePath, paperWidth string) *USBPrinter {
	return &USBPrinter{DevicePath: devicePath, PaperWidth: paperWidth}
}

func (p *USBPrinter) unsupported() error {
	return fmt.Errorf("impressão USB ainda não é suportada em %s - use uma impressora de rede (TCP/IP) por enquanto", runtime.GOOS)
}

func (p *USBPrinter) Connect() error        { return p.unsupported() }
func (p *USBPrinter) Print(_ Receipt) error { return p.unsupported() }
func (p *USBPrinter) Cut() error            { return p.unsupported() }
func (p *USBPrinter) Close() error          { return nil }
