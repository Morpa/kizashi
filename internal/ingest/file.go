package ingest

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// tailPollInterval é o intervalo entre checagens do tail de arquivo,
// acessado atomicamente: a goroutine de tail o lê a cada ciclo enquanto
// testes podem reajustá-lo (SetTailPollInterval) sem data race.
var tailPollInterval atomic.Int64

func init() {
	tailPollInterval.Store(int64(250 * time.Millisecond))
}

// SetTailPollInterval ajusta o intervalo de poll do tail. Usado por testes
// para encurtá-lo e, idealmente, chamado antes de iniciar a fonte.
func SetTailPollInterval(d time.Duration) {
	tailPollInterval.Store(int64(d))
}

func pollInterval() time.Duration {
	return time.Duration(tailPollInterval.Load())
}

// FileSource cria uma fonte que faz tail do arquivo em path, com detecção
// de rotação (arquivo substituído) e truncamento.
func FileSource(ctx context.Context, path string) (Source, error) {
	f, err := os.Open(path)
	if err != nil {
		return Source{}, err
	}
	lines := make(chan string, 1024)
	go tailFile(ctx, f, path, lines)
	return Source{Name: path, Lines: lines}, nil
}

// tailFile lê o arquivo a partir do fim (semântica de tail -f). Trata
// linhas parciais (aguarda o '\n'), rotação para um novo inode e
// truncamento/shrink, sem travar se o arquivo sumir momentaneamente.
func tailFile(ctx context.Context, f *os.File, path string, lines chan<- string) {
	defer close(lines)
	defer func() { _ = f.Close() }()

	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return
	}
	reader := bufio.NewReader(f)
	var pending []byte // linha parcial aguardando completar

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		replaced, shrunk := fileChanged(f, path)
		if replaced {
			_ = f.Close()
			nf, err := os.Open(path)
			if err != nil {
				pending = pending[:0]
				if !sleepOrDone(ctx) {
					return
				}
				continue
			}
			f = nf
			reader.Reset(f)
			pending = pending[:0]
		} else if shrunk {
			_, _ = f.Seek(0, io.SeekStart)
			reader.Reset(f)
			pending = pending[:0]
		}

		line, err := reader.ReadString('\n')
		switch err {
		case nil:
			full := string(pending) + line
			pending = pending[:0]
			full = strings.TrimRight(full, "\r\n")
			if !sendOrDone(ctx, lines, full) {
				return
			}
		case io.EOF:
			if len(line) > 0 {
				pending = append(pending, line...)
			}
			if !sleepOrDone(ctx) {
				return
			}
		default:
			return // erro real de leitura
		}
	}
}

// fileChanged detecta, via os.Stat, se o arquivo foi substituído (rotação,
// inode diferente) ou truncado (tamanho menor que a posição atual de leitura).
func fileChanged(f *os.File, path string) (replaced, shrunk bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, false // arquivo pode estar ausente momentaneamente
	}
	openFi, err := f.Stat()
	if err != nil {
		return false, false
	}
	if !os.SameFile(fi, openFi) {
		return true, false
	}
	pos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return false, false
	}
	if fi.Size() < pos {
		return false, true
	}
	return false, false
}

// sleepOrDone dorme por pollInterval e devolve false se o contexto for
// cancelado (o chamador deve retornar).
func sleepOrDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(pollInterval()):
		return true
	}
}
