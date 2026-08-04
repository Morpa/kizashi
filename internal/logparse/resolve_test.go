package logparse

import "testing"

func TestResolveExactShallowBeatsExactNested(t *testing.T) {
	m := map[string]any{"level": "debug", "log": map[string]any{"level": "info"}}
	if got, _ := resolve(m, fieldLevel, &Schema{}); got != "debug" {
		t.Errorf("level = %v, want debug (exato no topo vence aninhado)", got)
	}
}

func TestResolveExactNestedBare(t *testing.T) {
	m := map[string]any{"a": map[string]any{"b": map[string]any{"level": "warn"}}}
	if got, _ := resolve(m, fieldLevel, &Schema{}); got != "warn" {
		t.Errorf("level = %v, want warn (aninhado em profundidade)", got)
	}
}

func TestResolveExactNestedDotted(t *testing.T) {
	m := map[string]any{"log": map[string]any{"level": "warn"}, "message": "x"}
	if got, _ := resolve(m, fieldLevel, &Schema{}); got != "warn" {
		t.Errorf("level = %v, want warn (caminho log.level)", got)
	}
}

func TestResolveFlatDottedKey(t *testing.T) {
	m := map[string]any{"logging.level": "error"}
	if got, _ := resolve(m, fieldLevel, &Schema{}); got != "error" {
		t.Errorf("level = %v, want error (chave chata pontuada no topo)", got)
	}
}

func TestResolveExactNestedBeatsFoldShallow(t *testing.T) {
	m := map[string]any{"LEVEL": "info", "log": map[string]any{"level": "warn"}}
	if got, _ := resolve(m, fieldLevel, &Schema{}); got != "warn" {
		t.Errorf("level = %v, want warn (aninhado exato vence case-fold no topo)", got)
	}
}

func TestResolveFoldShallow(t *testing.T) {
	m := map[string]any{"Level": "info"}
	if got, _ := resolve(m, fieldLevel, &Schema{}); got != "info" {
		t.Errorf("level = %v, want info (case-fold no topo)", got)
	}
}

func TestResolveFoldNested(t *testing.T) {
	m := map[string]any{"log": map[string]any{"Level": "warn"}}
	if got, _ := resolve(m, fieldLevel, &Schema{}); got != "warn" {
		t.Errorf("level = %v, want warn (case-fold aninhado)", got)
	}
}

func TestResolveNumericNested(t *testing.T) {
	m := map[string]any{"a": map[string]any{"level": float64(50)}}
	got, ok := resolve(m, fieldLevel, &Schema{})
	if !ok {
		t.Fatal("esperava resolver nível numérico aninhado")
	}
	if got.(float64) != 50 {
		t.Errorf("level = %v, want 50", got)
	}
}

func TestResolveMessageMsgWinsOverMessage(t *testing.T) {
	m := map[string]any{"msg": "hi", "message": "bye"}
	if got, _ := resolve(m, fieldMessage, &Schema{}); got != "hi" {
		t.Errorf("message = %v, want hi (msg vem antes na tabela)", got)
	}
}

func TestResolveService(t *testing.T) {
	m := map[string]any{"component": "payments"}
	if got, _ := resolve(m, fieldService, &Schema{}); got != "payments" {
		t.Errorf("service = %v, want payments", got)
	}
}

func TestResolveSchemaOverride(t *testing.T) {
	m := map[string]any{"severity": "warn"}
	if got, _ := resolve(m, fieldLevel, &Schema{Level: "severity"}); got != "warn" {
		t.Errorf("level = %v, want warn (override level=severity)", got)
	}
}

func TestResolveSchemaOverrideNoFallback(t *testing.T) {
	// override é uma afirmação: ausente → não resolve, sem cair na tabela padrão.
	m := map[string]any{"level": "warn"}
	if _, ok := resolve(m, fieldLevel, &Schema{Level: "severity"}); ok {
		t.Error("esperava não resolver com override ausente no payload")
	}
}

func TestResolveDepthCap(t *testing.T) {
	// payload 20 níveis abaixo de "level": a busca deve retornar sem estourar.
	m := map[string]any{}
	cur := m
	for range 20 {
		nxt := map[string]any{}
		cur["a"] = nxt
		cur = nxt
	}
	cur["level"] = "info"
	if _, ok := resolve(m, fieldLevel, &Schema{}); ok {
		t.Error("esperava não resolver além do teto de profundidade")
	}
}

func TestLookupPathNotFound(t *testing.T) {
	if _, ok := lookupPath(map[string]any{"x": "y"}, []string{"level"}, false); ok {
		t.Error("esperava não encontrar")
	}
}
