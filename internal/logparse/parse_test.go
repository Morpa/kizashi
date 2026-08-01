package logparse

import (
	"testing"

	"github.com/Morpa/kizashi/internal/model"
)

func TestParseLineJSON(t *testing.T) {
	e := ParseLine(`{"level":"error","msg":"boom","service":"api","time":1780000000}`, "stdin")
	if !e.IsJSON {
		t.Fatal("esperava IsJSON true")
	}
	if e.Level != model.LevelError {
		t.Errorf("Level = %v", e.Level)
	}
	if e.Text != "boom" {
		t.Errorf("Text = %q", e.Text)
	}
	if e.Service != "api" {
		t.Errorf("Service = %q", e.Service)
	}
	if e.Time == "" {
		t.Error("Time vazio")
	}
	if e.Source != "stdin" {
		t.Errorf("Source = %q", e.Source)
	}
}

func TestParseLinePlain(t *testing.T) {
	e := ParseLine("this is just text", "stdin")
	if e.IsJSON {
		t.Fatal("esperava IsJSON false")
	}
	if e.Text != "this is just text" {
		t.Errorf("Text = %q", e.Text)
	}
}

func TestParseLineANSIJSON(t *testing.T) {
	e := ParseLine("\x1b[32m{\"level\":\"info\",\"msg\":\"hi\"}\x1b[0m", "stdin")
	if !e.IsJSON {
		t.Fatalf("esperava JSON mesmo com ANSI; text=%q", e.Text)
	}
	if e.Text != "hi" {
		t.Errorf("Text = %q", e.Text)
	}
}

func TestParseLineArrayNotJSON(t *testing.T) {
	e := ParseLine(`[1,2,3]`, "stdin")
	if e.IsJSON {
		t.Fatal("array não deveria virar JSON objeto")
	}
	if e.Text != `[1,2,3]` {
		t.Errorf("Text = %q", e.Text)
	}
}

func TestHeuristicLevelPlain(t *testing.T) {
	if got := ParseLine("ERROR: failed to connect", "s").Level; got != model.LevelError {
		t.Errorf("ERROR → %v", got)
	}
	if got := ParseLine("ocorreu um erro na conexão", "s").Level; got != model.LevelError {
		t.Errorf("'erro' (pt) → %v", got)
	}
	if got := ParseLine("normal log line", "s").Level; got != model.LevelNone {
		t.Errorf("normal → %v", got)
	}
}

func TestExtractMessageFallback(t *testing.T) {
	// sem chave de mensagem: usa campo string parecido com mensagem
	e := ParseLine(`{"level":"warn","duration":1500,"reason":"query muito lenta"}`, "s")
	if e.Text != "query muito lenta" {
		t.Errorf("Text = %q", e.Text)
	}
	// sem nada: JSON compacto
	e = ParseLine(`{"level":"info","status":200}`, "s")
	if e.Text == "" {
		t.Error("Text vazio")
	}
	// error como fallback de mensagem
	e = ParseLine(`{"level":"error","error":"boom"}`, "s")
	if e.Text != "boom" {
		t.Errorf("Text = %q", e.Text)
	}
}

func TestNestedLevel(t *testing.T) {
	e := ParseLine(`{"log":{"level":"warn"},"message":"x"}`, "s")
	if e.Level != model.LevelWarn {
		t.Errorf("Level = %v", e.Level)
	}
}

func TestNumericLevel(t *testing.T) {
	e := ParseLine(`{"level":30,"msg":"pino info"}`, "s")
	if e.Level != model.LevelInfo {
		t.Errorf("Level = %v", e.Level)
	}
}

func TestParseLineKeepsRaw(t *testing.T) {
	raw := "\x1b[31merror boom\x1b[0m"
	e := ParseLine(raw, "s")
	if e.Raw != raw {
		t.Errorf("Raw foi alterado")
	}
}
