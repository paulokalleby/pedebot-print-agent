//go:build darwin

package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Mesmo identificador usado no bundle ID (ver build/darwin/Info.plist)
// - Label do LaunchAgent precisa ser único no sistema, reverse-DNS é a
// convenção da própria Apple pra isso.
const label = "br.com.pedebot.printagent"

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, "Library", "LaunchAgents", label+".plist"), nil
}

func enable() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("falha ao resolver o caminho do próprio executável: %w", err)
	}

	path, err := plistPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("falha ao criar ~/Library/LaunchAgents: %w", err)
	}

	// Abre normal (não escondido) - tentei nascer direto em StartHidden
	// e travou uma limitação real do macOS: clicar no Dock/Launchpad de
	// um app já rodando só REATIVA a instância existente, sem nunca
	// chegar a lançar um processo novo - o SingleInstanceLock nunca
	// dispara porque depende exatamente disso. `[NSApp hide:nil]` (o
	// que HideWindowOnClose usa, ver main.go) já tem esse
	// comportamento de "clicar no Dock desesconde" de fábrica no
	// AppKit; `StartHidden` (nunca mostrar a janela) não tem. Deixar
	// abrir normal aqui e confiar no fluxo já validado (fechar
	// esconde, reabrir pelo Dock desesconde) é mais simples e funciona
	// de verdade - confirmado ao vivo.
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, label, execPath)

	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return fmt.Errorf("falha ao escrever o LaunchAgent: %w", err)
	}

	// `bootstrap` (não o `load` legado, removido no macOS moderno)
	// ativa o LaunchAgent na sessão atual sem precisar de um novo
	// login pra valer - falha silenciosa aqui (ex: já bootstrapped)
	// não deve quebrar o pareamento, o arquivo .plist já garante o
	// próximo login de qualquer forma.
	uid := fmt.Sprintf("%d", os.Getuid())
	_ = exec.Command("launchctl", "bootstrap", "gui/"+uid, path).Run()

	return nil
}
