package app

import (
	"bytes"
	"cmp"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func TestConfigFieldsAreAlphabetical(t *testing.T) {
	configType := reflect.TypeFor[Config]()
	names := make([]string, configType.NumField())
	for index := range names {
		names[index] = configType.Field(index).Name
	}
	if !slices.IsSorted(names) {
		t.Fatalf("Config fields are not alphabetical: %v", names)
	}
}

func TestInitSettingsAreAlphabetical(t *testing.T) {
	directory := t.TempDir()
	if err := Run([]string{"init", "--chdir", directory}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, ".tfstylus.hcl")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed, diagnostics := parseHCLSource(path, source)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics.Error())
	}
	body, ok := parsed.body.(*hclsyntax.Body)
	if !ok {
		t.Fatal("generated config is not HCL syntax")
	}
	attributes := make([]*hclsyntax.Attribute, 0, len(body.Attributes))
	for _, attribute := range body.Attributes {
		attributes = append(attributes, attribute)
	}
	slices.SortFunc(attributes, func(left, right *hclsyntax.Attribute) int {
		return cmp.Compare(left.SrcRange.Start.Byte, right.SrcRange.Start.Byte)
	})
	names := make([]string, len(attributes))
	for index, attribute := range attributes {
		names[index] = attribute.Name
	}
	if len(names) != reflect.TypeFor[Config]().NumField() || !slices.IsSorted(names) {
		t.Fatalf("generated settings do not contain all fields in alphabetical order: %v", names)
	}
}

func TestLoadConfigValidatesSettingsAlphabetically(t *testing.T) {
	names := []string{"replace_required_version"}
	for _, rule := range configRuleRenames {
		names = append(names, rule.name)
	}
	slices.Sort(names)
	for index, name := range names {
		t.Run(name, func(t *testing.T) {
			var source strings.Builder
			for other := len(names) - 1; other >= index; other-- {
				source.WriteString(names[other] + " = null\n")
			}
			path := filepath.Join(t.TempDir(), "config.hcl")
			writeConfig(t, path, source.String())
			_, err := loadConfig(path)
			if err == nil || !strings.Contains(err.Error(), name+" must be ") {
				t.Fatalf("loadConfig() = %v; want first invalid setting %q", err, name)
			}
		})
	}
}
