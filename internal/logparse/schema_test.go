package logparse

import (
	"testing"

	"github.com/Morpa/kizashi/internal/model"
)

func TestParseSchemaHappy(t *testing.T) {
	s, err := ParseSchema("level=severity,time=ts,msg=message,service=app")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if s.Level != "severity" || s.Time != "ts" || s.Message != "message" || s.Service != "app" {
		t.Errorf("schema = %+v", s)
	}
}

func TestParseSchemaMsgAlias(t *testing.T) {
	s, err := ParseSchema("msg=body")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if s.Message != "body" {
		t.Errorf("Message = %q, want body", s.Message)
	}
}

func TestParseSchemaDottedPath(t *testing.T) {
	s, err := ParseSchema("level=a.b.c")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if s.Level != "a.b.c" {
		t.Errorf("Level = %q, want a.b.c", s.Level)
	}
}

func TestParseSchemaEmpty(t *testing.T) {
	s, err := ParseSchema("")
	if err != nil || s != nil {
		t.Errorf("spec vazio: s=%v err=%v, esperava (nil, nil)", s, err)
	}
}

func TestParseSchemaUnknownRole(t *testing.T) {
	if _, err := ParseSchema("bogus=1"); err == nil {
		t.Error("esperava erro para role desconhecido")
	}
}

func TestParseSchemaDuplicateRole(t *testing.T) {
	if _, err := ParseSchema("level=severity,level=ts"); err == nil {
		t.Error("esperava erro para role duplicado")
	}
}

func TestParseSchemaEmptyValue(t *testing.T) {
	if _, err := ParseSchema("level="); err == nil {
		t.Error("esperava erro para valor vazio")
	}
	if _, err := ParseSchema("level=.x"); err == nil {
		t.Error("esperava erro para componente vazio")
	}
}

func TestParseSchemaEmptySegment(t *testing.T) {
	if _, err := ParseSchema("level=a,,"); err == nil {
		t.Error("esperava erro para segmento vazio")
	}
}

func TestParseSchemaMissingEquals(t *testing.T) {
	if _, err := ParseSchema("level"); err == nil {
		t.Error("esperava erro para segmento sem '='")
	}
}

func TestParseSchemaInvalidPathChar(t *testing.T) {
	if _, err := ParseSchema("level=a-b"); err == nil {
		t.Error("esperava erro para caminho com caractere inválido")
	}
}

func TestSchemaOverrideEndToEnd(t *testing.T) {
	s, err := ParseSchema("level=severity,time=ts,msg=message,service=component")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	e := ParseLineWith("severity=error ts=1780000000 message=boom component=api", "s", s)
	if !e.IsJSON {
		t.Fatal("esperava logfmt estruturado")
	}
	if e.Level != model.LevelError {
		t.Errorf("Level = %v, want Error", e.Level)
	}
	if e.Time == "" {
		t.Error("Time vazio")
	}
	if e.Text != "boom" {
		t.Errorf("Text = %q, want boom", e.Text)
	}
	if e.Service != "api" {
		t.Errorf("Service = %q, want api", e.Service)
	}
}

func TestSchemaOverridePresentButAbsentNoFallback(t *testing.T) {
	// level=severity é uma afirmação: sem "severity" no payload, LevelNone,
	// sem cair na tabela padrão. Mensagem (não override) continua resolvendo.
	e := ParseLineWith(`{"level":"info","msg":"hi"}`, "s", &Schema{Level: "severity"})
	if e.Level != model.LevelNone {
		t.Errorf("Level = %v, want LevelNone (override ausente não tem fallback)", e.Level)
	}
	if e.Text != "hi" {
		t.Errorf("Text = %q, want hi (msg não foi override)", e.Text)
	}
}
