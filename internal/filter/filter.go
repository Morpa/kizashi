// Package filter interpreta expressões de filtro aplicadas em tempo de render.
// Termos separados por espaço são combinados com AND:
//   - level=error, level>=warn, level<info   → comparação de nível
//   - service=api, status=200                 → substring do campo (top-level, "." = aninhado)
//   - qualquer outra palavra                  → substring case-insensitive da mensagem
//   - -termo ou !termo                        → nega qualquer um dos anteriores (esconde)
package filter

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/Morpa/kizashi/internal/model"
)

type term interface {
	Match(e model.Entry) bool
}

type textTerm struct{ sub string } // substring da mensagem (case-insensitive)

type fieldTerm struct {
	key, val string // val é substring do valor do campo
}

type levelTerm struct {
	op  string // "=", ">", ">=", "<", "<="
	lvl model.Level
}

// notTerm inverte o resultado de outro termo (prefixo "-" ou "!").
type notTerm struct{ inner term }

func (t notTerm) Match(e model.Entry) bool { return !t.inner.Match(e) }

var levelOpRe = regexp.MustCompile(`(?i)^level(>=|<=|>|<|=)(.+)$`)

// Filter combina termos com AND. A expressão original fica guardada para
// exibição na status bar.
type Filter struct {
	Text  string
	terms []term
}

// Parse interpreta uma expressão de filtro. Nunca falha: termos inválidos
// são ignorados. Expressão vazia → filtro vazio (mostra tudo).
func Parse(s string) *Filter {
	f := &Filter{Text: s}
	for tok := range strings.FieldsSeq(s) {
		if t := parseTerm(tok); t != nil {
			f.terms = append(f.terms, t)
		}
	}
	return f
}

func parseTerm(tok string) term {
	if len(tok) > 1 && (tok[0] == '-' || tok[0] == '!') {
		t := parseTerm(tok[1:])
		if t == nil {
			return nil
		}
		return notTerm{inner: t}
	}
	if m := levelOpRe.FindStringSubmatch(tok); m != nil {
		lvl, ok := model.LevelFromName(m[2])
		if !ok {
			return nil
		}
		return levelTerm{op: m[1], lvl: lvl}
	}
	if eq := strings.Index(tok, "="); eq > 0 && isFieldName(tok[:eq]) {
		return fieldTerm{key: tok[:eq], val: tok[eq+1:]}
	}
	return textTerm{sub: strings.ToLower(tok)}
}

// Match avalia a entrada contra todos os termos (AND).
func (f *Filter) Match(e model.Entry) bool {
	for _, t := range f.terms {
		if !t.Match(e) {
			return false
		}
	}
	return true
}

// IsEmpty indica se o filtro não tem termos (mostra tudo).
func (f *Filter) IsEmpty() bool { return len(f.terms) == 0 }

// noiseSuffixes são os sufixos reconhecidos por IsNoise. Padrão cobre a
// convenção mais comum em logs estruturados ("algo-start"/"algo-done"); um
// projeto com outra convenção (ex.: "_begin"/"_end") pode substituir a lista
// via SetNoiseSuffixes (flag --noise da CLI).
var noiseSuffixes = []string{"-start", "-done"}

// SetNoiseSuffixes substitui (não soma) a lista padrão de sufixos de ruído.
// suffixes vazio é um no-op — mantém o padrão. Chamar antes de qualquer
// leitura concorrente (a CLI faz isso na inicialização, antes de subir a UI).
func SetNoiseSuffixes(suffixes []string) {
	if len(suffixes) == 0 {
		return
	}
	noiseSuffixes = suffixes
}

// IsNoise identifica mensagens de "lifecycle" de baixo valor, segundo
// noiseSuffixes. Usada pelo toggle "silenciar ruído" do TUI (tecla `n`),
// independente do filtro de texto do usuário.
func IsNoise(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	for _, suf := range noiseSuffixes {
		if suf == "" {
			continue
		}
		if strings.HasSuffix(t, strings.ToLower(suf)) {
			return true
		}
	}
	return false
}

// Needle devolve o termo de substring simples, se houver, para destacá-lo
// na renderização. Campo/nível não geram destaque.
func (f *Filter) Needle() string {
	for _, t := range f.terms {
		if tt, ok := t.(textTerm); ok {
			return tt.sub
		}
	}
	return ""
}

func (t textTerm) Match(e model.Entry) bool {
	return strings.Contains(strings.ToLower(e.Text), t.sub)
}

func (t fieldTerm) Match(e model.Entry) bool {
	if !e.IsJSON {
		return false
	}
	v, ok := fieldValue(e.Fields, t.key)
	if !ok {
		return false
	}
	return strings.Contains(strings.ToLower(v), strings.ToLower(t.val))
}

func (t levelTerm) Match(e model.Entry) bool {
	switch t.op {
	case "=":
		return e.Level == t.lvl
	case ">":
		return e.Level > t.lvl
	case ">=":
		return e.Level >= t.lvl
	case "<":
		return e.Level < t.lvl
	case "<=":
		return e.Level <= t.lvl
	}
	return false
}

// fieldValue resolve um caminho de campo (possivelmente aninhado via ".") e
// devolve uma representação string do valor.
func fieldValue(m map[string]any, key string) (string, bool) {
	var cur any = m
	for part := range strings.SplitSeq(key, ".") {
		mm, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur, ok = lookupFold(mm, part)
		if !ok {
			return "", false
		}
	}
	switch v := cur.(type) {
	case string:
		return v, true
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(v), true
	default:
		if b, err := json.Marshal(cur); err == nil {
			return string(b), true
		}
		return "", false
	}
}

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

// isFieldName valida um nome de campo: letras, dígitos, '_' e '.' (p/ caminho).
func isFieldName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_' || r == '.':
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
