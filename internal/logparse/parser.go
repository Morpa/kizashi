package logparse

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Parser converte uma linha em um Record estruturado. Implementar a interface
// é como se registra um novo formato — sem tocar nas tabelas de resolução.
type Parser interface {
	Parse(line string) (Record, bool)
}

// parsers é o registro de formatos, na ordem de tentativa.
var parsers []Parser

// Register adiciona um parser ao registro (ordem = precedência de tentativa).
func Register(p Parser) { parsers = append(parsers, p) }

func init() {
	Register(jsonParser{})
	Register(logfmtParser{})
}

// jsonParser reconhece um objeto JSON ({...}), com ou sem código ANSI.
type jsonParser struct{}

func (jsonParser) Parse(line string) (Record, bool) {
	m, ok := parseJSONObject(line)
	if !ok {
		return Record{}, false
	}
	return Record{Fields: m, Structured: true}, true
}

// parseJSONObject tenta interpretar a linha como um objeto JSON.
// Aceita apenas objetos (mapas); arrays e escalares caem para o próximo parser.
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

// logfmtParser reconhece linhas k=v (logfmt/GELF-like), respeitando valores
// com aspas. Só casa se a linha começar com um token key= (guard): isso evita
// engolir texto puro, linhas HTTP ou prosa como logfmt.
type logfmtParser struct{}

func (logfmtParser) Parse(line string) (Record, bool) {
	m, ok := parseLogfmt(line)
	if !ok {
		return Record{}, false
	}
	return Record{Fields: m, Structured: true}, true
}

// parseLogfmt tokeniza "key=value key2=\"a b\" ...". Devolve false se a linha
// não for um logfmt válido (guard: sem token key= inicial; valor com aspas sem
// fechar; token sem '='; texto entre tokens).
func parseLogfmt(line string) (map[string]any, bool) {
	m := map[string]any{}
	i := 0
	for {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i >= len(line) {
			break
		}

		// chave: [A-Za-z0-9_.-]+
		start := i
		for i < len(line) && isKeyRune(line[i]) {
			i++
		}
		if i == start || i >= len(line) || line[i] != '=' {
			return nil, false
		}
		key := line[start:i]
		i++ // '='

		var val string
		if i < len(line) && line[i] == '"' {
			i++ // abre aspas
			var b strings.Builder
			closed := false
			for i < len(line) {
				c := line[i]
				if c == '\\' && i+1 < len(line) {
					b.WriteByte(line[i+1])
					i += 2
					continue
				}
				if c == '"' {
					closed = true
					i++
					break
				}
				b.WriteByte(c)
				i++
			}
			if !closed {
				return nil, false
			}
			val = b.String()
		} else {
			s := i
			for i < len(line) && line[i] != ' ' && line[i] != '\t' {
				i++
			}
			val = line[s:i]
		}
		m[key] = normalizeLogfmtValue(val)
	}
	if len(m) == 0 {
		return nil, false
	}
	return m, true
}

// isKeyRune reporta se o byte pode compor uma chave logfmt.
func isKeyRune(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	case b == '_', b == '.', b == '-':
		return true
	}
	return false
}

// normalizeLogfmtValue converte valores bare que parecem número para float64
// (para o nível numérico e o filtro tratarem igual ao JSON); o resto vira string.
func normalizeLogfmtValue(v string) any {
	if v == "" {
		return ""
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return v
}