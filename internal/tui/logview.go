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
	buf       *buffer.Buffer
	filter    *filter.Filter
	follow    bool
	topNum    uint64 // Num da primeira linha visível no viewport (estável sob wrap)
	cursorNum uint64 // Num da linha selecionada/destacada; em follow, segue sempre a mais nova
	hOffset   int    // scroll horizontal, em células, para linhas mais largas que a tela
	hideNoise bool   // tecla 'n': esconde mensagens de lifecycle (filter.IsNoise)
	dirty     bool   // força rebuild do índice filtrado
	cachedV   uint64 // versão do buffer na última refresh
	snap      []model.Entry
	idx       []int // índices (em snap) que passam no filtro, em ordem
	painter   *Painter
	lastPos   int // faixa [lastPos, lastEnd) de linhas do último Draw, usada
	lastEnd   int // por maxHOffset para saber o que está visível agora

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

// HideNoise indica se o toggle de silenciar ruído (tecla 'n') está ativo.
func (v *LogView) HideNoise() bool { return v.hideNoise }

// ToggleHideNoise alterna o toggle de silenciar ruído e devolve o novo estado.
func (v *LogView) ToggleHideNoise() bool {
	v.hideNoise = !v.hideNoise
	v.dirty = true
	return v.hideNoise
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

// ScrollBy move o cursor de seleção em delta linhas filtradas (não a
// viewport diretamente): a viewport só rola quando o cursor sai da área
// visível (ver ensureCursorVisible). Subir desativa o follow — nesse caso o
// cursor parte de onde estava a linha mais nova (o que se via ao seguir),
// para o movimento parecer contínuo em vez de saltar.
func (v *LogView) ScrollBy(delta int) {
	v.refresh()
	if len(v.idx) == 0 {
		return
	}
	pos := v.cursorPosition()
	pos += delta
	if pos < 0 {
		pos = 0
	}
	if maxPos := len(v.idx) - 1; pos > maxPos {
		pos = maxPos
	}
	v.cursorNum = v.snap[v.idx[pos]].Num
	if delta < 0 {
		v.setFollow(false)
	}
	v.ensureCursorVisible()
}

// ScrollToTop vai ao início (desativa follow) e move o cursor para lá.
func (v *LogView) ScrollToTop() {
	v.refresh()
	if len(v.idx) > 0 {
		v.topNum = v.snap[v.idx[0]].Num
		v.cursorNum = v.topNum
	}
	v.setFollow(false)
}

// HorizScrollStep é o número de células deslocadas por pressão de ←/→.
const HorizScrollStep = 8

// ScrollX desloca o scroll horizontal em delta células (negativo = esquerda).
// Não afeta o follow nem a âncora vertical. Trava em 0 à esquerda e na
// largura da maior linha visível à direita — sem isso, dava pra rolar
// indefinidamente para além do fim de todas as linhas na tela.
func (v *LogView) ScrollX(delta int) {
	v.hOffset += delta
	if v.hOffset < 0 {
		v.hOffset = 0
	}
	if clamp := v.maxHOffset(); v.hOffset > clamp {
		v.hOffset = clamp
	}
}

// maxHOffset devolve até onde o scroll horizontal pode ir: a largura da
// linha mais larga na faixa visível do último Draw, menos a largura da
// viewport (nunca negativo).
func (v *LogView) maxHOffset() int {
	_, _, w, _ := v.Box.GetInnerRect()
	maxLine := 0
	for row := v.lastPos; row < v.lastEnd && row < len(v.idx); row++ {
		if lw := v.painter.LineWidth(&v.snap[v.idx[row]]); lw > maxLine {
			maxLine = lw
		}
	}
	return max(maxLine-w, 0)
}

// HOffset devolve o scroll horizontal atual (para exibição na status bar).
func (v *LogView) HOffset() int { return v.hOffset }

// ScrollToBottom vai ao fim, reativa o follow e move o cursor pra lá.
func (v *LogView) ScrollToBottom() {
	v.refresh()
	if len(v.idx) > 0 {
		last := v.snap[v.idx[len(v.idx)-1]].Num
		v.topNum = last
		v.cursorNum = last
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
			if v.filter != nil && !v.filter.Match(v.snap[i]) {
				continue
			}
			if v.hideNoise && filter.IsNoise(v.snap[i].Text) {
				continue
			}
			v.idx = append(v.idx, i)
		}
		v.dirty = false
	}
}

// anchorPosition devolve a posição (no índice filtrado) do topo da viewport.
func (v *LogView) anchorPosition() int {
	return searchNum(v.snap, v.idx, v.topNum)
}

// cursorPosition devolve a posição (no índice filtrado) da linha
// selecionada. Em follow, é sempre a última (a mais nova) — é dali que um
// scroll pra cima parte ao sair do follow.
func (v *LogView) cursorPosition() int {
	if v.follow {
		return len(v.idx) - 1
	}
	return searchNum(v.snap, v.idx, v.cursorNum)
}

// ensureCursorVisible rola a viewport (topNum) o mínimo necessário para que
// a linha do cursor volte a ficar visível, em vez de recentralizar — mesmo
// comportamento de um pager comum (less, vim).
func (v *LogView) ensureCursorVisible() {
	_, _, _, h := v.Box.GetInnerRect()
	if h <= 0 || len(v.idx) == 0 {
		return
	}
	cpos := v.cursorPosition()
	tpos := v.anchorPosition()
	switch {
	case cpos < tpos:
		v.topNum = v.snap[v.idx[cpos]].Num
	case cpos > tpos+h-1:
		np := max(cpos-(h-1), 0)
		v.topNum = v.snap[v.idx[np]].Num
	}
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

// Selected devolve a entrada sob o cursor: a mais recente em follow, ou a
// linha destacada por ↑/↓/PgUp/PgDn/g/G quando pausado — a mesma linha
// destacada em Draw e usada pelo Enter para abrir o detalhe.
func (v *LogView) Selected() (model.Entry, bool) {
	v.refresh()
	if len(v.idx) == 0 {
		return model.Entry{}, false
	}
	pos := max(v.cursorPosition(), 0)
	if pos >= len(v.idx) {
		pos = len(v.idx) - 1
	}
	return v.snap[v.idx[pos]], true
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
		v.cursorNum = v.snap[v.idx[len(v.idx)-1]].Num
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

	cursorPos := v.cursorPosition()
	end := min(pos+h, len(v.idx))
	v.lastPos, v.lastEnd = pos, end
	for row := pos; row < end; row++ {
		e := &v.snap[v.idx[row]]
		v.painter.PaintLine(screen, e, x, y+row-pos, w, v.hOffset, row == cursorPos, needle)
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
