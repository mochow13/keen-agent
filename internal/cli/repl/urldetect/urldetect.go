package urldetect

import (
	"regexp"
	"strings"
)

var pattern = regexp.MustCompile(`https?://[^\s\x1b]+`)

const trailingPunctuation = ".,;:!?'\"`>"
const leadingPunctuation = "`'\"(<"

func Pattern() *regexp.Regexp { return pattern }

func Trim(raw string) (url string, leadingTrimmed int) {
	url = strings.TrimRight(raw, trailingPunctuation)
	trimmed := strings.TrimLeft(url, leadingPunctuation)
	leadingTrimmed = len(url) - len(trimmed)
	return trimUnbalanced(trimmed), leadingTrimmed
}

// trimUnbalanced drops trailing closing brackets with no matching opener, so
// "(see https://x.com)" loses the ")" but ".../Function_(mathematics)" keeps it.
func trimUnbalanced(url string) string {
	for len(url) > 0 {
		last := url[len(url)-1]
		var open byte
		switch last {
		case ')':
			open = '('
		case ']':
			open = '['
		case '}':
			open = '{'
		default:
			return url
		}
		if strings.Count(url, string(open)) >= strings.Count(url, string(last)) {
			return url
		}
		url = url[:len(url)-1]
	}
	return url
}
