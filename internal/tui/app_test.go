package tui

import (
	"strings"
	"testing"

	"github.com/Morpa/kizashi/internal/buffer"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// screenText reconstitui o texto da tela (sem cor) para asserções.
func screenText(t *testing.T, sim tcell.SimulationScreen) string {
	t.Helper()
	content, w, h := sim.GetContents()
	runes := make([]rune, 0, w*h)
	for i := 0; i < w*h; i++ {
		r := content[i].Runes[0]
		if r == 0 {
			r = ' '
		}
		runes = append(runes, r)
	}
	return string(runes)
}

// TestFilterModalShowsAndReceivesText cobre o regressão do "/": o modal de
// filtro precisa renderizar o InputField (e não apenas a moldura). Em
// tview v0.42.0, um Frame com borda só dimensiona o primitivo interno quando
// tem altura suficiente; antes o frame tinha altura 1 e o campo nunca era
// posicionado, ficando invisível.
func TestFilterModalShowsAndReceivesText(t *testing.T) {
	a := New(Options{Buffer: buffer.New(100), Sources: []string{"stdin"}})

	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	a.app.SetScreen(sim)

	a.buildUI()
	w, h := sim.Size()
	a.pages.SetRect(0, 0, w, h)
	a.pages.Draw(sim)
	sim.Show()

	// Abre o filtro com "/".
	if ev := a.app.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone)); ev != nil {
		t.Fatalf("o capturador deveria consumir '/', mas devolveu %v", ev)
	}
	if !a.filterOpen {
		t.Fatal("esperava filterOpen=true após '/'")
	}

	// Digita "abc" no InputField focado.
	handler := a.filterIn.InputHandler()
	for _, r := range "abc" {
		handler(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone), func(p tview.Primitive) {})
	}
	if got := a.filterIn.GetText(); got != "abc" {
		t.Fatalf("InputField deveria conter %q, contém %q", "abc", got)
	}

	// Redesenha e confere que o campo foi dimensionado (não ficou no tamanho
	// padrão 15x10 do tview) e que o título do modal aparece na tela.
	a.pages.Draw(sim)
	sim.Show()

	x, y, fw, fh := a.filterIn.GetRect()
	if fw <= 0 || fh <= 0 || (fw == 15 && fh == 10) {
		t.Errorf("InputField não foi dimensionado pelo modal (rect=%d,%d,%d,%d)", x, y, fw, fh)
	}

	text := screenText(t, sim)
	if !strings.Contains(text, "Filtro") {
		t.Errorf("título do modal 'Filtro' não aparece na tela")
	}
	if !strings.Contains(text, "abc") {
		t.Errorf("texto digitado 'abc' não aparece na tela")
	}
}
