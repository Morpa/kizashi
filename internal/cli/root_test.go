package cli

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "kizashi") {
		t.Errorf("version deveria mencionar 'kizashi', out = %q", out.String())
	}
}

func TestHelpListsCommands(t *testing.T) {
	var out bytes.Buffer
	cmd := NewRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "stream") {
		t.Errorf("help deveria listar o comando 'stream', out = %q", out.String())
	}
}

// TestStreamMissingFile é determinístico independente do stdin do teste:
// FileSource falha ao abrir um arquivo inexistente antes de qualquer leitura.
func TestStreamMissingFile(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"stream", "-f", filepath.Join(t.TempDir(), "nope.log")})
	if err := cmd.Execute(); err == nil {
		t.Error("esperava erro para arquivo inexistente")
	}
}
