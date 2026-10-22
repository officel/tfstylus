package app

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCommentIgnoreFileDirectiveExcludesSelectedRules(t *testing.T) {
	source := []byte(`# File header.

# tfstylus-ignore-file: resource_order, module_order, resource_order
# The configured ordering is intentionally skipped.

resource "example" "one" {
  depends_on = [local.first]
  count = 1
}

module "one" {
  version = "1.0"
  source = "./child"
}
`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{
		ResourceOrder: true,
		ModuleOrder:   true,
	})
	if !bytes.Equal(candidate, source) || len(changes) != 0 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want unchanged source", candidate, changes, warnings)
	}
}

func TestCommentIgnoreBlockDirectiveExcludesOnlyFollowingBlock(t *testing.T) {
	source := []byte(`# tfstylus-ignore: resource_order
# This block is managed manually.
resource "example" "ignored" {
  depends_on = [local.first]
  count = 1
}

resource "example" "sorted" {
  depends_on = [local.second]
  count = 2
}
`)
	want := []byte(`# tfstylus-ignore: resource_order
# This block is managed manually.
resource "example" "ignored" {
  depends_on = [local.first]
  count = 1
}

resource "example" "sorted" {
  count = 2
  depends_on = [local.second]
}
`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ResourceOrder: true})
	if !bytes.Equal(candidate, want) || len(changes) != 1 || changes[0].Rule != "resource_order" || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, want)
	}
}

func TestCommentIgnoreFileDirectiveKeepsKnownRuleWhenUnknownRuleIsListed(t *testing.T) {
	source := []byte(`# tfstylus-ignore-file: resource_order, typo_order, resource_order
resource "example" "one" {
  depends_on = [local.first]
  count = 1
}
`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ResourceOrder: true})
	if !bytes.Equal(candidate, source) || len(changes) != 0 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want known resource_order to suppress sorting", candidate, changes, warnings)
	}
}

func TestCommentIgnoreFileDirectiveSupportsRequiredVersionReplacement(t *testing.T) {
	source := []byte(`# tfstylus-ignore-file: replace_required_version
terraform {
  required_version = ">= 1.0"
}
`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{
		ReplaceRequiredVersion: ">= 1.7.0",
	})
	if !bytes.Equal(candidate, source) || len(changes) != 0 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want required_version unchanged", candidate, changes, warnings)
	}
}

func TestCommentIgnoreBlockDirectiveSupportsRequiredVersionReplacement(t *testing.T) {
	source := []byte(`# tfstylus-ignore: replace_required_version
terraform {
  required_version = ">= 1.0"
}
`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{
		ReplaceRequiredVersion: ">= 1.7.0",
	})
	if !bytes.Equal(candidate, source) || len(changes) != 0 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want required_version unchanged", candidate, changes, warnings)
	}
}

func TestCommentIgnoreSuppressesTerraformBlockRules(t *testing.T) {
	tests := []struct {
		rule   string
		config Config
		source string
	}{
		{
			rule:   "required_version_order",
			config: Config{RequiredVersionOrder: true},
			source: "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n",
		},
		{
			rule:   "backend_order",
			config: Config{BackendOrder: true},
			source: "terraform {\n  backend \"local\" {}\n  required_version = \">= 1.0\"\n}\n",
		},
		{
			rule:   "cloud_order",
			config: Config{CloudOrder: true},
			source: "terraform {\n  cloud {\n    workspaces { name = \"prod\" }\n    token = \"token\"\n    hostname = \"app.terraform.io\"\n    organization = \"example\"\n  }\n}\n",
		},
		{
			rule:   "cloud_workspaces_order",
			config: Config{CloudWorkspacesOrder: true},
			source: "terraform {\n  cloud {\n    workspaces {\n      tags = [\"prod\"]\n      name = \"prod\"\n    }\n  }\n}\n",
		},
		{
			rule:   "required_providers_order",
			config: Config{RequiredProvidersOrder: true},
			source: "terraform {\n  required_providers {\n    aws = {\n      version = \"~> 5.0\"\n      source = \"hashicorp/aws\"\n    }\n  }\n}\n",
		},
	}
	for _, test := range tests {
		for _, scope := range []string{"file", "block"} {
			t.Run(test.rule+"/"+scope, func(t *testing.T) {
				marker := "# tfstylus-ignore: "
				if scope == "file" {
					marker = "# tfstylus-ignore-file: "
				}
				ignoredSource := []byte(marker + test.rule + "\n" + test.source)
				parsed, diagnostics := parseHCLSource("main.tf", ignoredSource)
				if diagnostics.HasErrors() {
					t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
				}
				candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, test.config)
				if !bytes.Equal(candidate, ignoredSource) || len(changes) != 0 || len(warnings) != 0 {
					t.Fatalf("ignored transform =\n%s\nchanges = %v; warnings = %v; want unchanged source", candidate, changes, warnings)
				}

				baseline, diagnostics := parseHCLSource("main.tf", []byte(test.source))
				if diagnostics.HasErrors() {
					t.Fatalf("parse baseline diagnostics = %s, want none", diagnostics.Error())
				}
				candidate, changes, warnings = transformTerraformOrderWithChanges(baseline, test.config)
				if bytes.Equal(candidate, []byte(test.source)) || len(changes) == 0 || len(warnings) != 0 {
					t.Fatalf("baseline transform =\n%s\nchanges = %v; warnings = %v; want rule to run", candidate, changes, warnings)
				}
				foundRule := false
				for _, change := range changes {
					foundRule = foundRule || change.Rule == test.rule
				}
				if !foundRule {
					t.Fatalf("baseline changes = %v, want rule %q", changes, test.rule)
				}
			})
		}
	}
}

