package wordfilter

import (
	"strings"
	"testing"
)

func newFilter(t *testing.T, csv string) *Filter {
	t.Helper()
	var f Filter
	if _, err := f.Read(strings.NewReader(csv)); err != nil {
		t.Fatal(err)
	}
	return &f
}

func TestContainsBadWord(t *testing.T) {
	f := newFilter(t, "darn,\nheck,check_checkers\nlongestword\n")
	for _, tc := range []struct {
		in   string
		want bool
	}{
		{"darnit", true},
		{"DaRn", true},
		{"d_a_r_n", true}, // underscores stripped from usernames
		{"d4rn", true},    // leetspeak
		{"h3ck", true},
		{"check", false},      // exception
		{"checkers", false},   // exception
		{"longestword", true}, // the longest word must match too
		{"friendly", false},
	} {
		if got := f.ContainsBadWord(tc.in); got != tc.want {
			t.Errorf("ContainsBadWord(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestMask(t *testing.T) {
	f := newFilter(t, "darn\n")
	if got := f.Mask("well darnit that is fine"); got != "well **** that is fine" {
		t.Errorf("Mask = %q", got)
	}
	if got := f.Mask("hello 🐢 world"); got != "hello 🐢 world" {
		t.Errorf("Mask changed clean text: %q", got)
	}
}

func TestZeroValueFlagsNothing(t *testing.T) {
	var f Filter
	if f.ContainsBadWord("anything") {
		t.Error("empty filter flagged a word")
	}
}

func TestReadSkipsBlankAndTrailingCommas(t *testing.T) {
	f := newFilter(t, "\n,\ndarn,\n")
	// "darn," has no exceptions, so it must not be treated as having an empty one.
	if !f.ContainsBadWord("darn") {
		t.Error("darn should be flagged")
	}
}
