//go:build windows

package autostart

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const valueName = "PedeBotPrintAgent"

func enable() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("falha ao resolver o caminho do próprio executável: %w", err)
	}

	// HKCU (não HKLM) - não exige privilégio de administrador, e
	// autostart é por usuário mesmo (cada operador que faz login tem
	// seu próprio Agent, não é uma instalação de máquina compartilhada).
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("falha ao abrir a chave de autostart do Windows: %w", err)
	}
	defer key.Close()

	// Abre normal (não escondido/minimizado) - ver autostart_darwin.go
	// pro porquê de não tentar nascer escondido: no macOS isso quebrou
	// o único jeito nativo de voltar a ver a janela depois (clicar no
	// Dock só reativa a instância existente, nunca lança um processo
	// novo pra disparar o SingleInstanceLock). Mantendo o mesmo
	// comportamento nas duas plataformas por consistência, mesmo sem
	// ter confirmado ao vivo se o Windows tem essa mesma limitação.
	command := fmt.Sprintf(`"%s"`, execPath)

	return key.SetStringValue(valueName, command)
}
