package tui

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Morpa/kizashi/internal/model"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// Palette define as cores usadas na renderização.
type Palette struct {
	LevelColor [model.LevelFatal + 1]tcell.Color
	Time       tcell.Color
	Service    tcell.Color
	Highlight  tcell.Style
}

// DefaultPalette devolve a paleta padrão.
func DefaultPalette() Palette {
	return Palette{
		LevelColor: [model.LevelFatal + 1]tcell.Color{
			model.LevelNone:  tcell.ColorDefault,
			model.LevelTrace: tcell.ColorGray,
			model.LevelDebug: tcell.ColorCornflowerBlue,
			model.LevelInfo:  tcell.ColorGreen,
			model.LevelWarn:  tcell.ColorYellow,
			model.LevelError: tcell.ColorRed,
			model.LevelFatal: tcell.ColorRed,
		},
		Time:      tcell.ColorGray,
		Service:   tcell.ColorDarkMagenta,
		Highlight: tcell.StyleDefault.Reverse(true),
	}
}

// cell é um rune pronto para pintura, com seu estilo.
type cell struct {
	r  rune
	st tcell.Style
}

// Painter desenha linhas de log na tela, célula a célula, sem interpretar
// tags de estilo (texto de log pode conter "[" à vontade).
type Painter struct {
	pal Palette
}

func NewPainter(pal Palette) *Painter { return &Painter{pal: pal} }

// PaintLine desenha a entrada na posição (x, y), truncando em maxW células.
func (p *Painter) PaintLine(screen tcell.Screen, e *model.Entry, x, y, maxW int, needle string) {
	if maxW <= 0 {
		return
	}
	cells := p.cellsFor(e, needle)
	cx := x
	truncated := false
	for _, c := range cells {
		w := runewidth.RuneWidth(c.r)
		if cx+w > x+maxW {
			truncated = true
			break
		}
		screen.SetContent(cx, y, c.r, nil, c.st)
		cx += w
	}
	if truncated && cx < x+maxW {
		screen.SetContent(cx, y, '…', nil, tcell.StyleDefault)
	}
}

// cellsFor monta as células de uma entrada: para JSON, segmentos
// tempo/serviço/nível/mensagem com cores; para texto puro, parse de ANSI.
func (p *Painter) cellsFor(e *model.Entry, needle string) []cell {
	if !e.IsJSON {
		return p.plainCells(e)
	}

	timeStyle := tcell.StyleDefault.Foreground(p.pal.Time)
	svcStyle := tcell.StyleDefault.Foreground(p.pal.Service)
	lvlStyle := levelStyle(p.pal, e.Level)

	var cells []cell
	if e.Time != "" {
		cells = appendCells(cells, e.Time, timeStyle)
		cells = append(cells, cell{' ', timeStyle})
	}
	if e.Service != "" {
		cells = appendCells(cells, e.Service, svcStyle)
		cells = append(cells, cell{' ', svcStyle})
	}
	if e.Level != model.LevelNone {
		cells = appendCells(cells, e.Level.String(), lvlStyle)
		cells = append(cells, cell{' ', lvlStyle})
	}
	return appendCellsHighlight(cells, e.Text, lvlStyle, needle, p.pal.Highlight)
}

// plainCells renderiza texto puro: se houver ANSI, as cores reais do app
// prevalecem; senão, usa a cor do nível heurístico (ou a padrão).
func (p *Painter) plainCells(e *model.Entry) []cell {
	segs := ansiSegments(e.Text)
	if len(segs) == 0 {
		return runesToCells(e.Text, levelStyle(p.pal, e.Level))
	}
	var cells []cell
	for _, s := range segs {
		cells = appendCells(cells, s.text, s.style)
	}
	return cells
}

// levelStyle devolve o estilo de uma linha segundo seu nível.
func levelStyle(pal Palette, lvl model.Level) tcell.Style {
	st := tcell.StyleDefault.Foreground(pal.LevelColor[lvl])
	if lvl == model.LevelFatal {
		st = st.Bold(true)
	}
	return st
}

