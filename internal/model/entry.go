// Package model define as estruturas de dados compartilhadas por todo o
// pipeline (ingest → buffer → render). É propositalmente puro: nenhuma
// dependência de tview ou cobra aqui.
package model

import (
	"strconv"
	"strings"
)

// Level representa a severidade de uma entrada de log.
// A ordem é significativa: comparações (<, >, >=) usam essa ordem.
type Level uint8

const (
	LevelNone Level = iota
	LevelTrace
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// String devolve o rótulo com largura fixa de 5 caracteres para alinhamento.
func (l Level) String() string {
	switch l {
	case LevelTrace:
		return "TRACE"
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO "
	case LevelWarn:
		return "WARN "
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "-----"
	}
}

// LevelFromValue converte um valor arbitrário (string ou número) em Level.
func LevelFromValue(v any) (Level, bool) {
	switch t := v.(type) {
	case string:
		return LevelFromName(t)
	case float64:
		return LevelFromNumber(t)
	case int:
		return LevelFromNumber(float64(t))
	case int64:
		return LevelFromNumber(float64(t))
	}
	return LevelNone, false
}

// LevelFromName interpreta nomes de nível comuns (case-insensitive) e
// também escalas numéricas passadas como string (ex.: "30").
func LevelFromName(s string) (Level, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace", "t", "verbose", "finest", "all":
		return LevelTrace, true
	case "debug", "dbg", "dbgs", "fine":
		return LevelDebug, true
	case "info", "informational", "notice", "success", "log":
		return LevelInfo, true
	case "warn", "warning", "wrn":
		return LevelWarn, true
	case "error", "err", "erro", "severe":
		return LevelError, true
	case "fatal", "critical", "crit", "panic", "emerg", "emergency", "alert":
		return LevelFatal, true
	}
	// escala numérica como string (ex.: "30")
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return LevelFromNumber(float64(n))
	}
	return LevelNone, false
}

// LevelFromNumber interpreta as duas escalas numéricas comuns:
// syslog (0–7) e bunyan/pino (10–60).
func LevelFromNumber(n float64) (Level, bool) {
	if n >= 0 && n <= 7 {
		switch {
		case n <= 1: // emerg / alert
			return LevelFatal, true
		case n == 2: // crit
			return LevelFatal, true
		case n == 3: // err
			return LevelError, true
		case n == 4: // warning
			return LevelWarn, true
		case n == 5: // notice
			return LevelInfo, true
		case n == 6: // info
			return LevelInfo, true
		default: // 7 = debug
			return LevelDebug, true
		}
	}
	// bunyan/pino: 10=trace, 20=debug, 30=info, 40=warn, 50=error, 60=fatal.
	switch {
	case n <= 10:
		return LevelTrace, true
	case n <= 20:
		return LevelDebug, true
	case n <= 30:
		return LevelInfo, true
	case n <= 40:
		return LevelWarn, true
	case n <= 50:
		return LevelError, true
	default:
		return LevelFatal, true
	}
}

// Entry é a unidade que trafega do ingest até o renderer.
type Entry struct {
	Num     uint64         // contador monotônico de sessão (ordenação estável)
	Raw     string         // linha original, com ANSI se houver (detail view)
	IsJSON  bool           // true se Fields foi parseado com sucesso
	Source  string         // "stdin" ou caminho do arquivo
	Time    string         // timestamp textual; "" se ausente
	Level   Level          // LevelNone para linhas sem nível detectado
	Service string         // "" se ausente
	Fields  map[string]any // campos top-level do JSON; nil se !IsJSON
	Text    string         // mensagem (JSON) ou linha original (texto puro)
}
