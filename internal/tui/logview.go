package tui

import (
	"github.com/Morpa/kizashi/internal/buffer"
	"github.com/Morpa/kizashi/internal/filter"
	"github.com/Morpa/kizashi/internal/model"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// LogView é uma primitiva tview custom que renderiza apenas as linhas
// visíveis do buffer (virtualização), é dona do estado de follow/scroll e
// aplica o filtro em tempo de render.
type LogView struct {
	*tview.Box
	buf     *buffer.Buffer
	filter  *filter.Filter
	follow  bool
	topNum  uint64 // âncora: Num da primeira linha visível (estável sob wrap)
	dirty   bool   // força rebuild do índice filtrado
	cachedV uint64 // versão do buffer na última refresh
	snap    []model.Entry
	idx     []int // índices (em snap) que passam no filtro, em ordem
	painter *Painter

	// OnFollowChange, se definido, é chamado quando o estado de follow muda
	// (pausa/retomada por teclas), permitindo ao app atualizar a status bar
	// na hora em vez de esperar o próximo redraw de dados.
	OnFollowChange func(follow bool)
}

func NewLogView(buf *buffer.Buffer, painter *Painter) *LogView {
	l := &LogView{
		Box:     tview.NewBox(),
		buf:     buf,
		follow:  true,
		dirty:   true,
		painter: painter,
	}
	l.SetBorder(true)
	l.SetTitle(" logs ")
	l.SetBorderAttributes(tcell.AttrDim)
	return l
}

// Filter devolve o filtro ativo.
func (v *LogView) Filter() *filter.Filter { return v.filter }

// SetFilter troca o filtro ativo (reconstrói o índice no próximo Draw).
func (v *LogView) SetFilter(f *filter.Filter) {
	v.filter = f
	v.dirty = true
}

// Follow indica se o view está seguindo o fim da stream.
func (v *LogView) Follow() bool { return v.follow }

// SetFollow liga/desliga o follow.
func (v *LogView) SetFollow(on bool) { v.setFollow(on) }

// setFollow muda o follow e notifica OnFollowChange em cada transição.
func (v *LogView) setFollow(on bool) {
	if v.follow == on {
		return
	}
	v.follow = on
	if v.OnFollowChange != nil {
		v.OnFollowChange(on)
	}
}

// ToggleFollow alterna o follow e devolve o novo estado.
func (v *LogView) ToggleFollow() bool {
	v.setFollow(!v.follow)
	if v.follow {
		v.ScrollToBottom()
	}
	return v.follow
}

// ScrollBy desloca a âncora em delta linhas filtradas. Subir desativa o follow.
func (v *LogView) ScrollBy(delta int) {
	v.refresh()
	if len(v.idx) == 0 {
		return
	}
	pos := v.anchorPosition()
	pos += delta
	if pos < 0 {
		pos = 0
	}
	if maxPos := len(v.idx) - 1; pos > maxPos {
		pos = maxPos
	}
	v.topNum = v.snap[v.idx[pos]].Num
	if delta < 0 {
		v.setFollow(false)
	}
}

// ScrollToTop vai ao início (desativa follow).
func (v *LogView) ScrollToTop() {
	v.refresh()
	if len(v.idx) > 0 {
		v.topNum = v.snap[v.idx[0]].Num
	}
	v.setFollow(false)
}

// ScrollToBottom vai ao fim e reativa o follow.
func (v *LogView) ScrollToBottom() {
	v.refresh()
	if len(v.idx) > 0 {
		v.topNum = v.snap[v.idx[len(v.idx)-1]].Num
	}
	v.setFollow(true)
}

// Counts devolve (visíveis filtradas, total no buffer).
func (v *LogView) Counts() (visible, total int) {
	v.refresh()
	return len(v.idx), len(v.snap)
}

// refresh re-sincroniza snapshot e índice filtrado se algo mudou.
func (v *LogView) refresh() {
	vv := v.buf.Version()
	if v.dirty || v.cachedV != vv {
		v.snap = v.buf.Snapshot()
		v.cachedV = vv
		v.idx = v.idx[:0]
		for i := range v.snap {
			if v.filter == nil || v.filter.Match(v.snap[i]) {
				v.idx = append(v.idx, i)
			}
		}
		v.dirty = false
	}
}

// anchorPosition devolve a posição (no índice filtrado) da linha âncora.
func (v *LogView) anchorPosition() int {
	return searchNum(v.snap, v.idx, v.topNum)
}

// searchNum devolve o menor j tal que snap[idx[j]].Num >= num.
func searchNum(snap []model.Entry, idx []int, num uint64) int {
	lo, hi := 0, len(idx)
	for lo < hi {
		mid := (lo + hi) / 2
		if snap[idx[mid]].Num < num {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// Draw implementa tview.Primitive.
func (v *LogView) Draw(screen tcell.Screen) {
	v.Box.DrawForSubclass(screen, v)
	x, y, w, h := v.Box.GetInnerRect()
	if w <= 0 || h <= 0 {
		return
	}
	v.refresh()
	if len(v.idx) == 0 {
		return
	}

	var pos int
	if v.follow {
		pos = max(len(v.idx)-h, 0)
		v.topNum = v.snap[v.idx[pos]].Num
	} else {
		pos = v.anchorPosition()
		if maxPos := len(v.idx) - h; pos > maxPos && maxPos > 0 {
			pos = maxPos
		}
	}

	needle := ""
	if v.filter != nil {
		needle = v.filter.Needle()
	}

	end := min(pos+h, len(v.idx))
	for row := pos; row < end; row++ {
		e := &v.snap[v.idx[row]]
		v.painter.PaintLine(screen, e, x, y+row-pos, w, needle)
	}
}

// InputHandler lida com a navegação quando o LogView está focado.
func (v *LogView) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return v.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		switch event.Key() {
		case tcell.KeyUp:
			v.ScrollBy(-1)
		case tcell.KeyDown:
			v.ScrollBy(1)
		case tcell.KeyPgUp:
			v.ScrollBy(-v.pageSize())
		case tcell.KeyPgDn:
			v.ScrollBy(v.pageSize())
		case tcell.KeyHome:
			v.ScrollToTop()
		case tcell.KeyEnd:
			v.ScrollToBottom()
		case tcell.KeyRune:
			switch event.Rune() {
			case 'g':
				v.ScrollToTop()
			case 'G':
				v.ScrollToBottom()
			}
		}
	})
}

func (v *LogView) pageSize() int {
	_, _, _, h := v.Box.GetInnerRect()
	if h < 1 {
		h = 1
	}
	return h
}
