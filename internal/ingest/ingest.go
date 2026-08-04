// Package ingest conecta as fontes de log (stdin, arquivos) ao buffer,
// com backpressure natural e parada limpa via contexto.
package ingest

import (
	"context"
	"sync"

	"github.com/Morpa/kizashi/internal/buffer"
	"github.com/Morpa/kizashi/internal/logparse"
	"github.com/Morpa/kizashi/internal/model"
)

// Source representa uma origem de linhas de log brutas.
// O canal é fechado quando a fonte termina (EOF) ou o contexto é cancelado.
type Source struct {
	Name  string
	Lines <-chan string
}

// Options configura a ingestão.
type Options struct {
	// Schema mapeia os campos dos logs (flag --format); nil = detecção
	// automática.
	Schema *logparse.Schema
}

// Run inicia uma goroutine por fonte (parse + envio), multiplexa tudo em um
// único consumidor que grava no buffer e notifica o callback. Retorna quando
// todas as fontes terminarem e o buffer for drenado. Usa as opções padrão.
//
// Backpressure: o canal de entries tem capacidade limitada; se o consumidor
// não acompanhar, os produtores bloqueiam (o que freia o pipe upstream).
func Run(ctx context.Context, buf *buffer.Buffer, notify func(), sources ...Source) error {
	return RunWith(ctx, buf, notify, Options{}, sources...)
}

// RunWith é o Run com opções explícitas (schema de campos, por exemplo).
func RunWith(ctx context.Context, buf *buffer.Buffer, notify func(), opts Options, sources ...Source) error {
	entries := make(chan model.Entry, 4096)

	var wg sync.WaitGroup
	wg.Add(len(sources))
	for _, src := range sources {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case line, ok := <-src.Lines:
					if !ok {
						return
					}
					e := logparse.ParseLineWith(line, src.Name, opts.Schema)
					select {
					case <-ctx.Done():
						return
					case entries <- e:
					}
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(entries)
	}()

	var num uint64
	for e := range entries {
		num++
		e.Num = num
		buf.Append(e)
		if notify != nil {
			notify()
		}
	}
	return nil
}
