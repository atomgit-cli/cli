package cmdutil

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestAddJSONFlagRegistersFalseByDefault verifies AddJSONFlag registers the
// json flag defaulting to false so JSON output stays opt-in.
func TestAddJSONFlagRegistersFalseByDefault(t *testing.T) {
	cmd := &cobra.Command{}
	var jsonOut bool

	AddJSONFlag(cmd, &jsonOut)

	flag := cmd.Flags().Lookup("json")
	if flag == nil {
		t.Fatal("json flag not registered")
	}
	if flag.DefValue != "false" {
		t.Fatalf("json flag default = %q, want %q", flag.DefValue, "false")
	}
	if jsonOut {
		t.Fatal("target = true, want false before parsing")
	}
}

// TestAddJSONFlagBindsTarget verifies the flag writes parsed values into the
// target variable.
func TestAddJSONFlagBindsTarget(t *testing.T) {
	cmd := &cobra.Command{}
	var jsonOut bool

	AddJSONFlag(cmd, &jsonOut)

	if err := cmd.Flags().Set("json", "true"); err != nil {
		t.Fatalf("Set(json, true) error = %v", err)
	}
	if !jsonOut {
		t.Fatal("target = false, want true after Set")
	}
}

func TestWriteJSONNilSliceEmitsEmptyArray(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{
			name:  "nil int slice emits []",
			value: func() interface{} { var s []int; return s }(),
			want:  "[]",
		},
		{
			name:  "nil struct slice emits []",
			value: func() interface{} { var s []struct{ N int }; return s }(),
			want:  "[]",
		},
		{
			name:  "empty int slice emits []",
			value: []int{},
			want:  "[]",
		},
		{
			name:  "non-empty slice emits array",
			value: []int{1, 2, 3},
			want:  "[\n  1,\n  2,\n  3\n]",
		},
		{
			name:  "struct emits object unchanged",
			value: struct{ A int }{A: 1},
			want:  "{\n  \"A\": 1\n}",
		},
		{
			name:  "nil interface still emits null",
			value: nil,
			want:  "null",
		},
		{
			// normalizeNilSlice only inspects a top-level slice; a pointer to a
			// nil slice has Kind==Ptr and is returned unchanged, so the nil
			// slice it points to still marshals as `null`. This locks the
			// intended scope boundary - no current call site passes *[]T.
			name:  "pointer to nil slice still emits null (scope boundary)",
			value: func() interface{} { var s []int; return &s }(),
			want:  "null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := WriteJSON(&buf, tt.value); err != nil {
				t.Fatalf("WriteJSON() error = %v", err)
			}
			got := strings.TrimSpace(buf.String())
			if got != tt.want {
				t.Errorf("WriteJSON(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}
