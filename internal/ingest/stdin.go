package ingest

import (
	"bufio"
	"context"
	"os"
	"strings"
)

// StdinIsPipe indica se o stdin é um pipe (não um TTY).
func StdinIsPipe() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice == 0
}

// StdinSource cria uma fonte que lê o stdin linha a linha. Usa
// bufio.Reader (sem o limite de 64 KB do Scanner), com buffer de 1 MB.
func StdinSource(ctx context.Context) Source {
	lines := make(chan string, 1024)
	go func() {
		defer close(lines)
		r := bufio.NewReaderSize(os.Stdin, 1<<20)
		for {
			line, err := r.ReadBytes('\n')
			if len(line) > 0 {
				s := strings.TrimRight(string(line), "\r\n")
				if !sendOrDone(ctx, lines, s) {
					return
				}
			}
			if err != nil {
				return // EOF ou erro de leitura
			}
		}
	}()
	return Source{Name: "stdin", Lines: lines}
}

func sendOrDone(ctx context.Context, ch chan<- string, s string) bool {
	select {
	case <-ctx.Done():
		return false
	case ch <- s:
		return true
	}
}
