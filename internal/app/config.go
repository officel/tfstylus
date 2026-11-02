package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

type Config struct {
	OrderAction            bool
	OrderBackend           bool
	OrderCheck             bool
	OrderCloud             bool
	OrderCloudWorkspaces   bool
	OrderData              bool
	OrderEphemeral         bool
	OrderImport            bool
	OrderList              bool
	OrderModule            bool
	OrderMoved             bool
	OrderOutput            bool
	OrderProvider          bool
	OrderRemoved           bool
	OrderRequiredProviders bool
	OrderRequiredVersion   bool
	OrderResource          bool
	OrderVariable          bool
	RenameLocalsFile       bool
	RenameOutputsFile      bool
	RenameTerraformFile    bool
	RenameVariablesFile    bool
	ReplaceRequiredVersion string
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

func loadConfig(path string) (Config, error) {
	config := Config{
		OrderAction:            true,
		OrderBackend:           true,
		OrderCheck:             true,
		OrderCloud:             true,
		OrderCloudWorkspaces:   true,
		OrderData:              true,
		OrderEphemeral:         true,
		OrderImport:            true,
		OrderList:              true,
		OrderModule:            true,
		OrderMoved:             true,
		OrderOutput:            true,
		OrderProvider:          true,
		OrderRemoved:           true,
		OrderRequiredProviders: true,
		OrderRequiredVersion:   true,
		OrderResource:          true,
		OrderVariable:          true,
		RenameLocalsFile:       true,
		RenameOutputsFile:      true,
		RenameTerraformFile:    true,
		RenameVariablesFile:    true,
	}
	source, err := os.ReadFile(path) // #nosec G304 -- Config paths are explicitly selected by the CLI user or found by config discovery.
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	file, diagnostics := hclsyntax.ParseConfig(source, path, hcl.Pos{Line: 1, Column: 1})
	if diagnostics.HasErrors() {
		return Config{}, fmt.Errorf("parse config %q: %s", path, diagnostics.Error())
	}

	content, diagnostics := file.Body.Content(&hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{Name: "order_action"},
			{Name: "order_backend"},
			{Name: "order_check"},
			{Name: "order_cloud"},
			{Name: "order_cloud_workspaces"},
			{Name: "order_data"},
			{Name: "order_ephemeral"},
			{Name: "order_import"},
			{Name: "order_list"},
			{Name: "order_module"},
			{Name: "order_moved"},
			{Name: "order_output"},
			{Name: "order_provider"},
			{Name: "order_removed"},
			{Name: "order_required_providers"},
			{Name: "order_required_version"},
			{Name: "order_resource"},
			{Name: "order_variable"},
			{Name: "rename_locals_file"},
			{Name: "rename_outputs_file"},
			{Name: "rename_terraform_file"},
			{Name: "rename_variables_file"},
			{Name: "replace_required_version"},
		},
	})
	if diagnostics.HasErrors() {
		return Config{}, fmt.Errorf("validate config %q: %s", path, diagnostics.Error())
	}

	if attribute, exists := content.Attributes["order_action"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_action must be a boolean literal", path)
		}
		config.OrderAction = value.True()
	}

	if attribute, exists := content.Attributes["order_backend"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_backend must be a boolean literal", path)
		}
		config.OrderBackend = value.True()
	}
	if attribute, exists := content.Attributes["order_check"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_check must be a boolean literal", path)
		}
		config.OrderCheck = value.True()
	}
	if attribute, exists := content.Attributes["order_cloud"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_cloud must be a boolean literal", path)
		}
		config.OrderCloud = value.True()
	}
	if attribute, exists := content.Attributes["order_cloud_workspaces"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_cloud_workspaces must be a boolean literal", path)
		}
		config.OrderCloudWorkspaces = value.True()
	}
	if attribute, exists := content.Attributes["order_data"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_data must be a boolean literal", path)
		}
		config.OrderData = value.True()
	}
	if attribute, exists := content.Attributes["order_ephemeral"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_ephemeral must be a boolean literal", path)
		}
		config.OrderEphemeral = value.True()
	}

	if attribute, exists := content.Attributes["order_import"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_import must be a boolean literal", path)
		}
		config.OrderImport = value.True()
	}
	if attribute, exists := content.Attributes["order_list"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_list must be a boolean literal", path)
		}
		config.OrderList = value.True()
	}
	if attribute, exists := content.Attributes["order_module"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_module must be a boolean literal", path)
		}
		config.OrderModule = value.True()
	}
	if attribute, exists := content.Attributes["order_moved"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_moved must be a boolean literal", path)
		}
		config.OrderMoved = value.True()
	}
	if attribute, exists := content.Attributes["order_output"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_output must be a boolean literal", path)
		}
		config.OrderOutput = value.True()
	}
	if attribute, exists := content.Attributes["order_provider"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_provider must be a boolean literal", path)
		}
		config.OrderProvider = value.True()
	}
	if attribute, exists := content.Attributes["order_removed"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_removed must be a boolean literal", path)
		}
		config.OrderRemoved = value.True()
	}
	if attribute, exists := content.Attributes["order_required_providers"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_required_providers must be a boolean literal", path)
		}
		config.OrderRequiredProviders = value.True()
	}
	if attribute, exists := content.Attributes["order_required_version"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_required_version must be a boolean literal", path)
		}
		config.OrderRequiredVersion = value.True()
	}
	if attribute, exists := content.Attributes["order_resource"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_resource must be a boolean literal", path)
		}
		config.OrderResource = value.True()
	}
	if attribute, exists := content.Attributes["order_variable"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: order_variable must be a boolean literal", path)
		}
		config.OrderVariable = value.True()
	}
	if attribute, exists := content.Attributes["rename_locals_file"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: rename_locals_file must be a boolean literal", path)
		}
		config.RenameLocalsFile = value.True()
	}
	if attribute, exists := content.Attributes["rename_outputs_file"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: rename_outputs_file must be a boolean literal", path)
		}
		config.RenameOutputsFile = value.True()
	}
	if attribute, exists := content.Attributes["rename_terraform_file"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: rename_terraform_file must be a boolean literal", path)
		}
		config.RenameTerraformFile = value.True()
	}
	if attribute, exists := content.Attributes["rename_variables_file"]; exists {
		value, diagnostics := attribute.Expr.Value(nil)
		if diagnostics.HasErrors() {
			return Config{}, fmt.Errorf("evaluate config %q: %s", path, diagnostics.Error())
		}
		if !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.Bool) {
			return Config{}, fmt.Errorf("validate config %q: rename_variables_file must be a boolean literal", path)
		}
		config.RenameVariablesFile = value.True()
	}
	if attribute, exists := content.Attributes["replace_required_version"]; exists {
		if literal, ok := attribute.Expr.(*hclsyntax.LiteralValueExpr); ok && literal.Val.RawEquals(cty.False) {
			config.ReplaceRequiredVersion = ""
		} else {
			constraint, ok := quotedStringLiteral(attribute.Expr, source)
			if !ok || strings.TrimSpace(constraint) == "" {
				return Config{}, fmt.Errorf("validate config %q: replace_required_version must be false or a quoted version constraint", path)
			}
			if _, err := version.NewConstraint(constraint); err != nil {
				return Config{}, fmt.Errorf("validate config %q: replace_required_version: %w", path, err)
			}
			config.ReplaceRequiredVersion = constraint
		}
	}
	return config, nil
}

func quotedStringLiteral(expression hcl.Expression, source []byte) (string, bool) {
	template, ok := expression.(*hclsyntax.TemplateExpr)
	if !ok || !template.IsStringLiteral() {
		return "", false
	}
	expressionRange := expression.Range()
	if expressionRange.Start.Byte < 0 || expressionRange.End.Byte > len(source) || expressionRange.End.Byte <= expressionRange.Start.Byte {
		return "", false
	}
	raw := source[expressionRange.Start.Byte:expressionRange.End.Byte]
	if raw[0] != '"' || raw[len(raw)-1] != '"' {
		return "", false
	}
	value, diagnostics := expression.Value(nil)
	if diagnostics.HasErrors() || !value.IsKnown() || value.IsNull() || !value.Type().Equals(cty.String) {
		return "", false
	}
	return value.AsString(), true
}
