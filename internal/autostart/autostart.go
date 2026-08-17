// Package autostart liga o Agent pra abrir sozinho no login do sistema
// (macOS/Windows - ver SPEC-025, "operador nunca deveria precisar de
// lançamento manual"). Ficou bloqueado até `HideWindowOnClose`/
// `SingleInstanceLock` existirem (ver app.go/main.go) - antes disso,
// abrir escondido deixaria o Agent inalcançável sem uma bandeja do
// sistema funcionando (3 tentativas travaram, ver memória da Spec).
// Agora abrir minimizado é seguro: fechar não mata o processo, e
// reabrir o app (ícone/atalho) traz a janela de volta.
package autostart

// Enable é chamado uma vez, depois de um pareamento bem-sucedido (ver
// internal/service.Service.Pair) - o pareamento é o sinal de "isto é
// uma instalação de produção de verdade", não um teste local qualquer.
// Idempotente (chamar de novo com o Agent já habilitado não duplica
// nada). Erros aqui nunca devem impedir o pareamento de completar -
// autostart é uma comodidade, não um requisito pro Agent funcionar.
func Enable() error {
	return enable()
}
