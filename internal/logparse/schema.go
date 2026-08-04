package logparse

import (
	"fmt"
	"strings"
)

// Schema mapeia os papéis semânticos (level/time/message/service) para os
// caminhos de campo do payload. Caminhos podem ser aninhados via ".".
// Um caminho configurado REPÕE a detecção automática daquele papel.
type Schema struct {
	Level   string
	Time    string
	Message string
	Service string
}

// DefaultSchema devolve um schema vazio (tudo por detecção automática).
func DefaultSchema() *Schema { return &Schema{} }

// ParseSchema interpreta uma especificação comma-separada "role=path".
// Ex.: "level=severity,time=ts,msg=message". spec vazio → (nil, nil).
//
// Roles: level, time, message (alias msg), service.
func ParseSchema(spec string) (*Schema, error) {
	if spec == "" {
		return nil, nil
	}
	s := &Schema{}
	seen := map[string]bool{}
	for seg := range strings.SplitSeq(spec, ",") {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			return nil, fmt.Errorf("segmento vazio em %q", spec)
		}
		eq := strings.Index(seg, "=")
		if eq <= 0 {
			return nil, fmt.Errorf("esperava role=path, got %q", seg)
		}
		role, path := seg[:eq], seg[eq+1:]

		canonical := role
		if role == "msg" {
			canonical = "message"
		}
		switch canonical {
		case "level", "time", "message", "service":
		default:
			return nil, fmt.Errorf("role desconhecido %q (válidos: level, time, message/msg, service)", role)
		}
		if seen[canonical] {
			return nil, fmt.Errorf("role %q duplicado em %q", canonical, spec)
		}
		seen[canonical] = true
		if !validPath(path) {
			return nil, fmt.Errorf("caminho inválido %q", path)
		}
		switch canonical {
		case "level":
			s.Level = path
		case "time":
			s.Time = path
		case "message":
			s.Message = path
		case "service":
			s.Service = path
		}
	}
	return s, nil
}

// validPath valida componente(.component)*, cada componente começando com
// letra ou '_' e contendo letras/dígitos/'_'.
func validPath(p string) bool {
	if p == "" {
		return false
	}
	for _, part := range strings.Split(p, ".") {
		if !validComponent(part) {
			return false
		}
	}
	return true
}

func validComponent(c string) bool {
	if c == "" {
		return false
	}
	for i, r := range c {
		switch {
		case r == '_':
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
