package printer

import (
	"bytes"
	"strings"
	"testing"
)

func TestRenderStartsWithInitAndCodePage(t *testing.T) {
	out := Render(Receipt{Title: "PEDEBOT", Lines: []string{"linha"}}, "80mm")

	if !bytes.HasPrefix(out, append(cmdInit, cmdCodePage860...)) {
		t.Fatalf("esperava ESC @ seguido de ESC t 3 no início, recebeu %v", out[:8])
	}
}

func TestRenderIncludesTitleAndLines(t *testing.T) {
	out := Render(Receipt{Title: "PEDEBOT", Lines: []string{"Pedido #123", "Total: 50,00"}}, "80mm")

	for _, expected := range []string{"PEDEBOT", "Pedido #123", "Total: 50,00"} {
		if !bytes.Contains(out, []byte(expected)) {
			t.Fatalf("esperava encontrar %q nos bytes renderizados", expected)
		}
	}
}

func TestRenderDoesNotIncludeCutCommand(t *testing.T) {
	out := Render(Receipt{Title: "T", Lines: []string{"x"}}, "80mm")

	if bytes.Contains(out, cmdPartialCut) {
		t.Fatal("Render() não deveria incluir o comando de corte - isso é responsabilidade de Cut()/CutCommand()")
	}
}

func TestCutCommandEndsWithPartialCut(t *testing.T) {
	out := CutCommand()

	if !bytes.HasSuffix(out, cmdPartialCut) {
		t.Fatalf("CutCommand() deveria terminar com GS V 1, recebeu %v", out)
	}
}

func TestRenderWrapsLongLinesToColumnWidth(t *testing.T) {
	long := strings.Repeat("palavra ", 20) // bem mais que 32/48 colunas
	out := Render(Receipt{Title: "T", Lines: []string{long}}, "58mm")

	for _, line := range bytes.Split(out, []byte{0x0A}) {
		// Ignora linhas de controle (título em negrito, separador) -
		// só o corpo (escrito via writeWrapped, sem bytes ESC/GS
		// misturados) precisa respeitar o limite de colunas.
		if bytes.ContainsAny(line, "\x1b\x1d") {
			continue
		}

		decoded := string(line)
		if len([]rune(decoded)) > 32 {
			t.Fatalf("linha excede 32 colunas (58mm): %q (%d runas)", decoded, len([]rune(decoded)))
		}
	}
}

func TestRenderAccentedTextUsesCodePage860Bytes(t *testing.T) {
	out := Render(Receipt{Title: "T", Lines: []string{"café"}}, "80mm")

	if !bytes.Contains(out, []byte{'c', 'a', 'f', 0x82}) {
		t.Fatal("esperava 'café' codificado como CP860 (é = 0x82) nos bytes renderizados")
	}
}
