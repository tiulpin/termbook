package diff

import (
	"reflect"
	"testing"

	"github.com/tiulpin/termbook/internal/config"
)

func TestRedactor(t *testing.T) {
	r, err := NewRedactor([]config.RedactRule{
		{Pattern: `\d{4}-\d{2}-\d{2}`, Replace: "DATE"},
		{Pattern: `\b[a-f0-9]{40}\b`, Replace: "HASH"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := string(r.Apply([]byte("built 2026-05-01 sha=abcdef0123456789abcdef0123456789abcdef01")))
	want := "built DATE sha=HASH"
	if got != want {
		t.Errorf("redactor: got %q, want %q", got, want)
	}
}

func TestRedactorBackref(t *testing.T) {
	r, _ := NewRedactor([]config.RedactRule{
		{Pattern: `took (\d+)ms`, Replace: "took ${1}MS"},
	})
	got := string(r.Apply([]byte("took 42ms")))
	want := "took 42MS"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDiffLinesNoChange(t *testing.T) {
	hunks := DiffLines([]string{"a", "b", "c"}, []string{"a", "b", "c"})
	for _, h := range hunks {
		if h.Op != ' ' {
			t.Errorf("expected only context lines, got %c %q", h.Op, h.Line)
		}
	}
}

func TestDiffLinesAddRemove(t *testing.T) {
	hunks := DiffLines([]string{"a", "b", "c"}, []string{"a", "X", "c"})
	want := []Hunk{
		{' ', "a"},
		{'-', "b"},
		{'+', "X"},
		{' ', "c"},
	}
	if !reflect.DeepEqual(hunks, want) {
		t.Errorf("got %+v, want %+v", hunks, want)
	}
}

func TestDiffLinesAllAdded(t *testing.T) {
	hunks := DiffLines(nil, []string{"a", "b"})
	want := []Hunk{{'+', "a"}, {'+', "b"}}
	if !reflect.DeepEqual(hunks, want) {
		t.Errorf("got %+v, want %+v", hunks, want)
	}
}

func TestDiffLinesAllRemoved(t *testing.T) {
	hunks := DiffLines([]string{"a", "b"}, nil)
	want := []Hunk{{'-', "a"}, {'-', "b"}}
	if !reflect.DeepEqual(hunks, want) {
		t.Errorf("got %+v, want %+v", hunks, want)
	}
}

func TestSplitLinesCRLF(t *testing.T) {
	got := splitLines([]byte("a\r\nb\r\n"))
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
