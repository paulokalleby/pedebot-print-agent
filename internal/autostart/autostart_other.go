//go:build !darwin && !windows

// Autostart ainda não implementado no Linux (systemd user service seria
// o caminho, ver macOS/Windows pra o mesmo padrão) - não é o alvo
// principal desta Spec (ver SPEC-025, "a maioria dos clientes reais usa
// Windows"). Enable() é um no-op, não um erro - a ausência de autostart
// não deve impedir o pareamento de completar.
package autostart

func enable() error { return nil }
