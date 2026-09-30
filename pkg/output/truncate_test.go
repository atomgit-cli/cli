package output

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		value string
		max   int
		want  string
	}{
		{name: "short ascii unchanged", value: "hello", max: 10, want: "hello"},
		{name: "exact fit unchanged", value: "hello", max: 5, want: "hello"},
		{name: "ascii truncates with ellipsis", value: "hello world", max: 8, want: "hello..."},
		{name: "cjk never splits mid-codepoint", value: "中文标题长度超过限制的正文内容", max: 10, want: "中文标题长度超..."},
		{name: "cjk exact fit unchanged", value: "中文标题", max: 4, want: "中文标题"},
		{name: "emoji counted per rune", value: "🚀🚀🚀🚀🚀🚀🚀", max: 6, want: "🚀🚀🚀..."},
		{name: "no ellipsis room at max 3", value: "abcdef", max: 3, want: "abc"},
		{name: "max zero returns value", value: "anything", max: 0, want: "anything"},
		{name: "negative max returns value", value: "anything", max: -1, want: "anything"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Truncate(tt.value, tt.max); got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.value, tt.max, got, tt.want)
			}
		})
	}
}
