// Package cli define a interface de linha de comando (cobra).
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version é substituível via ldflags (go build -ldflags "-X kizashi/internal/cli.version=1.0.0").
var version = "dev"

// NewRootCmd monta o comando raiz com os subcomandos.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "kizashi",
		Short: "Streama e visualiza logs locais estruturados no terminal",
		Long: `kizashi é um TUI para ver logs estruturados (JSON) localmente, com
follow ao vivo, filtros e cores por nível.

Exemplos:
  meu-servidor 2>&1 | kizashi stream
  kizashi stream -f app.log -f api.log`,
		SilenceUsage: true,
	}
	root.AddCommand(newStreamCmd())
	root.AddCommand(newVersionCmd())
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Exibe a versão",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "kizashi %s\n", version)
			return nil
		},
	}
}

// Execute executa a CLI e sai com código de erro se algo falhar.
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
