package urldetect

import "testing"

func TestTrim(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantURL string
		wantLed int
	}{
		{"plain", "https://example.com", "https://example.com", 0},
		{"trailing period", "https://example.com.", "https://example.com", 0},
		{"trailing comma", "https://example.com,", "https://example.com", 0},
		{"query string", "https://youtube.com?id=123", "https://youtube.com?id=123", 0},
		{"comma query", "https://api.com?ids=1,2,3", "https://api.com?ids=1,2,3", 0},
		{"backtick wrapped", "`https://google.com`", "https://google.com", 1},
		{"paren wrapped", "(https://example.com)", "https://example.com", 1},
		{"balanced parens kept", "https://en.wikipedia.org/wiki/Function_(mathematics)", "https://en.wikipedia.org/wiki/Function_(mathematics)", 0},
		{"unbalanced trailing paren", "https://example.com/foo)", "https://example.com/foo", 0},
		{"wrapped balanced parens", "(https://en.wikipedia.org/wiki/Function_(mathematics))", "https://en.wikipedia.org/wiki/Function_(mathematics)", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			url, lead := Trim(c.raw)
			if url != c.wantURL {
				t.Errorf("Trim(%q) url = %q, want %q", c.raw, url, c.wantURL)
			}
			if lead != c.wantLed {
				t.Errorf("Trim(%q) lead = %d, want %d", c.raw, lead, c.wantLed)
			}
		})
	}
}

func TestPatternMatchesWikipediaURL(t *testing.T) {
	in := "see https://en.wikipedia.org/wiki/Function_(mathematics) now"
	loc := Pattern().FindStringIndex(in)
	if loc == nil {
		t.Fatal("no match")
	}
	if got := in[loc[0]:loc[1]]; got != "https://en.wikipedia.org/wiki/Function_(mathematics)" {
		t.Errorf("matched %q", got)
	}
}
