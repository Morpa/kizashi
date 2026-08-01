package ingest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fastPoll(t *testing.T) func() {
	t.Helper()
	old := pollInterval()
	SetTailPollInterval(10 * time.Millisecond)
	return func() { SetTailPollInterval(old) }
}

// tailCollector abre uma fonte de arquivo e entrega as linhas em um canal.
func tailCollector(t *testing.T, ctx context.Context, path string) <-chan string {
	t.Helper()
	src, err := FileSource(ctx, path)
	if err != nil {
		t.Fatalf("FileSource: %v", err)
	}
	lines := make(chan string, 256)
	go func() {
		for l := range src.Lines {
			lines <- l
		}
	}()
	return lines
}

func readN(t *testing.T, lines <-chan string, n int) []string {
	t.Helper()
	var got []string
	for len(got) < n {
		select {
		case l := <-lines:
			got = append(got, l)
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout lendo %d linhas; recebeu %v", n, got)
		}
	}
	return got
}

func writeAppend(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

func TestTailAppends(t *testing.T) {
	restore := fastPoll(t)
	defer restore()

	path := filepath.Join(t.TempDir(), "app.log")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	lines := tailCollector(t, ctx, path)

	writeAppend(t, path, "line1\n")
	time.Sleep(30 * time.Millisecond)
	writeAppend(t, path, "line2\n")
	writeAppend(t, path, "line3\n")

	got := readN(t, lines, 3)
	for i, want := range []string{"line1", "line2", "line3"} {
		if got[i] != want {
			t.Errorf("linha %d = %q, esperava %q", i, got[i], want)
		}
	}
}

func TestTailFromEnd(t *testing.T) {
	restore := fastPoll(t)
	defer restore()

	path := filepath.Join(t.TempDir(), "app.log")
	if err := os.WriteFile(path, []byte("antigo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	lines := tailCollector(t, ctx, path)

	writeAppend(t, path, "novo\n")
	got := readN(t, lines, 1)
	if got[0] != "novo" {
		t.Errorf("esperava 'novo' (tail do fim), got %q", got[0])
	}
}

func TestTailRotation(t *testing.T) {
	restore := fastPoll(t)
	defer restore()

	path := filepath.Join(t.TempDir(), "app.log")
	writeAppend(t, path, "old\n")

	ctx := t.Context()
	lines := tailCollector(t, ctx, path)

	// rotação: renomeia o arquivo e cria um novo com conteúdo
	if err := os.Rename(path, path+".1"); err != nil {
		t.Fatal(err)
	}
	writeAppend(t, path, "new1\n")

	got := readN(t, lines, 1)
	if got[0] != "new1" {
		t.Fatalf("após rotação, esperava 'new1', got %q", got[0])
	}

	writeAppend(t, path, "new2\n")
	got = readN(t, lines, 1)
	if got[0] != "new2" {
		t.Fatalf("após nova escrita, esperava 'new2', got %q", got[0])
	}
}

func TestTailTruncate(t *testing.T) {
	restore := fastPoll(t)
	defer restore()

	path := filepath.Join(t.TempDir(), "app.log")
	if err := os.WriteFile(path, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := t.Context()
	lines := tailCollector(t, ctx, path)

	// trunca e reescreve do zero (como "> arquivo")
	if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := readN(t, lines, 1)
	if got[0] != "x" {
		t.Errorf("após truncamento, esperava 'x', got %q", got[0])
	}
}

func TestFileSourceMissing(t *testing.T) {
	_, err := FileSource(context.Background(), filepath.Join(t.TempDir(), "nope.log"))
	if err == nil {
		t.Error("esperava erro para arquivo inexistente")
	}
}
