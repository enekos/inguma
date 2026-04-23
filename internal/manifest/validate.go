package manifest

import (
	"fmt"

	"github.com/enekos/agentpop/internal/namespace"
)

// Validate checks a parsed Tool for semantic correctness.
// Syntactic / unknown-key errors are caught earlier in Parse.
func Validate(t *Tool) error {
	if t == nil {
		return fmt.Errorf("manifest: nil tool")
	}
	if t.Name == "" {
		return fmt.Errorf("manifest: name is required")
	}
	if _, err := namespace.Parse(t.Name); err != nil {
		return fmt.Errorf("manifest: name %q invalid: %w", t.Name, err)
	}
	if t.DisplayName == "" {
		return fmt.Errorf("manifest: display_name is required")
	}
	if t.Description == "" {
		return fmt.Errorf("manifest: description is required")
	}
	if t.Readme == "" {
		return fmt.Errorf("manifest: readme path is required")
	}
	if t.License == "" {
		return fmt.Errorf("manifest: license is required")
	}
	switch t.Kind {
	case KindMCP:
		if t.MCP == nil {
			return fmt.Errorf("manifest: kind=mcp requires mcp section")
		}
		if err := validateMCP(t.MCP); err != nil {
			return err
		}
		if t.CLI != nil {
			return fmt.Errorf("manifest: kind=mcp must not include cli section")
		}
	case KindCLI:
		if t.CLI == nil {
			return fmt.Errorf("manifest: kind=cli requires cli section")
		}
		if err := validateCLI(t.CLI); err != nil {
			return err
		}
		if t.MCP != nil {
			return fmt.Errorf("manifest: kind=cli must not include mcp section")
		}
	case "":
		return fmt.Errorf("manifest: kind is required")
	default:
		return fmt.Errorf("manifest: kind %q is not supported (want mcp or cli)", t.Kind)
	}
	if len(t.Compatibility.Harnesses) == 0 {
		return fmt.Errorf("manifest: compatibility.harnesses must list at least one harness (or \"*\")")
	}
	if len(t.Compatibility.Platforms) == 0 {
		return fmt.Errorf("manifest: compatibility.platforms must list at least one platform")
	}
	return nil
}

func validateMCP(m *MCPConfig) error {
	switch m.Transport {
	case "stdio":
		if m.Command == "" {
			return fmt.Errorf("manifest: mcp.command is required for stdio transport")
		}
	case "http":
		if m.URL == "" {
			return fmt.Errorf("manifest: mcp.url is required for http transport")
		}
	case "":
		return fmt.Errorf("manifest: mcp.transport is required")
	default:
		return fmt.Errorf("manifest: mcp.transport %q not supported (want stdio or http)", m.Transport)
	}
	return nil
}

func validateCLI(c *CLIConfig) error {
	if len(c.Install) == 0 {
		return fmt.Errorf("manifest: cli.install must have at least one source")
	}
	for i, s := range c.Install {
		switch s.Type {
		case "npm":
			if s.Package == "" {
				return fmt.Errorf("manifest: cli.install[%d] type=npm requires package", i)
			}
		case "go":
			if s.Module == "" {
				return fmt.Errorf("manifest: cli.install[%d] type=go requires module", i)
			}
		case "binary":
			if s.URLTemplate == "" {
				return fmt.Errorf("manifest: cli.install[%d] type=binary requires url_template", i)
			}
		case "":
			return fmt.Errorf("manifest: cli.install[%d].type is required", i)
		default:
			return fmt.Errorf("manifest: cli.install[%d].type %q not supported", i, s.Type)
		}
	}
	if c.Bin == "" {
		return fmt.Errorf("manifest: cli.bin is required")
	}
	return nil
}
