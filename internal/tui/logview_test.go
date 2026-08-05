package tui

import (
	"testing"

	"github.com/Morpa/kizashi/internal/buffer"
	"github.com/Morpa/kizashi/internal/model"
	"github.com/gdamore/tcell/v2"
)

func newTestScreen(t *testing.T) tcell.SimulationScreen {
	t.Helper()
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sim.Fini)
	return sim
}

// TestLogViewCursorFollowsNewestThenScrollsIndependently cobre a regressão
// que motivou o cursor de seleção: antes, Enter abria sempre a linha do topo
// da viewport (a mais antiga na tela em follow), não a mais recente.
func TestLogViewCursorFollowsNewestThenScrollsIndependently(t *testing.T) {
	buf := buffer.New(100)
	for i := uint64(1); i <= 5; i++ {
		buf.Append(model.Entry{Num: i, Text: "linha"})
	}

	lv := NewLogView(buf, NewPainter(DefaultPalette()))
	lv.SetRect(0, 0, 20, 7) // rect com borda; inner rect menor que o total de linhas
	sim := newTestScreen(t)

	lv.Draw(sim) // follow=true: cursor deve ir para a mais nova (Num=5)
	e, ok := lv.Selected()
	if !ok || e.Num != 5 {
		t.Fatalf("em follow, Selected() = Num %d, want 5", e.Num)
	}

	lv.ScrollBy(-1) // simula ↑: sai do follow, cursor parte da mais nova e sobe uma
	if lv.Follow() {
		t.Fatal("ScrollBy(-1) deveria desativar o follow")
	}
	e, ok = lv.Selected()
	if !ok || e.Num != 4 {
		t.Fatalf("após ScrollBy(-1), Selected() = Num %d, want 4", e.Num)
	}

	// Redesenhar pausado não deve mover o cursor de volta ao topo da viewport
	// (o bug que motivou esta mudança).
	lv.Draw(sim)
	e, ok = lv.Selected()
	if !ok || e.Num != 4 {
		t.Fatalf("após Draw pausado, Selected() = Num %d, want 4 (cursor não deveria voltar ao topo)", e.Num)
	}
}

func TestLogViewHideNoise(t *testing.T) {
	buf := buffer.New(100)
	texts := []string{"get-exam-start", "request-done", "mensagem normal", "outra coisa"}
	for i, txt := range texts {
		buf.Append(model.Entry{Num: uint64(i + 1), Text: txt})
	}

	lv := NewLogView(buf, NewPainter(DefaultPalette()))
	lv.SetRect(0, 0, 40, 10)

	if _, tot := lv.Counts(); tot != 4 {
		t.Fatalf("total = %d, want 4", tot)
	}
	if vis, _ := lv.Counts(); vis != 4 {
		t.Fatalf("sem hideNoise, visible = %d, want 4", vis)
	}

	lv.ToggleHideNoise()
	if vis, _ := lv.Counts(); vis != 2 {
		t.Fatalf("com hideNoise, visible = %d, want 2 (só as 2 que não terminam em -start/-done)", vis)
	}

	lv.ToggleHideNoise()
	if vis, _ := lv.Counts(); vis != 4 {
		t.Fatalf("após religar, visible = %d, want 4", vis)
	}
}

func TestLogViewScrollXClamp(t *testing.T) {
	buf := buffer.New(10)
	longText := "esta e uma linha bem mais larga do que a viewport de teste vai mostrar"
	buf.Append(model.Entry{Num: 1, Text: longText})

	painter := NewPainter(DefaultPalette())
	lv := NewLogView(buf, painter)
	lv.SetRect(0, 0, 20, 5) // inner width = 18 (2 de borda)
	sim := newTestScreen(t)
	lv.Draw(sim) // popula lastPos/lastEnd usados pelo clamp

	lineWidth := painter.LineWidth(&model.Entry{Text: longText})
	_, _, innerW, _ := lv.Box.GetInnerRect()
	wantMax := max(lineWidth-innerW, 0)

	lv.ScrollX(100000) // tenta rolar bem além do fim da linha
	if got := lv.HOffset(); got != wantMax {
		t.Fatalf("HOffset() = %d, want %d (clamp na largura da linha)", got, wantMax)
	}

	lv.ScrollX(-100000) // trava em 0 do outro lado
	if got := lv.HOffset(); got != 0 {
		t.Fatalf("HOffset() = %d, want 0", got)
	}
}
