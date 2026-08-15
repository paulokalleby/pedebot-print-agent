package printer

import (
	"bytes"
	"strings"
)

// Comandos ESC/POS padrão (Epson, seguido pela maioria dos fabricantes
// de impressora térmica não-fiscal - ver SPEC-025, "Estratégia ESC/POS").
var (
	cmdInit         = []byte{0x1B, 0x40}       // ESC @
	cmdCodePage860  = []byte{0x1B, 0x74, 0x03} // ESC t 3 - DOS Portuguese
	cmdAlignLeft    = []byte{0x1B, 0x61, 0x00} // ESC a 0
	cmdAlignCenter  = []byte{0x1B, 0x61, 0x01} // ESC a 1
	cmdBoldOn       = []byte{0x1B, 0x45, 0x01} // ESC E 1
	cmdBoldOff      = []byte{0x1B, 0x45, 0x00} // ESC E 0
	cmdPartialCut   = []byte{0x1D, 0x56, 0x01} // GS V 1
	lineFeed        = []byte{0x0A}
)

// Colunas por largura de papel na fonte padrão (fonte A, 12x24) - valor
// típico documentado pela maioria dos fabricantes (Epson TM-T20/T88,
// clones genéricos); pode variar por modelo real - a confirmar contra
// impressora física (ver SPEC-025, Fase 5, "sem layout único fixo").
func columnsFor(paperWidth string) int {
	if paperWidth == "58mm" {
		return 32
	}

	return 48 // default 80mm
}

// Render converte um Receipt (já montado pelo backend - o Agent nunca
// calcula preço/pedido, ver SPEC-025) em bytes ESC/POS prontos pra
// mandar pra impressora, adaptando ao número de colunas da largura de
// papel configurada.
func Render(receipt Receipt, paperWidth string) []byte {
	cols := columnsFor(paperWidth)

	var buf bytes.Buffer
	buf.Write(cmdInit)
	buf.Write(cmdCodePage860)

	buf.Write(cmdAlignCenter)
	buf.Write(cmdBoldOn)
	writeWrapped(&buf, receipt.Title, cols)
	buf.Write(cmdBoldOff)
	writeLine(&buf, strings.Repeat("-", cols))

	buf.Write(cmdAlignLeft)
	for _, line := range receipt.Lines {
		writeWrapped(&buf, line, cols)
	}

	return buf.Bytes()
}

// CutCommand são os bytes que avançam o papel e cortam - separado de
// Render() de propósito, seguindo a interface Printer (Print/Cut são
// operações distintas, ver printer.go) - permite imprimir mais de uma
// coisa antes de cortar uma única vez, se algum dia fizer sentido.
func CutCommand() []byte {
	var buf bytes.Buffer
	buf.Write(lineFeed)
	buf.Write(lineFeed)
	buf.Write(lineFeed)
	buf.Write(cmdPartialCut)

	return buf.Bytes()
}

func writeLine(buf *bytes.Buffer, s string) {
	buf.Write(EncodeCodePage860(s))
	buf.Write(lineFeed)
}

// writeWrapped quebra por palavra respeitando o número de colunas - uma
// palavra sozinha mais longa que `cols` é cortada no limite (impressora
// térmica não tem como exibir além da largura física do papel).
func writeWrapped(buf *bytes.Buffer, text string, cols int) {
	words := strings.Fields(text)
	if len(words) == 0 {
		writeLine(buf, "")

		return
	}

	line := ""
	for _, word := range words {
		for len([]rune(word)) > cols {
			if line != "" {
				writeLine(buf, line)
				line = ""
			}
			runes := []rune(word)
			writeLine(buf, string(runes[:cols]))
			word = string(runes[cols:])
		}

		candidate := word
		if line != "" {
			candidate = line + " " + word
		}

		if len([]rune(candidate)) > cols {
			writeLine(buf, line)
			line = word
		} else {
			line = candidate
		}
	}

	if line != "" {
		writeLine(buf, line)
	}
}
