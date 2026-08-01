// Package logparse converte linhas brutas de log em model.Entry.
// Lógica pura, sem dependências de UI.

package logparse

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/Morpa/kizashi/internal/model"
)

var ansiStrip = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// ParseLine converte uma linha de log em uma model.Entry.
func ParseLine(line, source string) model.Entry {
	e := model.Entry{Raw: line, Source: source}

	if fields, ok := parseJSONObject(line); ok {
		e.IsJSON = true
		e.Fields = fields
		e.Level = extractLevel(fields)
		e.Time = extractTime(fields)
		e.Service = extractService(fields)
		e.Text = extractMessage(fields)
		return e
	}

	// texto puro: mantém a linha original (com ANSI, se houver)
	e.Text = line
	e.Level = heuristicLevel(line)
	return e
}

// parseJSONObject tenta interpretar a linha como um objeto JSON.
// Aceita apenas objetos (mapas); arrays e escalares caem para texto puro.
// Tenta a linha crua primeiro e, se falhar, tenta sem códigos ANSI
// (alguns frameworks colorem a linha inteira, inclusive o JSON).
func parseJSONObject(line string) (map[string]any, bool) {
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err == nil && m != nil {
		return m, true
	}
	clean := ansiStrip.ReplaceAllString(line, "")
	if clean != line {
		m = nil
		if err := json.Unmarshal([]byte(clean), &m); err == nil && m != nil {
			return m, true
		}
	}
	return nil, false
}

// heuristicLevel detecta o nível por palavras em linhas de texto puro.
func heuristicLevel(s string) model.Level {
	ls := strings.ToLower(s)
	switch {
	case strings.Contains(ls, "panic"), strings.Contains(ls, "fatal"):
		return model.LevelFatal
	case strings.Contains(ls, "erro"): // "error" e "erro" (pt)
		return model.LevelError
	case strings.Contains(ls, "warn"):
		return model.LevelWarn
	case strings.Contains(ls, "debug"):
		return model.LevelDebug
	case strings.Contains(ls, "trace"):
		return model.LevelTrace
	case strings.Contains(ls, "info"):
		return model.LevelInfo
	}
	return model.LevelNone
}
