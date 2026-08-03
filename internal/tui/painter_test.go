package tui

import (
	"strings"
	"testing"

	"github.com/Morpa/kizashi/internal/model"
	"github.com/gdamore/tcell/v2"
)

// cellsRunes concatena os runes das células para facilitar a asserção.
func cellsRunes(cells []cell) string {
	var b strings.Builder
	for _, c := range cells {
		b.WriteRune(c.r)
	}
	return b.String()
}

func TestCellsForJSON(t *testing.T) {
	p := NewPainter(DefaultPalette())
	e := &model.Entry{
		IsJSON:  true,
		Time:    "2026-01-02 03:04:05.000",
		Service: "api",
		Level:   model.LevelError,
		Text:    "boom",
	}
	got := cellsRunes(p.cellsFor(e, ""))
	want := "2026-01-02 03:04:05.000 api                  ERROR boom"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCellsForJSONMinimal(t *testing.T) {
	p := NewPainter(DefaultPalette())
	e := &model.Entry{IsJSON: true, Level: model.LevelInfo, Text: "msg"}
	// LevelInfo.String() tem largura fixa: "INFO " (com espaço no fim)
	got := cellsRunes(p.cellsFor(e, ""))
	if got != "INFO  msg" {
		t.Errorf("got %q", got)
	}
}

func TestCellsForPlainText(t *testing.T) {
	p := NewPainter(DefaultPalette())
	e := &model.Entry{Text: "linha simples"}
	if got := cellsRunes(p.plainCells(e)); got != "linha simples" {
		t.Errorf("got %q", got)
	}
}

func TestPlainCellsANSIPreserved(t *testing.T) {
	p := NewPainter(DefaultPalette())
	e := &model.Entry{Text: "\x1b[31mred\x1b[0m plain"}
	got := cellsRunes(p.plainCells(e))
	if got != "red plain" {
		t.Errorf("got %q", got)
	}
}

func TestPadService(t *testing.T) {
	if got := padService("api", 6); got != "api   " {
		t.Errorf("got %q", got)
	}
	if got := padService("be/general-rest-api", 10); got != "be/genera…" {
		t.Errorf("got %q", got)
	}
	if got := padService("ok", 2); got != "ok" {
		t.Errorf("got %q", got)
	}
}

func TestCellsForPlainTextWithService(t *testing.T) {
	p := NewPainter(DefaultPalette())
	e := &model.Entry{Service: "ui", Text: "linha simples"}
	got := cellsRunes(p.cellsFor(e, ""))
	want := padService("ui", serviceWidth) + " linha simples"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHTTPLineColored(t *testing.T) {
	p := NewPainter(DefaultPalette())
	e := &model.Entry{Text: "GET /api/auth/get-session 200 in 105ms"}
	cells := p.plainCells(e)
	got := cellsRunes(cells)
	if got != e.Text {
		t.Fatalf("got %q, want %q", got, e.Text)
	}
	// os 3 dígitos do status devem estar na cor de sucesso (info).
	statusIdx := strings.Index(got, "200")
	for i := statusIdx; i < statusIdx+3; i++ {
		fg, _, _ := cells[i].st.Decompose()
		if fg != p.pal.LevelColor[model.LevelInfo] {
			t.Errorf("status[%d] fg = %v, want %v", i, fg, p.pal.LevelColor[model.LevelInfo])
		}
	}
}

func TestHTTPLineANSIPreserved(t *testing.T) {
	p := NewPainter(DefaultPalette())
	e := &model.Entry{Text: "\x1b[31mGET /path 500 in 1ms\x1b[0m"}
	got := cellsRunes(p.plainCells(e))
	if got != "GET /path 500 in 1ms" {
		t.Errorf("got %q", got)
	}
}

func TestHighlightNeedle(t *testing.T) {
	p := NewPainter(DefaultPalette())
	e := &model.Entry{IsJSON: true, Text: "slow query"}
	cells := p.cellsFor(e, "slow")
	highlighted := 0
	for _, c := range cells {
		if _, _, attr := c.st.Decompose(); attr&tcell.AttrReverse != 0 {
			highlighted++
		}
	}
	// "slow" = 4 runes destacados
	if highlighted != 4 {
		t.Errorf("esperava 4 células destacadas, got %d", highlighted)
	}
}

func TestMatchRanges(t *testing.T) {
	r := matchRanges("Hello error world", "error")
	if len(r) != 1 || r[0] != [2]int{6, 11} {
		t.Errorf("got %v", r)
	}
	if r := matchRanges("olá mundo", "á"); r != nil {
		t.Errorf("não-ASCII deveria retornar nil, got %v", r)
	}
	if r := matchRanges("abc", ""); r != nil {
		t.Errorf("needle vazio deveria retornar nil, got %v", r)
	}
}

func TestSearchNum(t *testing.T) {
	snap := []model.Entry{{Num: 2}, {Num: 5}, {Num: 7}}
	idx := []int{0, 1, 2}
	cases := []struct {
		num  uint64
		want int
	}{
		{5, 1}, {6, 2}, {1, 0}, {8, 3}, {2, 0},
	}
	for _, c := range cases {
		if got := searchNum(snap, idx, c.num); got != c.want {
			t.Errorf("searchNum(%d) = %d, want %d", c.num, got, c.want)
		}
	}
}
