package cmdutil

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestAddFormatFlagRegistersEnum verifies AddFormatFlag registers the format
// flag with the documented enum annotation for schema/export consumers.
func TestAddFormatFlagRegistersEnum(t *testing.T) {
	cmd := &cobra.Command{}
	var format string

	AddFormatFlag(cmd, &format)

	flag := cmd.Flags().Lookup("format")
	if flag == nil {
		t.Fatal("format flag not registered")
	}
	if flag.DefValue != "" {
		t.Fatalf("format flag default = %q, want empty", flag.DefValue)
	}
	got, ok := flag.Annotations[FlagEnumAnnotation]
	if !ok {
		t.Fatal("format flag missing enum annotation")
	}
	if strings.Join(got, ",") != "json,simple,table" {
		t.Fatalf("format enum annotation = %v, want [json simple table]", got)
	}
}

// TestAddFormatFlagBindsTarget verifies the flag writes parsed values into the
// target variable.
func TestAddFormatFlagBindsTarget(t *testing.T) {
	cmd := &cobra.Command{}
	var format string

	AddFormatFlag(cmd, &format)

	if err := cmd.Flags().Set("format", "json"); err != nil {
		t.Fatalf("Set(format, json) error = %v", err)
	}
	if format != "json" {
		t.Fatalf("target = %q, want %q", format, "json")
	}
}

// TestAddTimeFormatFlagRegistersEnum verifies AddTimeFormatFlag registers the
// time-format flag with its enum annotation.
func TestAddTimeFormatFlagRegistersEnum(t *testing.T) {
	cmd := &cobra.Command{}
	var timeFormat string

	AddTimeFormatFlag(cmd, &timeFormat)

	flag := cmd.Flags().Lookup("time-format")
	if flag == nil {
		t.Fatal("time-format flag not registered")
	}
	got, ok := flag.Annotations[FlagEnumAnnotation]
	if !ok {
		t.Fatal("time-format flag missing enum annotation")
	}
	if strings.Join(got, ",") != "absolute,relative" {
		t.Fatalf("time-format enum annotation = %v, want [absolute relative]", got)
	}
}

// TestAddTemplateFlagRegistersPlainFlag verifies AddTemplateFlag registers the
// template flag without an enum annotation (free-form Go template).
func TestAddTemplateFlagRegistersPlainFlag(t *testing.T) {
	cmd := &cobra.Command{}
	var template string

	AddTemplateFlag(cmd, &template)

	flag := cmd.Flags().Lookup("template")
	if flag == nil {
		t.Fatal("template flag not registered")
	}
	if flag.DefValue != "" {
		t.Fatalf("template flag default = %q, want empty", flag.DefValue)
	}
	if _, ok := flag.Annotations[FlagEnumAnnotation]; ok {
		t.Fatal("template flag should not carry an enum annotation")
	}
	if !strings.Contains(flag.Usage, "Go template") {
		t.Fatalf("template flag usage = %q, want mention of Go template", flag.Usage)
	}
}
