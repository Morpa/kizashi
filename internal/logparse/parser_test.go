package logparse

import (
	"testing"

	"github.com/Morpa/kizashi/internal/model"
)

func TestLogfmtBasic(t *testing.T) {
	m, ok := parseLogfmt("level=info msg=boom service=api")
	if !ok {
		t.Fatal("esperava logfmt válido")
	}
	if m["level"] != "info" || m["msg"] != "boom" || m["service"] != "api" {
		t.Errorf("m = %v", m)
	}
}

func TestLogfmtQuotedValue(t *testing.T) {
	m, ok := parseLogfmt(`msg="hello world"`)
	if !ok {
		t.Fatal("esperava logfmt válido")
	}
	if m["msg"] != "hello world" {
		t.Errorf("msg = %v, want 'hello world'", m["msg"])
	}
}

func TestLogfmtEscapedQuote(t *testing.T) {
	m, ok := parseLogfmt(`msg="a\"b"`)
	if !ok {
		t.Fatal("esperava logfmt válido")
	}
	if m["msg"] != `a"b` {
		t.Errorf("msg = %v, want a\"b", m["msg"])
	}
}

func TestLogfmtBareNumeric(t *testing.T) {
	m, ok := parseLogfmt("ts=1780000000 status=200")
	if !ok {
		t.Fatal("esperava logfmt válido")
	}
	if m["ts"].(float64) != 1780000000 || m["status"].(float64) != 200 {
		t.Errorf("m = %v", m)
	}
}

func TestLogfmtNoEquals(t *testing.T) {
	if _, ok := parseLogfmt("GET /path 200 in 105ms"); ok {
		t.Error("linha HTTP não deveria virar logfmt")
	}
}

func TestLogfmtNonKeyLead(t *testing.T) {
	if _, ok := parseLogfmt("ui text here"); ok {
		t.Error("linha sem key= inicial não deveria virar logfmt")
	}
}

func TestLogfmtUnterminatedQuote(t *testing.T) {
	if _, ok := parseLogfmt(`msg="abc`); ok {
		t.Error("aspas sem fechar deveriam rejeitar logfmt")
	}
}

func TestLogfmtEmpty(t *testing.T) {
	if _, ok := parseLogfmt(""); ok {
		t.Error("linha vazia não é logfmt")
	}
}

func TestParseLineLogfmt(t *testing.T) {
	e := ParseLine("level=warn msg=slow", "s")
	if !e.IsJSON {
		t.Fatal("logfmt deveria marcar IsJSON (estruturado)")
	}
	if e.Level != model.LevelWarn {
		t.Errorf("Level = %v, want Warn", e.Level)
	}
	if e.Text != "slow" {
		t.Errorf("Text = %q, want slow", e.Text)
	}
	if e.Service != "" {
		t.Errorf("Service = %q, want vazio", e.Service)
	}
}

func TestParseLineLogfmtNumericLevel(t *testing.T) {
	e := ParseLine("level=50 msg=boom", "s")
	if e.Level != model.LevelError {
		t.Errorf("Level = %v, want Error (50 pino)", e.Level)
	}
}

func TestParseLineJSONBeforeLogfmt(t *testing.T) {
	// linha JSON com '=' dentro: o jsonParser vence o logfmt.
	e := ParseLine(`{"level":"info","msg":"a=b"}`, "s")
	if !e.IsJSON {
		t.Fatal("esperava JSON estruturado")
	}
	if e.Level != model.LevelInfo {
		t.Errorf("Level = %v, want Info", e.Level)
	}
	if e.Text != "a=b" {
		t.Errorf("Text = %q, want a=b", e.Text)
	}
}

func TestParseLineLogfmtFloatTime(t *testing.T) {
	e := ParseLine("level=info ts=1780000000 msg=ok", "s")
	if e.Time == "" {
		t.Error("Time vazio; esperava epoch formatado")
	}
}