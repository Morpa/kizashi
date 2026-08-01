package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Morpa/kizashi/internal/buffer"
	"github.com/Morpa/kizashi/internal/ingest"
	"github.com/Morpa/kizashi/internal/tui"
	"github.com/spf13/cobra"
)

func newStreamCmd() *cobra.Command {
	var files []string
	var bufferSize int

	cmd := &cobra.Command{
		Use:   "stream",
		Short: "Streama e inspeciona logs de stdin e/ou arquivos",
		Long: `Lê logs do stdin (pipe) e/ou faz tail de arquivos (-f), renderizando-os
em um TUI com follow, filtros e cores por nível.

Exemplos:
  meu-servidor 2>&1 | kizashi stream
  kizashi stream -f app.log
  kizashi stream -f api.log -f worker.log --buffer 20000`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStream(cmd.Context(), files, bufferSize)
		},
	}
	cmd.Flags().StringArrayVarP(&files, "file", "f", nil, "faz tail de um arquivo (repetível: -f a.log -f b.log)")
	cmd.Flags().IntVar(&bufferSize, "buffer", 10000, "máximo de entradas mantidas no buffer")
	return cmd
}

func runStream(ctx context.Context, files []string, bufferSize int) error {
	if bufferSize < 1 {
		bufferSize = 1
	}

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var sources []ingest.Source
	var names []string

	if ingest.StdinIsPipe() {
		sources = append(sources, ingest.StdinSource(ctx))
		names = append(names, "stdin")
	}
	for _, path := range files {
		src, err := ingest.FileSource(ctx, path)
		if err != nil {
			return fmt.Errorf("abrindo %s: %w", path, err)
		}
		sources = append(sources, src)
		names = append(names, path)
	}
	if len(sources) == 0 {
		fmt.Fprintln(os.Stderr, "kizashi stream espera logs no stdin (pipe) ou via -f ARQUIVO")
		fmt.Fprintln(os.Stderr, "Exemplo: meu-servidor 2>&1 | kizashi stream")
		return fmt.Errorf("nenhuma fonte de logs (use stdin pipe ou -f)")
	}

	buf := buffer.New(bufferSize)
	doneCh := make(chan struct{})

	ui := tui.New(tui.Options{Buffer: buf, Sources: names, DoneCh: doneCh})
	go func() {
		_ = ingest.Run(ctx, buf, ui.Notify, sources...)
		close(doneCh)
	}()

	return ui.Run()
}
