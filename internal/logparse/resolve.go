package logparse

import (
	"strings"
	"time"
)

// fieldKind identifica o papel semântico de um campo no payload.
type fieldKind int

const (
	fieldLevel fieldKind = iota
	fieldTime
	fieldMessage
	fieldService
)

// Tabelas de aliases padrão. Diferente do modelo antigo ("primeira chave da
// tabela vence"), aqui elas são apenas candidatos: a precedência é decidida
// pelo resolvedor (exato no topo > exato aninhado > case-fold no topo >
// case-fold aninhado). Caminhos pontuados ("log.level") são buscados como
// cadeia aninhada.
var (
	levelKeys   = []string{"level", "log.level", "logging.level", "log_level", "loglevel", "severity", "lvl", "levelname"}
	timeKeys    = []string{"time", "timestamp", "@timestamp", "ts", "datetime", "date", "logtime", "logged_at", "time_ms"}
	messageKeys = []string{"msg", "message", "msg_txt", "msgtxt", "body", "@message", "messagetext", "msgtext", "log", "event"}
	serviceKeys = []string{"service", "logger", "logger_name", "loggername", "name", "app", "component", "module", "category"}
)

// metadataKeys agrupa as chaves conhecidas para que nenhuma delas seja
// usada como mensagem no fallback firstMessageLike.
var metadataKeys = func() map[string]bool {
	m := map[string]bool{}
	all := append(append(append(levelKeys, timeKeys...), messageKeys...), serviceKeys...)
	for _, k := range all {
		if dot := strings.LastIndex(k, "."); dot >= 0 {
			k = k[dot+1:] // chaves pontuadas excluem só o componente folha
		}
		m[strings.ToLower(k)] = true
	}
	for _, k := range []string{"error", "err", "status", "code", "stack", "ctx", "context"} {
		m[k] = true
	}
	return m
}()

// maxDepth limita a profundidade da busca aninhada (evita payloads muito
// profundos custarem caro no hot path de ingestão).
const maxDepth = 8

// candidates devolve os caminhos a tentar para um papel. Se o schema tiver
// caminho explícito para o papel, ele REPÕE a tabela padrão (sem merge):
// um override é uma afirmação sobre a forma da fonte.
func candidates(kind fieldKind, schema *Schema) []string {
	if schema != nil {
		switch kind {
		case fieldLevel:
			if schema.Level != "" {
				return []string{schema.Level}
			}
		case fieldTime:
			if schema.Time != "" {
				return []string{schema.Time}
			}
		case fieldMessage:
			if schema.Message != "" {
				return []string{schema.Message}
			}
		case fieldService:
			if schema.Service != "" {
				return []string{schema.Service}
			}
		}
	}
	switch kind {
	case fieldLevel:
		return levelKeys
	case fieldTime:
		return timeKeys
	case fieldMessage:
		return messageKeys
	default:
		return serviceKeys
	}
}

// resolve procura o valor de um papel semântico no payload, com precedência
// por força do sinal (não por ordem da tabela):
//  1. chave exata no topo (cobre também chaves chatas pontuadas: "logging.level")
//  2. caminho exato aninhado, em qualquer profundidade
//  3. case-fold no topo
//  4. case-fold aninhado
func resolve(m map[string]any, kind fieldKind, schema *Schema) (any, bool) {
	return resolveCandidates(m, candidates(kind, schema))
}

// resolveCandidates aplica as 4 passadas do resolvedor a uma lista explícita
// de candidatos (usada por resolve e por fallbacks específicos, como error/err).
func resolveCandidates(m map[string]any, cands []string) (any, bool) {
	// 1. chave exata no topo.
	for _, c := range cands {
		if v, ok := m[c]; ok {
			return v, true
		}
	}
	// 2. caminho exato aninhado (DFS limitado).
	for _, c := range cands {
		if v, ok := lookupPath(m, strings.Split(c, "."), false); ok {
			return v, true
		}
	}
	// 3. case-fold no topo.
	for _, c := range cands {
		if !strings.Contains(c, ".") {
			if v, ok := lookupFold(m, c); ok {
				return v, true
			}
		}
	}
	// 4. case-fold aninhado.
	for _, c := range cands {
		if v, ok := lookupPath(m, strings.Split(c, "."), true); ok {
			return v, true
		}
	}
	return nil, false
}

// lookupFold procura key em m, exata ou case-insensitive.
func lookupFold(m map[string]any, key string) (any, bool) {
	if v, ok := m[key]; ok {
		return v, true
	}
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return nil, false
}

// lookupPath procura a cadeia de componentes (comps) em qualquer
// profundidade do payload, com comparação exata ou case-insensitive.
func lookupPath(m map[string]any, comps []string, fold bool) (any, bool) {
	return lookupPathDepth(m, comps, fold, 0)
}

func lookupPathDepth(m map[string]any, comps []string, fold bool, depth int) (any, bool) {
	if depth > maxDepth || len(comps) == 0 {
		return nil, false
	}
	first := comps[0]
	// tenta casar a cadeia a partir deste nível.
	for k, v := range m {
		match := k == first
		if fold {
			match = strings.EqualFold(k, first)
		}
		if !match {
			continue
		}
		if len(comps) == 1 {
			return v, true
		}
		if nxt, ok := v.(map[string]any); ok {
			if got, ok := lookupPathDepth(nxt, comps[1:], fold, depth+1); ok {
				return got, true
			}
		}
	}
	// não casou aqui: desce procurando a cadeia completa em mapas aninhados.
	for _, v := range m {
		if nxt, ok := v.(map[string]any); ok {
			if got, ok := lookupPathDepth(nxt, comps, fold, depth+1); ok {
				return got, true
			}
		}
	}
	return nil, false
}

// formatEpoch formata epoch em segundos ou milissegundos (hora local).
func formatEpoch(n float64) string {
	if n > 1e11 {
		n /= 1e3 // milissegundos
	}
	return time.Unix(int64(n), 0).Local().Format("2006-01-02 15:04:05.000")
}
