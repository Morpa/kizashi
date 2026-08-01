package buffer

import (
	"fmt"
	"sync"
	"testing"

	"github.com/Morpa/kizashi/internal/model"
)

func TestAppendSnapshotOrder(t *testing.T) {
	b := New(3)
	for i := range 3 {
		b.Append(model.Entry{Num: uint64(i), Text: fmt.Sprintf("m%d", i)})
	}
	snap := b.Snapshot()
	if len(snap) != 3 {
		t.Fatalf("len = %d", len(snap))
	}
	for i, e := range snap {
		if e.Num != uint64(i) {
			t.Errorf("snap[%d].Num = %d", i, e.Num)
		}
	}
}

func TestDropOldest(t *testing.T) {
	b := New(2)
	b.Append(model.Entry{Num: 1})
	b.Append(model.Entry{Num: 2})
	b.Append(model.Entry{Num: 3}) // descarta 1
	if b.Dropped() != 1 {
		t.Errorf("Dropped = %d", b.Dropped())
	}
	if b.Len() != 2 {
		t.Errorf("Len = %d", b.Len())
	}
	snap := b.Snapshot()
	if len(snap) != 2 || snap[0].Num != 2 || snap[1].Num != 3 {
		t.Errorf("snap = %+v", snap)
	}
}

func TestVersionBumps(t *testing.T) {
	b := New(2)
	v0 := b.Version()
	b.Append(model.Entry{})
	if b.Version() == v0 {
		t.Error("version deveria subir após Append")
	}
}

func TestNewClampsCapacity(t *testing.T) {
	b := New(0)
	if b.Len() != 0 {
		t.Error("deveria começar vazio")
	}
	b.Append(model.Entry{Num: 1})
	if b.Len() != 1 {
		t.Error("cap mínimo 1 deveria aceitar 1 entrada")
	}
}

func TestConcurrentAppend(t *testing.T) {
	b := New(100)
	var wg sync.WaitGroup
	for g := range 10 {
		wg.Add(1)
		go func(base uint64) {
			defer wg.Done()
			for i := range 100 {
				b.Append(model.Entry{Num: base + uint64(i)})
			}
		}(uint64(g) * 100)
	}
	wg.Wait()

	// 1000 appends num buffer de 100: retenção exata e contabilidade de drops
	// são determinísticas, independentemente da intercalação das goroutines.
	if b.Len() != 100 {
		t.Errorf("Len = %d (esperava 100)", b.Len())
	}
	if b.Dropped() != 900 {
		t.Errorf("Dropped = %d (esperava 900)", b.Dropped())
	}

	// Cada goroutine escreve valores distintos, então as 100 entradas retidas
	// (as últimas 100 chamadas Append, em ordem de intercalação) precisam ser
	// todas distintas e dentro do intervalo. Ordenação por valor NÃO é
	// garantida com writers concorrentes, então não deve ser verificada.
	snap := b.Snapshot()
	seen := make(map[uint64]bool, len(snap))
	for _, e := range snap {
		if e.Num >= 1000 {
			t.Errorf("valor fora do intervalo: %d", e.Num)
		}
		if seen[e.Num] {
			t.Errorf("valor duplicado no snapshot: %d", e.Num)
		}
		seen[e.Num] = true
	}
}
