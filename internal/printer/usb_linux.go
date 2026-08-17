//go:build linux

package printer

import (
	"fmt"
	"os"
)

// USBPrinter escreve ESC/POS puro direto no device file da impressora -
// no Linux, uma impressora USB genérica normalmente aparece como
// `/dev/usb/lpN`. `DevicePath` deve ser configurado pelo usuário na
// tela de impressoras (Fase 6) - não há descoberta automática ainda.
type USBPrinter struct {
	DevicePath string
	PaperWidth string

	file *os.File
}

func NewUSBPrinter(devicePath, paperWidth string) *USBPrinter {
	return &USBPrinter{DevicePath: devicePath, PaperWidth: paperWidth}
}

func (p *USBPrinter) Connect() error {
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
