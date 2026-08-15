package printer

// CP860 é a code page "DOS Portuguese" (IBM 00860) - cobre os acentos
// do português (á ã ç é ê ó etc.) que a maioria das impressoras ESC/POS
// entende nativamente via `ESC t 3` (ver escpos.go). Valores conferidos
// contra a tabela oficial da Unicode Consortium
// (unicode.org/Public/MAPPINGS/VENDORS/MICSFT/PC/CP860.TXT) - a metade
// baixa (0x00-0x7F) é igual ao ASCII puro, só a extended table (0x80+)
// muda por code page.
var cp860 = map[rune]byte{
	'Ç': 0x80, 'ü': 0x81, 'é': 0x82, 'â': 0x83, 'ã': 0x84, 'à': 0x85,
	'Á': 0x86, 'ç': 0x87, 'ê': 0x88, 'Ê': 0x89, 'è': 0x8a, 'Í': 0x8b,
	'Ô': 0x8c, 'ì': 0x8d, 'Ã': 0x8e, 'Â': 0x8f,
	'É': 0x90, 'À': 0x91, 'Ó': 0x9f,
	'ô': 0x93, 'õ': 0x94, 'Õ': 0x99, 'Ú': 0x96,
	'á': 0xa0, 'í': 0xa1, 'ó': 0xa2, 'ú': 0xa3, 'ñ': 0xa4, 'Ñ': 0xa5,
}

// EncodeCodePage860 converte uma string UTF-8 pros bytes da CP860 -
// ESC/POS não entende UTF-8 nativamente (ver SPEC-025, "dar atenção
// especial à codificação de caracteres"). Caractere sem mapeamento
// conhecido vira '?' em vez de quebrar a impressão inteira por causa de
// um símbolo raro.
func EncodeCodePage860(s string) []byte {
	out := make([]byte, 0, len(s))

	for _, r := range s {
		if r < 128 {
			out = append(out, byte(r))

			continue
		}

		if b, ok := cp860[r]; ok {
			out = append(out, b)

			continue
		}

		out = append(out, '?')
	}

	return out
}
