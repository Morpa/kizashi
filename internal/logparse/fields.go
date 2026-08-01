package logparse

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/Morpa/kizashi/internal/model"
)

// Tabelas de aliases comuns de campos estruturados. A primeira chave
// encontrada (case-insensitive) vence.
var (
	levelKeys   = []string{"level", "severity", "lvl", "levelname"}
	timeKeys    = []string{"time", "timestamp", "@timestamp", "ts", "datetime", "date", "logtime", "logged_at", "time_ms"}
	messageKeys = []string{"msg", "message", "msg_txt", "msgtxt", "body", "@message", "messagetext", "msgtext", "log", "event"}
	serviceKeys = []string{"service", "logger", "logger_name", "loggername", "name", "app", "component", "module", "category"}
)

// metadataKeys agrupa as chaves conhecidas para que nenhuma delas seja
// usada como mensagem no fallback.
var metadataKeys = func() map[string]bool {
	m := map[string]bool{}
	all := append(append(append(levelKeys, timeKeys...), messageKeys...), serviceKeys...)
	for _, k := range all {
		m[strings.ToLower(k)] = true
	}
	for _, k := range []string{"error", "err", "status", "code", "stack", "ctx", "context"} {
		m[k] = true
	}
	return m
}()

// pick devolve o valor da primeira chave presente no mapa, com busca
// case-insensitive.
func pick(m map[string]any, keys ...string) (any, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v, true
		}
	}
	for k, v := range m {
		if slices.Contains(keys, strings.ToLower(k)) {
			return v, true
		}
	}
	return nil, false
}

func extractLevel(m map[string]any) model.Level {
	if v, ok := pick(m, levelKeys...); ok {
		if l, ok := model.LevelFromValue(v); ok {
			return l
		}
	}
	// nível aninhado: {"log":{"level":"warn"}} / log_level / logging.level
	if lm, ok := m["log"].(map[string]any); ok {
		if v, ok := pick(lm, "level", "Level"); ok {
			if l, ok := model.LevelFromValue(v); ok {
				return l
			}
		}
	}
	if v, ok := pick(m, "log_level", "logging.level"); ok {
		if l, ok := model.LevelFromValue(v); ok {
			return l
		}
	}
	return model.LevelNone
}

func extractTime(m map[string]any) string {
	v, ok := pick(m, timeKeys...)
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return formatEpoch(t)
	}
	return ""
}

// formatEpoch formata epoch em segundos ou milissegundos (hora local).
func formatEpoch(n float64) string {
	if n > 1e11 {
		n /= 1e3 // milissegundos
	}
	return time.Unix(int64(n), 0).Local().Format("2006-01-02 15:04:05.000")
}

func extractService(m map[string]any) string {
	v, ok := pick(m, serviceKeys...)
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// extractMessage escolhe a melhor representação textual do log:
// 1. aliases de mensagem, 2. error/err, 3. primeiro campo string
// "parecido com mensagem", 4. JSON compacto da linha inteira.
func extractMessage(m map[string]any) string {
	if v, ok := pick(m, messageKeys...); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if v, ok := pick(m, "error", "err"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if s, ok := firstMessageLike(m); ok {
		return s
	}
	if b, err := json.Marshal(m); err == nil {
		return string(b)
	}
	return ""
}

// firstMessageLike procura o primeiro valor string que não seja chave de
// metadado e que pareça uma mensagem (contém espaço ou é razoavelmente longo).
func firstMessageLike(m map[string]any) (string, bool) {
	for k, v := range m {
		if metadataKeys[strings.ToLower(k)] {
			continue
		}
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		if strings.ContainsAny(s, " \t") || len(s) > 20 {
			return s, true
		}
	}
	return "", false
}
