// Package buffer implementa um ring buffer thread-safe com teto de memória
// fixo: quando cheio, a entrada mais antiga é descartada (drop-oldest).
package buffer

import (
	"sync"

	"github.com/Morpa/kizashi/internal/model"
)

// Buffer é um ring buffer thread-safe com teto de memória fixo.
type Buffer struct {
	mu      sync.RWMutex
	ring    []model.Entry
	start   int    // índice da entrada mais antiga
	n       int    // quantidade ocupada
	dropped uint64 // entradas descartadas por estouro
	version uint64 // incrementado a cada Append
}

// New cria um buffer com a capacidade dada (mínimo 1).
func New(capacity int) *Buffer {
	if capacity < 1 {
		capacity = 1
	}
	return &Buffer{ring: make([]model.Entry, capacity)}
}

// Append insere uma entrada. Se o buffer estiver cheio, descarta a mais antiga.
func (b *Buffer) Append(e model.Entry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	idx := (b.start + b.n) % cap(b.ring)
	b.ring[idx] = e
	if b.n < cap(b.ring) {
		b.n++
	} else {
		b.start = (b.start + 1) % cap(b.ring)
		b.dropped++
	}
	b.version++
}

// Len devolve a quantidade atual de entradas.
func (b *Buffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.n
}

// Version é incrementado a cada Append; serve para invalidar caches no render.
func (b *Buffer) Version() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.version
}

// Dropped devolve quantas entradas foram descartadas por estouro.
func (b *Buffer) Dropped() uint64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.dropped
}

// Snapshot devolve uma cópia em ordem cronológica das entradas atuais.
// O lock é liberado antes do retorno; o render nunca segura o lock durante a pintura.
func (b *Buffer) Snapshot() []model.Entry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make([]model.Entry, b.n)
	for i := 0; i < b.n; i++ {
		out[i] = b.ring[(b.start+i)%cap(b.ring)]
	}
	return out
}
