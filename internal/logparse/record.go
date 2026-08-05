package logparse

import (
	"encoding/json"
	"strings"

	"github.com/Morpa/kizashi/internal/model"
)

// Record é o resultado de um parser: payload estruturado normalizado + flag
// de estrutura. O resolvedor extrai os papéis semânticos dele.
type Record struct {
	Fields     map[string]any
	Structured bool
}

// applySchema projeta um Record em Entry (nível/time/serviço/texto) usando o
// resolvedor genérico. schema nil → detecção automática.
func applySchema(rec Record, schema *Schema) (level model.Level, timeStr, service, text string) {
	m := rec.Fields
	if v, ok := resolve(m, fieldLevel, schema); ok {
		level, _ = model.LevelFromValue(v)
	}
	if v, ok := resolve(m, fieldTime, schema); ok {
		switch t := v.(type) {
		case string:
			timeStr = shortenTime(t)
		case float64:
			timeStr = formatEpoch(t)
		}
	}
	if v, ok := resolve(m, fieldService, schema); ok {
		if s, ok := v.(string); ok {
			service = s
		}
	}
	text = extractMessage(m, schema)
	return
}

// extractMessage escolhe a melhor representação textual do log:
// 1. aliases de mensagem, 2. error/err, 3. primeiro campo string
// "parecido com mensagem" (em qualquer profundidade), 4. JSON compacto.
func extractMessage(m map[string]any, schema *Schema) string {
	if v, ok := resolve(m, fieldMessage, schema); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if v, ok := resolveCandidates(m, []string{"error", "err"}); ok {
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

// firstMessageLike procura em qualquer profundidade o primeiro valor string
// que não seja chave de metadado e que pareça uma mensagem (contém espaço ou
// é razoavelmente longo). DFS limitado por maxDepth.
func firstMessageLike(m map[string]any) (string, bool) {
	return firstMessageLikeDepth(m, 0)
}

func firstMessageLikeDepth(m map[string]any, depth int) (string, bool) {
	if depth > maxDepth {
		return "", false
	}
	for k, v := range m {
		if metadataKeys[strings.ToLower(k)] {
			continue
		}
		switch t := v.(type) {
		case string:
			if t == "" {
				continue
			}
			if strings.ContainsAny(t, " \t") || len(t) > 20 {
				return t, true
			}
		case map[string]any:
			if s, ok := firstMessageLikeDepth(t, depth+1); ok {
				return s, true
			}
		}
	}
	return "", false
}