package printer

import (
	"bytes"
	"testing"
)

func TestEncodeCodePage860KnownChars(t *testing.T) {
	cases := []struct {
		in   string
		want []byte
	}{
		{"café", []byte{'c', 'a', 'f', 0x82}}, // é = 0x82
		{"água", []byte{0xa0, 'g', 'u', 'a'}}, // á = 0xa0
		{"ção", []byte{0x87, 0x84, 'o'}},      // ç = 0x87, ã = 0x84
	}

	for _, c := range cases {
		got := EncodeCodePage860(c.in)
		if !bytes.Equal(got, c.want) {
			t.Fatalf("%q: esperava %v, recebeu %v", c.in, c.want, got)
		}
	}
}

func TestEncodeCodePage860FallsBackToQuestionMark(t *testing.T) {
	got := EncodeCodePage860("emoji 🎉")
	if got[len(got)-1] != '?' {
		t.Fatalf("caractere sem mapeamento deveria virar '?', último byte foi %v", got[len(got)-1])
	}
}

func TestEncodeCodePage860LeavesASCIIUntouched(t *testing.T) {
	got := EncodeCodePage860("PEDEBOT 123")
	if string(got) != "PEDEBOT 123" {
		t.Fatalf("ASCII puro não deveria mudar, recebeu %q", string(got))
	}
}
