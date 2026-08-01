package filter

import (
	"testing"

	"github.com/Morpa/kizashi/internal/model"
)

func entry(text string, lvl model.Level, fields map[string]any) model.Entry {
	return model.Entry{Text: text, Level: lvl, IsJSON: fields != nil, Fields: fields}
}

func TestSubstring(t *testing.T) {
	f := Parse("boom")
	if !f.Match(entry("Boom happened", model.LevelNone, nil)) {
		t.Error("substring case-insensitive deveria casar")
	}
	if f.Match(entry("ok", model.LevelNone, nil)) {
		t.Error("não deveria casar")
	}
}

func TestField(t *testing.T) {
	f := Parse("service=api")
	if !f.Match(entry("request", model.LevelInfo, map[string]any{"service": "api", "status": 200})) {
		t.Error("service=api deveria casar")
	}
	if f.Match(entry("request", model.LevelInfo, map[string]any{"service": "worker"})) {
		t.Error("service=worker não deveria casar com service=api")
	}
	if f.Match(entry("api", model.LevelInfo, nil)) {
		t.Error("entrada de texto puro não deveria casar fieldTerm")
	}
}

func TestLevelOps(t *testing.T) {
	if !Parse("level>=warn").Match(entry("", model.LevelError, map[string]any{})) {
		t.Error("error >= warn deveria casar")
	}
	if Parse("level>=warn").Match(entry("", model.LevelInfo, map[string]any{})) {
		t.Error("info >= warn não deveria casar")
	}
	if !Parse("level=error").Match(entry("", model.LevelError, map[string]any{})) {
		t.Error("level=error deveria casar")
	}
}

func TestAnd(t *testing.T) {
	f := Parse("level=error timeout")
	if !f.Match(entry("timeout on db", model.LevelError, map[string]any{})) {
		t.Error("deveria casar ambos os termos")
	}
	if f.Match(entry("slow query", model.LevelError, map[string]any{})) {
		t.Error("faltou o termo 'timeout'")
	}
}

func TestEmptyMatchesAll(t *testing.T) {
	f := Parse("")
	if !f.IsEmpty() {
		t.Error("esperava filtro vazio")
	}
	if !f.Match(entry("qualquer coisa", model.LevelNone, nil)) {
		t.Error("filtro vazio casa tudo")
	}
}

func TestNestedField(t *testing.T) {
	f := Parse("request.id=42")
	e := entry("", model.LevelInfo, map[string]any{"request": map[string]any{"id": 42}})
	if !f.Match(e) {
		t.Error("campo aninhado deveria casar")
	}
}

func TestNeedle(t *testing.T) {
	if got := Parse("timeout").Needle(); got != "timeout" {
		t.Errorf("Needle = %q", got)
	}
	if got := Parse("level=error").Needle(); got != "" {
		t.Errorf("Needle de levelTerm = %q (esperava vazio)", got)
	}
}
