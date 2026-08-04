// Package logparse converte linhas brutas de log em model.Entry.
// Lógica pura, sem dependências de UI.
//
// Arquitetura em 3 camadas:
//   - parsers: formatos estruturados (JSON, logfmt) atrás da interface Parser;
//   - record: payload normalizado de onde os papéis semânticos são extraídos;
//   - resolver: localiza level/time/mensagem/serviço por precedência de sinal,
//     com override opcional via Schema (flag --format).

package logparse

import (
	"regexp"
	"strings"

	"github.com/Morpa/kizashi/internal/model"
)

var ansiStrip = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// taskPrefix casa prefixos de task-runners de monorepo (turbo/pnpm/
// concurrently), ex.: "be/general-rest-api dev: {...}" ou "ui dev: GET ...".
// Exige início em minúscula para não colidir com "Error:"/"Warning:"/
// "Note:" (convenção de frase capitalizada usada por heuristicLevel).
var taskPrefix = regexp.MustCompile(`^([a-z][a-z0-9._/@-]*)(?:\s+[a-z][a-z0-9_-]*)?:\s(.*)$`)

// ParseLine converte uma linha de log em uma model.Entry usando o schema padrão.
func ParseLine(line, source string) model.Entry {
	return ParseLineWith(line, source, nil)
}

// ParseLineWith converte uma linha usando o schema dado (nil = detecção
// automática). O prefixo de task-runner é extraído antes da seleção de parser
// e vira Service quando o payload não tiver um.
func ParseLineWith(line, source string, schema *Schema) model.Entry {
	e := model.Entry{Raw: line, Source: source}

	rest := line
	prefixService := ""
	if m := taskPrefix.FindStringSubmatch(line); m != nil {
		prefixService = m[1]
		rest = m[2]
	}

	for _, p := range parsers {
		if rec, ok := p.Parse(rest); ok {
			e.IsJSON = rec.Structured
			e.Fields = rec.Fields
			e.Level, e.Time, e.Service, e.Text = applySchema(rec, schema)
			if e.Service == "" {
				e.Service = prefixService
			}
			return e
		}
	}

	// texto puro: mantém o resto da linha (com ANSI, se houver)
	e.Service = prefixService
	e.Text = rest
	e.Level = heuristicLevel(rest)
	return e
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