// Package tui implementa a interface de terminal (tview).
package tui

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Morpa/kizashi/internal/buffer"
	"github.com/Morpa/kizashi/internal/filter"
	"github.com/Morpa/kizashi/internal/model"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ansiRe remove códigos de escape ANSI do texto exibido no painel de
// detalhe (o TextView do detalhe não interpreta cores dinâmicas).
var ansiRe = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")

// Options configura o App. As fontes (nomes) aparecem na status bar.
type Options struct {
	Buffer  *buffer.Buffer
	Sources []string
	// DoneCh é fechado quando a ingestão termina (sources EOF). Opcional.
	DoneCh <-chan struct{}
}

// App reúne a aplicação tview e seu estado.
type App struct {
	app        *tview.Application
	pages      *tview.Pages
	logView    *LogView
	status     *tview.TextView
	filterIn   *tview.InputField
	detailView *tview.TextView

	buf        *buffer.Buffer
	sources    []string
	doneCh     <-chan struct{}
	painter    *Painter
	pending    atomic.Bool // dados novos desde o último draw
	ingestEnd  atomic.Bool
	filterOpen bool
	detailOpen bool
}

// New cria o App. A UI é montada em Run.
func New(opts Options) *App {
	return &App{
		app:     tview.NewApplication(),
		buf:     opts.Buffer,
		sources: opts.Sources,
		doneCh:  opts.DoneCh,
		painter: NewPainter(DefaultPalette()),
	}
}

// Notify é chamado pelas goroutines de ingestão para sinalizar dados novos.
// Apenas marca um flag atômico; o redraw é feito pelo loop de render.
func (a *App) Notify() { a.pending.Store(true) }

// Run monta a UI, inicia o loop de render e bloqueia até o usuário sair.
func (a *App) Run() error {
	a.buildUI()

	stop := make(chan struct{})
	defer close(stop)
	go a.redrawLoop(stop)

	if a.doneCh != nil {
		go func() {
			<-a.doneCh
			a.app.QueueUpdateDraw(func() {
				a.ingestEnd.Store(true)
				a.renderStatus()
			})
		}()
	}

	return a.app.Run()
}

// buildUI monta o layout: página principal (LogView + status) e o modal de
// filtro (InputField) em uma página separada.
func (a *App) buildUI() {
	a.logView = NewLogView(a.buf, a.painter)
	// Mudanças de follow por teclas (g/G/End/setas) refletem na status bar
	// imediatamente, sem esperar o próximo redraw de dados.
	a.logView.OnFollowChange = func(bool) { a.renderStatus() }

	a.status = tview.NewTextView()
	a.status.SetDynamicColors(true)

	main := tview.NewFlex().SetDirection(tview.FlexRow)
	main.AddItem(a.logView, 0, 1, true)
	main.AddItem(a.status, 1, 0, false)

	a.filterIn = tview.NewInputField()
	a.filterIn.SetLabel("Filtro: ")
	a.filterIn.SetFieldBackgroundColor(tview.Styles.ContrastBackgroundColor)
	a.filterIn.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			a.applyFilter()
		case tcell.KeyEsc:
			a.cancelFilter()
		}
	})
	frame := tview.NewFrame(a.filterIn)
	frame.SetBorder(true)
	frame.SetTitle(" Filtro — Enter aplica, Esc limpa ")
	frame.SetBorderAttributes(tcell.AttrDim)
	// Um Frame com borda no tview só dimensiona o primitivo interno quando tem
	// pelo menos 6 linhas (2 de borda + padding interno de 1 em cima/embaixo).
	// Com menos, ele desenha só a moldura e o InputField nunca é posicionado.
	modalHeight := 6

	// centraliza o modal vertical e horizontalmente.
	modal := tview.NewFlex().SetDirection(tview.FlexRow)
	modal.AddItem(nil, 0, 1, false)
	modal.AddItem(tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(frame, 60, 0, true).
		AddItem(nil, 0, 1, false), modalHeight, 0, true)
	modal.AddItem(nil, 0, 1, false)

	a.detailView = tview.NewTextView()
	a.detailView.SetDynamicColors(false)
	a.detailView.SetWrap(true)
	a.detailView.SetScrollable(true)
	a.detailView.SetBorder(true)
	a.detailView.SetTitle(" detalhe — Esc/q fecha ")
	a.detailView.SetBorderAttributes(tcell.AttrDim)

	a.pages = tview.NewPages()
	a.pages.AddPage("main", main, true, true)
	a.pages.AddPage("filter", modal, true, false)
	a.pages.AddPage("detail", a.detailView, true, false)

	a.app.SetRoot(a.pages, true)
	a.app.SetFocus(a.logView)
	a.setupKeys()
	a.renderStatus() // status bar visível já no boot, antes de qualquer dado
}

