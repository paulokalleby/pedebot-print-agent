package printer

import (
	"fmt"
	"os"
	"runtime"
)

// USBPrinter escreve ESC/POS puro direto no device file da impressora -
// funciona no Linux, onde uma impressora USB genérica normalmente
// aparece como `/dev/usb/lpN`. **Ainda não implementado no Windows/
// macOS de propósito** - USB raw nesses dois exige uma API bem mais
// pesada (IOKit no macOS, WinUSB/spooler no Windows, tipicamente via
// cgo ou libusb) que não faz sentido construir às cegas sem uma
// impressora física real pra validar contra (ver SPEC-025, Fase 5 -
// "sem impressora física real ainda", mesma disciplina de não
// implementar o que não dá pra testar). `DevicePath` deve ser
// configurado pelo usuário na tela de impressoras (Fase 6) - não há
// descoberta automática ainda.
type USBPrinter struct {
	DevicePath string
	PaperWidth string

	file *os.File
}

func NewUSBPrinter(devicePath, paperWidth string) *USBPrinter {
	return &USBPrinter{DevicePath: devicePath, PaperWidth: paperWidth}
}

func (p *USBPrinter) Connect() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("impressão USB ainda não é suportada em %s - só Linux por enquanto (ver internal/printer/usb.go)", runtime.GOOS)
	}

	f, err := os.OpenFile(p.DevicePath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("falha ao abrir dispositivo USB %s: %w", p.DevicePath, err)
	}

	p.file = f

	return nil
}

func (p *USBPrinter) Print(receipt Receipt) error {
	if p.file == nil {
		if err := p.Connect(); err != nil {
			return err
		}
	}

	_, err := p.file.Write(Render(receipt, p.PaperWidth))

	return err
}

func (p *USBPrinter) Cut() error {
	if p.file == nil {
		return fmt.Errorf("impressora não conectada")
	}

	_, err := p.file.Write(CutCommand())

	return err
}

func (p *USBPrinter) Close() error {
	if p.file == nil {
		return nil
	}

	err := p.file.Close()
	p.file = nil

	return err
}
