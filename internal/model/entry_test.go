package model

import "testing"

func TestLevelFromName(t *testing.T) {
	cases := map[string]Level{
		"info":     LevelInfo,
		"INFO":     LevelInfo,
		"warning":  LevelWarn,
		"warn":     LevelWarn,
		"error":    LevelError,
		"erro":     LevelError,
		"fatal":    LevelFatal,
		"panic":    LevelFatal,
		"debug":    LevelDebug,
		"trace":    LevelTrace,
		"30":       LevelInfo,
		"4":        LevelWarn,
		"50":       LevelError,
		"60":       LevelFatal,
		"critical": LevelFatal,
	}
	for in, want := range cases {
		got, ok := LevelFromName(in)
		if !ok || got != want {
			t.Errorf("LevelFromName(%q) = %v, %v; want %v", in, got, ok, want)
		}
	}
	if _, ok := LevelFromName("banana"); ok {
		t.Error("esperava !ok para nível desconhecido")
	}
}

func TestLevelFromValue(t *testing.T) {
	if l, ok := LevelFromValue(30.0); !ok || l != LevelInfo {
		t.Errorf("LevelFromValue(30.0) = %v, %v", l, ok)
	}
	if l, ok := LevelFromValue("warn"); !ok || l != LevelWarn {
		t.Errorf("LevelFromValue(warn) = %v, %v", l, ok)
	}
	if l, ok := LevelFromValue(true); ok || l != LevelNone {
		t.Errorf("LevelFromValue(true) = %v, %v", l, ok)
	}
}

func TestLevelStringPad(t *testing.T) {
	if got := LevelInfo.String(); got != "INFO " {
		t.Errorf("LevelInfo.String() = %q", got)
	}
	if got := LevelFatal.String(); got != "FATAL" {
		t.Errorf("LevelFatal.String() = %q", got)
	}
}