func runesToCells(s string, st tcell.Style) []cell {
	cells := make([]cell, 0, len(s))
	for _, r := range s {
		cells = append(cells, cell{r, st})
	}
	return cells
}

func appendCells(cells []cell, s string, st tcell.Style) []cell {
	for _, r := range s {
		cells = append(cells, cell{r, st})
	}
	return cells
}

// appendCellsHighlight aplica o estilo de destaque às ocorrências de needle.
func appendCellsHighlight(cells []cell, s string, st tcell.Style, needle string, hl tcell.Style) []cell {
	ranges := matchRanges(s, needle)
	if len(ranges) == 0 {
		return appendCells(cells, s, st)
	}
	b := 0
	for _, r := range s {
		style := st
		for _, rg := range ranges {
			if b >= rg[0] && b < rg[1] {
				style = hl
				break
			}
		}
		cells = append(cells, cell{r, style})
		b += utf8.RuneLen(r)
	}
	return cells
}

// matchRanges devolve os intervalos [início, fim) em bytes onde needle
// ocorre em s (case-insensitive). Só para ASCII (case folding não altera
// offsets de bytes). Retorna nil se needle vazio ou não-ASCII.
func matchRanges(s, needle string) [][2]int {
	if needle == "" || !isASCII(s) || !isASCII(needle) {
		return nil
	}
	lower := strings.ToLower(s)
	sub := strings.ToLower(needle)
	var out [][2]int
	start := 0
	for {
		i := strings.Index(lower[start:], sub)
		if i < 0 {
			return out
		}
		pos := start + i
		out = append(out, [2]int{pos, pos + len(needle)})
		start = pos + len(needle)
	}
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// seg é um trecho de texto com um estilo (resultado do parse de ANSI).
type seg struct {
	text  string
	style tcell.Style
}

// ansiSegments divide a string em trechos estilizados segundo códigos SGR
// (\x1b[...m). Cores 30–37/90–97 (fundo 40–47/100–107), bold e reverse.
func ansiSegments(s string) []seg {
	if !strings.ContainsRune(s, '\x1b') {
		return nil
	}
	var out []seg
	style := tcell.StyleDefault
	idx := 0
	for i := 0; i < len(s); {
		if s[i] != '\x1b' || i+1 >= len(s) || s[i+1] != '[' {
			i++
			continue
		}
		// encontra o final da sequência (byte 'm' precedido de parâmetros)
		end := strings.IndexByte(s[i+2:], 'm')
		if end < 0 {
			break
		}
		end += i + 2 // posição do 'm'
		if idx < i {
			out = append(out, seg{text: s[idx:i], style: style})
		}
		style = applySGR(style, s[i+2:end])
		idx = end + 1
		i = end + 1
	}
	if idx < len(s) {
		out = append(out, seg{text: s[idx:], style: style})
	}
	return out
}

func applySGR(st tcell.Style, params string) tcell.Style {
	if params == "" {
		return st
	}
	for p := range strings.SplitSeq(params, ";") {
		switch p {
		case "0":
			st = tcell.StyleDefault
		case "1":
			st = st.Bold(true)
		case "7":
			st = st.Reverse(true)
		case "39":
			st = st.Foreground(tcell.ColorDefault)
		case "49":
			st = st.Background(tcell.ColorDefault)
		default:
			n, err := strconv.Atoi(p)
			if err != nil {
				continue
			}
			switch {
			case n >= 30 && n <= 37:
				st = st.Foreground(tcell.PaletteColor(n - 30))
			case n >= 90 && n <= 97:
				st = st.Foreground(tcell.PaletteColor(n - 90 + 8))
			case n >= 40 && n <= 47:
				st = st.Background(tcell.PaletteColor(n - 40))
			case n >= 100 && n <= 107:
				st = st.Background(tcell.PaletteColor(n - 100 + 8))
			}
		}
	}
	return st
}
