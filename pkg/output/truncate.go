package output

import "unicode/utf8"

// Truncate cuts value to at most max runes, ending with "..." when the
// ellipsis fits. It is rune-aware: multi-byte text (CJK titles, emoji) never
// splits mid-codepoint. max <= 0 returns the value unchanged; max <= 3 has
// no room for an ellipsis and truncates plainly.
func Truncate(value string, max int) string {
	if max <= 0 || utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}
