//go:build windows

package printer

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// USBPrinter no Windows não fala USB raw (WinUSB) - impressão térmica
// ESC/POS via USB no Windows passa incondicionalmente pelo spooler do
// sistema (winspool.drv), com a impressora já instalada como fila comum
// (Painel de Controle > Dispositivos e Impressoras). `DevicePath`
// carrega o NOME dessa fila (ex: "POS-80 Printer"), não um caminho de
// arquivo - reaproveita a mesma coluna `address` do backend por design
// (ver factory.go). Os bytes ESC/POS vão crus via `pDatatype="RAW"`,
// sem qualquer processamento do driver da impressora.
//
// Chamado direto via `syscall.NewLazyDLL`/`golang.org/x/sys/windows` -
// sem CGO (confirmado que `wails build -platform windows/amd64`
// cross-compila limpo do macOS sem toolchain Windows, e CGO quebraria
// isso). Preferido a `github.com/alexbrainman/printer` (única lib Go
// conhecida pra isso) por essa estar sem commit desde 2020 - mesma
// disciplina já aplicada ao protocolo Pusher e ao Wails v3 (não confiar
// em dependência mal mantida pra algo que um estabelecimento real
// depende todo dia).
type USBPrinter struct {
	DevicePath string
	PaperWidth string

	handle windows.Handle
}

func NewUSBPrinter(devicePath, paperWidth string) *USBPrinter {
	return &USBPrinter{DevicePath: devicePath, PaperWidth: paperWidth}
}

var (
	winspool             = syscall.NewLazyDLL("winspool.drv")
	procOpenPrinterW     = winspool.NewProc("OpenPrinterW")
	procClosePrinter     = winspool.NewProc("ClosePrinter")
	procStartDocPrinterW = winspool.NewProc("StartDocPrinterW")
	procEndDocPrinter    = winspool.NewProc("EndDocPrinter")
	procStartPagePrinter = winspool.NewProc("StartPagePrinter")
	procEndPagePrinter   = winspool.NewProc("EndPagePrinter")
	procWritePrinter     = winspool.NewProc("WritePrinter")
)

// docInfo1 espelha DOC_INFO_1 do Windows (ver
// learn.microsoft.com/windows/win32/printdocs/doc-info-1) - só
// pDatatype="RAW" nos interessa, os outros dois campos ficam nulos.
type docInfo1 struct {
	pDocName    *uint16
	pOutputFile *uint16
	pDatatype   *uint16
}

func (p *USBPrinter) Connect() error {
	name, err := windows.UTF16PtrFromString(p.DevicePath)
	if err != nil {
		return fmt.Errorf("nome de impressora inválido %q: %w", p.DevicePath, err)
	}

	var handle windows.Handle
	r, _, _ := procOpenPrinterW.Call(uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&handle)), 0)
	if r == 0 {
		return fmt.Errorf("falha ao abrir a impressora Windows %q - confira o nome exato em Dispositivos e Impressoras: %w", p.DevicePath, syscall.GetLastError())
	}

	p.handle = handle

	return nil
}

func (p *USBPrinter) Print(receipt Receipt) error {
	if p.handle == 0 {
		if err := p.Connect(); err != nil {
			return err
		}
	}

	return p.write(Render(receipt, p.PaperWidth))
}

func (p *USBPrinter) Cut() error {
	if p.handle == 0 {
		return fmt.Errorf("impressora não conectada")
	}

	return p.write(CutCommand())
}

// write encapsula o ciclo StartDocPrinter/StartPagePrinter/WritePrinter/
// EndPagePrinter/EndDocPrinter que o spooler exige pra qualquer job -
// sem isso o Windows nunca entrega os bytes pra impressora.
func (p *USBPrinter) write(data []byte) error {
	docName, _ := windows.UTF16PtrFromString("PedeBot")
	dataType, _ := windows.UTF16PtrFromString("RAW")
	info := docInfo1{pDocName: docName, pDatatype: dataType}

	if r, _, _ := procStartDocPrinterW.Call(uintptr(p.handle), 1, uintptr(unsafe.Pointer(&info))); r == 0 {
		return fmt.Errorf("falha ao iniciar documento na impressora: %w", syscall.GetLastError())
	}
	defer procEndDocPrinter.Call(uintptr(p.handle))

	if r, _, _ := procStartPagePrinter.Call(uintptr(p.handle)); r == 0 {
		return fmt.Errorf("falha ao iniciar página na impressora: %w", syscall.GetLastError())
	}
	defer procEndPagePrinter.Call(uintptr(p.handle))

	var written uint32
	r, _, _ := procWritePrinter.Call(uintptr(p.handle), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), uintptr(unsafe.Pointer(&written)))
	if r == 0 {
		return fmt.Errorf("falha ao escrever na impressora: %w", syscall.GetLastError())
	}
	if int(written) != len(data) {
		return fmt.Errorf("escrita parcial na impressora: %d de %d bytes", written, len(data))
	}

	return nil
}

func (p *USBPrinter) Close() error {
	if p.handle == 0 {
		return nil
	}

	r, _, _ := procClosePrinter.Call(uintptr(p.handle))
	p.handle = 0
	if r == 0 {
		return fmt.Errorf("falha ao fechar a impressora: %w", syscall.GetLastError())
	}

	return nil
}