// setupKeys registra o capturador global de teclas. Quando o modal de filtro
// está aberto, todos os eventos passam para o InputField.
func (a *App) setupKeys() {
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if a.filterOpen {
			return event
		}
		if a.detailOpen {
			switch {
			case event.Key() == tcell.KeyEsc:
				a.closeDetail()
				return nil
			case event.Key() == tcell.KeyRune && event.Rune() == 'q':
				a.closeDetail()
				return nil
			}
			// setas/PgUp/PgDn passam para o TextView de detalhe rolar o conteúdo.
			return event
		}
		switch event.Key() {
		case tcell.KeyCtrlC:
			a.app.Stop()
			return nil
		case tcell.KeyEsc:
			if f := a.logView.Filter(); f != nil && !f.IsEmpty() {
				a.logView.SetFilter(nil)
				a.renderStatus()
			}
			return nil
		case tcell.KeyEnter:
			a.openDetail()
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'q':
				a.app.Stop()
				return nil
			case '/':
				a.openFilter()
				return nil
			case 'f', ' ':
				a.logView.ToggleFollow() // OnFollowChange atualiza a status bar
				return nil
			case 'n':
				a.logView.ToggleHideNoise()
				a.renderStatus()
				return nil
			}
		case tcell.KeyLeft:
			a.logView.ScrollX(-HorizScrollStep)
			a.renderStatus()
			return nil
		case tcell.KeyRight:
			a.logView.ScrollX(HorizScrollStep)
			a.renderStatus()
			return nil
		}
		// PageUp/PageDown/Home/End/↑/↓/g/G → LogView
		return event
	})
}

// openDetail mostra o registro completo (JSON formatado, ou texto cru sem
// ANSI) da linha selecionada — a mesma âncora usada pelo scroll.
func (a *App) openDetail() {
	e, ok := a.logView.Selected()
	if !ok {
		return
	}
	a.detailView.SetText(detailText(&e))
	a.detailView.ScrollToBeginning()
	a.detailOpen = true
	a.pages.ShowPage("detail")
	a.app.SetFocus(a.detailView)
}

func (a *App) closeDetail() {
	a.detailOpen = false
	a.pages.HidePage("detail")
	a.app.SetFocus(a.logView)
}

// detailText monta o texto do painel de detalhe: JSON formatado quando a
// linha é estruturada, senão a linha crua sem códigos ANSI.
func detailText(e *model.Entry) string {
	if e.IsJSON {
		if b, err := json.MarshalIndent(e.Fields, "", "  "); err == nil {
			return string(b)
		}
	}
	return ansiRe.ReplaceAllString(e.Raw, "")
}

func (a *App) openFilter() {
	if f := a.logView.Filter(); f != nil {
		a.filterIn.SetText(f.Text)
	} else {
		a.filterIn.SetText("")
	}
	a.filterOpen = true
	a.pages.ShowPage("filter")
	a.app.SetFocus(a.filterIn)
}

func (a *App) applyFilter() {
	a.logView.SetFilter(filter.Parse(a.filterIn.GetText()))
	a.filterOpen = false
	a.pages.HidePage("filter")
	a.app.SetFocus(a.logView)
	a.renderStatus()
}

func (a *App) cancelFilter() {
	a.logView.SetFilter(nil)
	a.filterIn.SetText("")
	a.filterOpen = false
	a.pages.HidePage("filter")
	a.app.SetFocus(a.logView)
	a.renderStatus()
}

// renderStatus monta a linha de status. Roda sempre na main goroutine.
func (a *App) renderStatus() {
	var sb strings.Builder

	if a.logView.Follow() {
		sb.WriteString("[green]● SEGUINDO[-]")
	} else {
		sb.WriteString("[yellow]⏸ PAUSADO[-]")
	}

	vis, tot := a.logView.Counts()
	fmt.Fprintf(&sb, "  %d/%d", vis, tot)

	if d := a.buf.Dropped(); d > 0 {
		fmt.Fprintf(&sb, "  [red](%d descartados)[-]", d)
	}
	if f := a.logView.Filter(); f != nil && !f.IsEmpty() {
		fmt.Fprintf(&sb, "  [::d]filtro: %s[-:-:-]", tview.Escape(f.Text))
	}
	if off := a.logView.HOffset(); off > 0 {
		fmt.Fprintf(&sb, "  [::d]«scroll: %d[-:-:-]", off)
	}
	if a.logView.HideNoise() {
		sb.WriteString("  [::d]ruído oculto (n)[-]")
	}
	fmt.Fprintf(&sb, "  [::d]fontes: %s[-:-:-]", strings.Join(a.sources, ", "))
	if a.ingestEnd.Load() {
		sb.WriteString("  [::d][fim da ingestão][-:-:-]")
	}

	a.status.SetText(sb.String())
}

// redrawLoop coalesce os sinais de ingestão e agenda um redraw a cada ~80 ms.
func (a *App) redrawLoop(stop <-chan struct{}) {
	tick := time.NewTicker(time.Second / 12)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			if a.pending.Swap(false) {
				a.app.QueueUpdateDraw(func() { a.renderStatus() })
			}
		}
	}
}