func TestCommentIgnoreFileDirectiveSupportsQueryListRule(t *testing.T) {
	source := []byte(`# tfstylus-ignore-file: list_order
list "aws_instance" "one" {
  provider = aws
  count = 1
}
`)
	parsed, diagnostics := parseHCLSource("query.tfquery.hcl", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformListOrderWithChanges(parsed)
	if !bytes.Equal(candidate, source) || len(changes) != 0 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want unchanged query source", candidate, changes, warnings)
	}
}

func TestCommentIgnoreDoesNotEnableDisabledRule(t *testing.T) {
	source := []byte(`# tfstylus-ignore-file: resource_order
resource "example" "one" {
  depends_on = [local.first]
  count = 1
}
`)

	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{})
	if !bytes.Equal(candidate, source) || len(changes) != 0 || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want unchanged source with disabled rule", candidate, changes, warnings)
	}
}

func TestCommentIgnoreOnlyRecognizesValidStandaloneDirectives(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "blank line separates directive",
			source: `# tfstylus-ignore: resource_order

resource "example" "one" {
  depends_on = [local.first]
  count = 1
}
`,
			want: `# tfstylus-ignore: resource_order

resource "example" "one" {
  count = 1
  depends_on = [local.first]
}
`,
		},
		{
			name: "file directive after first element is not active",
			source: `locals {
  value = "unchanged"
}

# tfstylus-ignore-file: resource_order
resource "example" "one" {
  depends_on = [local.first]
  count = 1
}
`,
			want: `locals {
  value = "unchanged"
}

# tfstylus-ignore-file: resource_order
resource "example" "one" {
  count = 1
  depends_on = [local.first]
}
`,
		},
		{
			name: "attribute comment is not a block directive",
			source: `resource "example" "one" {
  depends_on = [local.first]
  # tfstylus-ignore: resource_order
  count = 1
}
`,
			want: `resource "example" "one" {
  # tfstylus-ignore: resource_order
  count = 1
  depends_on = [local.first]
}
`,
		},
		{
			name: "trailing comment is not a directive",
			source: `resource "example" "one" { # tfstylus-ignore: resource_order
  depends_on = [local.first]
  count = 1
}
`,
			want: `resource "example" "one" { # tfstylus-ignore: resource_order
  count = 1
  depends_on = [local.first]
}
`,
		},
		{
			name: "slash comment is not a directive",
			source: `// tfstylus-ignore: resource_order
resource "example" "one" {
  depends_on = [local.first]
  count = 1
}
`,
			want: `// tfstylus-ignore: resource_order
resource "example" "one" {
  count = 1
  depends_on = [local.first]
}
`,
		},
		{
			name: "extra text makes directive invalid",
			source: `# tfstylus-ignore: resource_order because manual
resource "example" "one" {
  depends_on = [local.first]
  count = 1
}
`,
			want: `# tfstylus-ignore: resource_order because manual
resource "example" "one" {
  count = 1
  depends_on = [local.first]
}
`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, diagnostics := parseHCLSource("main.tf", []byte(test.source))
			if diagnostics.HasErrors() {
				t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
			}
			candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ResourceOrder: true})
			if test.name == "trailing comment is not a directive" {
				if !bytes.Equal(candidate, []byte(test.source)) || len(changes) != 0 || len(warnings) != 1 || warnings[0].Reason != "resource body contains an ambiguous comment attachment" {
					t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want unchanged source with ambiguous-comment warning", candidate, changes, warnings)
				}
				return
			}
			if !bytes.Equal(candidate, []byte(test.want)) || len(changes) != 1 || changes[0].Rule != "resource_order" || len(warnings) != 0 {
				t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want:\n%s", candidate, changes, warnings, test.want)
			}
		})
	}
}

