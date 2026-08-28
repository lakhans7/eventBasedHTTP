package fiverr

import (
	"encoding/csv"
	"strings"
	"testing"
)

func TestParseCents(t *testing.T) {
	cases := map[string]int64{
		"":       0,
		"10":     1000,
		"10.50":  1050,
		"$99.99": 9999,
	}
	for in, want := range cases {
		got, err := parseCents(in)
		if err != nil {
			t.Fatalf("parseCents(%q): %v", in, err)
		}
		if got != want {
			t.Errorf("parseCents(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseCentsRejectsGarbage(t *testing.T) {
	if _, err := parseCents("not-a-number"); err == nil {
		t.Fatal("expected an error for a non-numeric amount")
	}
}

func TestParseTimeAcceptsMultipleLayouts(t *testing.T) {
	for _, in := range []string{"2026-01-15", "2026-01-15T10:00:00Z", "2026-01-15 10:00:00"} {
		got, err := parseTime(in)
		if err != nil {
			t.Errorf("parseTime(%q): %v", in, err)
		}
		if got == nil {
			t.Errorf("parseTime(%q) returned nil", in)
		}
	}
}

func TestParseTimeRejectsGarbage(t *testing.T) {
	if _, err := parseTime("not-a-date"); err == nil {
		t.Fatal("expected an error for an unparsable date")
	}
}

func TestHeaderGetIsCaseInsensitive(t *testing.T) {
	r := csv.NewReader(strings.NewReader("Title,Base_Price_Cents\nHello,500\n"))
	h, err := readHeader(r)
	if err != nil {
		t.Fatalf("readHeader: %v", err)
	}
	row, _ := r.Read()
	if got := h.get(row, "title"); got != "Hello" {
		t.Errorf("expected case-insensitive header lookup to find 'title', got %q", got)
	}
}
