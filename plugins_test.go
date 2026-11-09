package main

import (
	"testing"
)

func TestPageStats(t *testing.T) {
	for _, c := range []struct {
		in           string
		pages, empty int
	}{
		{"enough text on this page\f", 1, 0},
		{"enough text on this page\f \f", 2, 1},
		{"\f\f", 2, 2},
	} {
		if p, e := pageStats(c.in); p != c.pages || e != c.empty {
			t.Errorf("%q: %d %d", c.in, p, e)
		}
	}
}

func TestLanguagesStayWithinTheImage(t *testing.T) {
	have := []string{"eng", "deu"}
	j := job{Settings: map[string]any{"languages": []any{"deu", "klingon"}}}
	if got := languages(j, have); got != "deu" {
		t.Fatalf("got %q", got)
	}
	if got := languages(job{}, have); got != "eng+deu" {
		t.Fatalf("default %q", got)
	}
}