func TestCommentIgnoreTextInStringAndHeredocDoesNotSuppressSorting(t *testing.T) {
	source := []byte(`locals {
  example = "# tfstylus-ignore-file: resource_order"
}

resource "example" "one" {
  depends_on = [local.first]
  user_data = <<-EOT
    # tfstylus-ignore: resource_order
  EOT
  count = 1
}
`)
	parsed, diagnostics := parseHCLSource("main.tf", source)
	if diagnostics.HasErrors() {
		t.Fatalf("parseHCLSource() diagnostics = %s, want none", diagnostics.Error())
	}

	candidate, changes, warnings := transformTerraformOrderWithChanges(parsed, Config{ResourceOrder: true})
	if bytes.Equal(candidate, source) || len(changes) != 1 || changes[0].Rule != "resource_order" || len(warnings) != 0 {
		t.Fatalf("candidate =\n%s\nchanges = %v; warnings = %v; want resource_order to run", candidate, changes, warnings)
	}
	if bytes.Index(candidate, []byte("count = 1")) > bytes.Index(candidate, []byte("depends_on =")) {
		t.Fatalf("candidate did not move count before depends_on:\n%s", candidate)
	}
	if !bytes.Contains(candidate, []byte("# tfstylus-ignore: resource_order")) {
		t.Fatalf("candidate lost heredoc text:\n%s", candidate)
	}
}

func TestCommentIgnorePlanAndApplyPreserveIgnoredFile(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "main.tf")
	source := []byte(`# tfstylus-ignore-file: resource_order
resource "example" "one" {
  depends_on = [local.first]
  count = 1
}
`)
	writeConfig(t, filename, string(source))

	var output bytes.Buffer
	if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil || output.String() != "No changes.\n" {
		t.Fatalf("plan = %v; output = %q, want no changes", err, output.String())
	}
	if got, err := os.ReadFile(filename); err != nil || !bytes.Equal(got, source) {
		t.Fatalf("plan changed ignored file: %q; read error = %v", got, err)
	}

	for iteration := 0; iteration < 2; iteration++ {
		output.Reset()
		if err := Run([]string{"apply", "--chdir", directory}, &output); err != nil || output.Len() != 0 {
			t.Fatalf("apply %d = %v; output = %q, want successful no-op", iteration+1, err, output.String())
		}
		if got, err := os.ReadFile(filename); err != nil || !bytes.Equal(got, source) {
			t.Fatalf("apply %d changed ignored file: %q; read error = %v", iteration+1, got, err)
		}
	}
}

func TestCommentIgnoreFileDirectiveSuppressesFileRename(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "local.tf")
	source := []byte(`# tfstylus-ignore-file: locals_file_rename
locals {
  value = "kept"
}
`)
	writeConfig(t, filename, string(source))

	var output bytes.Buffer
	if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil || output.String() != "No changes.\n" {
		t.Fatalf("plan = %v; output = %q, want no changes", err, output.String())
	}
	if got, err := os.ReadFile(filename); err != nil || !bytes.Equal(got, source) {
		t.Fatalf("plan changed ignored file: %q; read error = %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(directory, "locals.tf")); !os.IsNotExist(err) {
		t.Fatalf("ignored file was renamed: stat error = %v", err)
	}
}

func TestCommentIgnoreFileDirectiveSuppressesTerraformFileRename(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "backend.tf")
	source := []byte(`# tfstylus-ignore-file: terraform_file_rename
terraform {
  required_version = ">= 1.0"
}
`)
	writeConfig(t, filename, string(source))

	var output bytes.Buffer
	if err := Run([]string{"plan", "--chdir", directory}, &output); err != nil || output.String() != "No changes.\n" {
		t.Fatalf("plan = %v; output = %q, want no changes", err, output.String())
	}
	if got, err := os.ReadFile(filename); err != nil || !bytes.Equal(got, source) {
		t.Fatalf("plan changed ignored file: %q; read error = %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(directory, "terraform.tf")); !os.IsNotExist(err) {
		t.Fatalf("ignored file was renamed: stat error = %v", err)
	}
}

func TestCommentIgnoreBlockDirectiveDoesNotSuppressFileRename(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	filename := filepath.Join(directory, "local.tf")
	source := []byte(`# tfstylus-ignore: locals_file_rename
locals {
  value = "kept"
}
`)
	writeConfig(t, filename, string(source))

	var output bytes.Buffer
	err := Run([]string{"plan", "--chdir", directory}, &output)
	var exitError interface{ ExitCode() int }
	if !bytes.Contains(output.Bytes(), []byte("rename to locals.tf")) || !errors.As(err, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("plan = %v; output = %q, want locals.tf rename", err, output.String())
	}
	if got, readErr := os.ReadFile(filename); readErr != nil || !bytes.Equal(got, source) {
		t.Fatalf("plan changed source file: %q; read error = %v", got, readErr)
	}
}
