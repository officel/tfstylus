package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

type Config struct {
	RequiredVersionOrder   bool
	BackendOrder           bool
	CloudOrder             bool
	CloudWorkspacesOrder   bool
	RequiredProvidersOrder bool
	VariableOrder          bool
	OutputOrder            bool
	DataOrder              bool
	ActionOrder            bool
	CheckOrder             bool
	EphemeralOrder         bool
	ResourceOrder          bool
	ModuleOrder            bool
	ListOrder              bool
	ImportOrder            bool
	MovedOrder             bool
	RemovedOrder           bool
	LocalsFileRename       bool
	OutputsFileRename      bool
	VariablesFileRename    bool
}

func discoverConfig(startDir string) (string, error) {
	directory, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("resolve config search directory %q: %w", startDir, err)
	}

	for {
		configPath, found, err := configInDirectory(directory)
		if err != nil {
			return "", err
		}
		if found {
			return configPath, nil
		}

		parent := filepath.Dir(directory)
		if parent == directory {
			homeDirectory, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve user home directory: %w", err)
			}
			homeDirectory, err = filepath.Abs(homeDirectory)
			if err != nil {
				return "", fmt.Errorf("resolve user home directory %q: %w", homeDirectory, err)
			}
			if homeDirectory == directory {
				return "", nil
			}
			configPath, found, err = configInDirectory(homeDirectory)
			if err != nil {
				return "", err
			}
			if found {
				return configPath, nil
			}
			return "", nil
		}
		directory = parent
	}
}

func configInDirectory(directory string) (string, bool, error) {
	candidates := []string{
		filepath.Join(directory, ".tfstylus.hcl"),
		filepath.Join(directory, ".config", ".tfstylus.hcl"),
	}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return "", false, fmt.Errorf("inspect config %q: %w", candidate, err)
		}
		if !info.Mode().IsRegular() {
			return "", false, fmt.Errorf("config %q is not a regular file", candidate)
		}
		return candidate, true, nil
	}
	return "", false, nil
}

func loadConfig(path string) (Config, error) {
	config := Config{RequiredVersionOrder: true, BackendOrder: true, CloudOrder: true, CloudWorkspacesOrder: true, RequiredProvidersOrder: true, VariableOrder: true, OutputOrder: true, DataOrder: true, ActionOrder: true, CheckOrder: true, EphemeralOrder: true, ResourceOrder: true, ModuleOrder: true, ListOrder: true, ImportOrder: true, MovedOrder: true, RemovedOrder: true, LocalsFileRename: true, OutputsFileRename: true, VariablesFileRename: true}
	source, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	file, diagnostics := hclsyntax.ParseConfig(source, path, hcl.Pos{Line: 1, Column: 1})
	if diagnostics.HasErrors() {
		return Config{}, fmt.Errorf("parse config %q: %s", path, diagnostics.Error())
	}

	content, diagnostics := file.Body.Content(&hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{{Name: "required_version_order"}, {Name: "backend_order"}, {Name: "cloud_order"}, {Name: "cloud_workspaces_order"}, {Name: "required_providers_order"}, {Name: "variable_order"}, {Name: "output_order"}, {Name: "data_order"}, {Name: "action_order"}, {Name: "check_order"}, {Name: "ephemeral_order"}, {Name: "resource_order"}, {Name: "module_order"}, {Name: "list_order"}, {Name: "import_order"}, {Name: "moved_order"}, {Name: "removed_order"}, {Name: "locals_file_rename"}, {Name: "outputs_file_rename"}, {Name: "variables_file_rename"}},
	})
	if diagnostics.HasErrors() {
		return Config{}, fmt.Errorf("validate config %q: %s", path, diagnostics.Error())
	}

	if attribute, exists := content.Attributes["required_version_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: required_version_order must be a boolean literal", path)
		}
		config.RequiredVersionOrder = value.True()
	}
	if attribute, exists := content.Attributes["backend_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: backend_order must be a boolean literal", path)
		}
		config.BackendOrder = value.True()
	}
	if attribute, exists := content.Attributes["cloud_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: cloud_order must be a boolean literal", path)
		}
		config.CloudOrder = value.True()
	}
	if attribute, exists := content.Attributes["cloud_workspaces_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: cloud_workspaces_order must be a boolean literal", path)
		}
		config.CloudWorkspacesOrder = value.True()
	}
	if attribute, exists := content.Attributes["required_providers_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: required_providers_order must be a boolean literal", path)
		}
		config.RequiredProvidersOrder = value.True()
	}

	if attribute, exists := content.Attributes["variable_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: variable_order must be a boolean literal", path)
		}
		config.VariableOrder = value.True()
	}
	if attribute, exists := content.Attributes["output_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: output_order must be a boolean literal", path)
		}
		config.OutputOrder = value.True()
	}
	if attribute, exists := content.Attributes["data_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: data_order must be a boolean literal", path)
		}
		config.DataOrder = value.True()
	}
	if attribute, exists := content.Attributes["action_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: action_order must be a boolean literal", path)
		}
		config.ActionOrder = value.True()
	}
	if attribute, exists := content.Attributes["check_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: check_order must be a boolean literal", path)
		}
		config.CheckOrder = value.True()
	}
	if attribute, exists := content.Attributes["ephemeral_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: ephemeral_order must be a boolean literal", path)
		}
		config.EphemeralOrder = value.True()
	}
	if attribute, exists := content.Attributes["resource_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: resource_order must be a boolean literal", path)
		}
		config.ResourceOrder = value.True()
	}
	if attribute, exists := content.Attributes["module_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: module_order must be a boolean literal", path)
		}
		config.ModuleOrder = value.True()
	}
	if attribute, exists := content.Attributes["list_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: list_order must be a boolean literal", path)
		}
		config.ListOrder = value.True()
	}
	if attribute, exists := content.Attributes["import_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: import_order must be a boolean literal", path)
		}
		config.ImportOrder = value.True()
	}
	if attribute, exists := content.Attributes["moved_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: moved_order must be a boolean literal", path)
		}
		config.MovedOrder = value.True()
	}
	if attribute, exists := content.Attributes["removed_order"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: removed_order must be a boolean literal", path)
		}
		config.RemovedOrder = value.True()
	}
	if attribute, exists := content.Attributes["locals_file_rename"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: locals_file_rename must be a boolean literal", path)
		}
		config.LocalsFileRename = value.True()
	}
	if attribute, exists := content.Attributes["outputs_file_rename"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: outputs_file_rename must be a boolean literal", path)
		}
		config.OutputsFileRename = value.True()
	}
	if attribute, exists := content.Attributes["variables_file_rename"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: variables_file_rename must be a boolean literal", path)
		}
		config.VariablesFileRename = value.True()
	}
	return config, nil
}
