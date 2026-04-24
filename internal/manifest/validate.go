package manifest

import (
	"fmt"
	"strings"

	"github.com/enekos/inguma/internal/namespace"
	"github.com/enekos/inguma/internal/permissions"
	"github.com/enekos/inguma/internal/versioning"
)

// MaxCompanions caps how many soft suggestions a publisher can attach.
// Keeps install-time output readable and prevents spam.
const MaxCompanions = 5

// MaxCompanionReasonLen caps the reason string. Forces publishers to be
// concise in the install-time prompt.
const MaxCompanionReasonLen = 120

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
		if err := exclusiveKindSection(t, "mcp"); err != nil {
			return err
		}
	case KindCLI:
		if t.CLI == nil {
			return fmt.Errorf("manifest: kind=cli requires cli section")
		}
		if err := validateCLI(t.CLI); err != nil {
			return err
		}
		if err := exclusiveKindSection(t, "cli"); err != nil {
			return err
		}
	case KindSkill:
		if t.Skill == nil || t.Skill.Entry == "" {
			return fmt.Errorf("manifest: kind=skill requires skill.entry")
		}
		if err := exclusiveKindSection(t, "skill"); err != nil {
			return err
		}
	case KindSubagent:
		if t.Subagent == nil || t.Subagent.Entry == "" {
			return fmt.Errorf("manifest: kind=subagent requires subagent.entry")
		}
		if err := exclusiveKindSection(t, "subagent"); err != nil {
			return err
		}
	case KindCommand:
		if t.Command == nil || t.Command.Entry == "" || t.Command.Name == "" {
			return fmt.Errorf("manifest: kind=command requires command.entry and command.name")
		}
		if err := exclusiveKindSection(t, "command"); err != nil {
			return err
		}
	case KindBundle:
		if t.Bundle == nil || len(t.Bundle.Includes) == 0 {
			return fmt.Errorf("manifest: kind=bundle requires non-empty bundle.includes")
		}
		if err := exclusiveKindSection(t, "bundle"); err != nil {
			return err
		}
	case "":
		return fmt.Errorf("manifest: kind is required")
	default:
		return fmt.Errorf("manifest: kind %q is not supported", t.Kind)
	}
	if err := permissions.Validate(t.Permissions); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}
	if len(t.Compatibility.Harnesses) == 0 {
		return fmt.Errorf("manifest: compatibility.harnesses must list at least one harness (or \"*\")")
	}
	if len(t.Compatibility.Platforms) == 0 {
		return fmt.Errorf("manifest: compatibility.platforms must list at least one platform")
	}
	if err := validateCompanions(t.Name, t.Companions); err != nil {
		return err
	}
	return nil
}

// validateCompanions enforces limits and slug grammar for the soft
// "you probably also want this" pointers.
func validateCompanions(selfSlug string, cs []Companion) error {
	if len(cs) == 0 {
		return nil
	}
	if len(cs) > MaxCompanions {
		return fmt.Errorf("manifest: at most %d companions allowed (got %d)", MaxCompanions, len(cs))
	}
	selfBase := stripCompanionVersion(selfSlug)
	seen := map[string]bool{}
	for i, c := range cs {
		if c.Slug == "" {
			return fmt.Errorf("manifest: companion[%d].slug is required", i)
		}
		if c.Reason == "" {
			return fmt.Errorf("manifest: companion[%d].reason is required", i)
		}
		if len(c.Reason) > MaxCompanionReasonLen {
			return fmt.Errorf("manifest: companion[%d].reason exceeds %d chars", i, MaxCompanionReasonLen)
		}
		base := stripCompanionVersion(c.Slug)
		nm, err := namespace.Parse(base)
		if err != nil {
			return fmt.Errorf("manifest: companion[%d].slug %q invalid: %w", i, c.Slug, err)
		}
		if nm.IsBare {
			return fmt.Errorf("manifest: companion[%d].slug %q must use @owner/slug form", i, c.Slug)
		}
		if base == selfBase {
			return fmt.Errorf("manifest: companion[%d] is a self-reference", i)
		}
		if seen[base] {
			return fmt.Errorf("manifest: companion[%d].slug %q is a duplicate", i, c.Slug)
		}
		seen[base] = true
		if spec := versionSpec(c.Slug); spec != "" {
			if strings.HasPrefix(spec, "^") || strings.HasPrefix(spec, "~") {
				if _, err := versioning.ParseRange(spec); err != nil {
					return fmt.Errorf("manifest: companion[%d] range %q invalid: %w", i, spec, err)
				}
			} else if _, err := versioning.ParseVersion(spec); err != nil {
				return fmt.Errorf("manifest: companion[%d] version %q invalid: %w", i, spec, err)
			}
		}
		if c.Kind != "" {
			ok := false
			for _, k := range AllKinds() {
				if k == c.Kind {
					ok = true
					break
				}
			}
			if !ok {
				return fmt.Errorf("manifest: companion[%d].kind %q not supported", i, c.Kind)
			}
		}
	}
	return nil
}

// stripCompanionVersion returns the slug part before any trailing @version.
func stripCompanionVersion(slug string) string {
	if len(slug) < 2 {
		return slug
	}
	if idx := strings.LastIndex(slug[1:], "@"); idx >= 0 {
		return slug[:idx+1]
	}
	return slug
}

// versionSpec returns the @version suffix from a slug, or "".
func versionSpec(slug string) string {
	if len(slug) < 2 {
		return ""
	}
	if idx := strings.LastIndex(slug[1:], "@"); idx >= 0 {
		return slug[idx+2:]
	}
	return ""
}

// exclusiveKindSection returns an error if any kind-specific section
// other than the expected one is populated.
func exclusiveKindSection(t *Tool, kind string) error {
	type entry struct {
		name    string
		present bool
	}
	sections := []entry{
		{"mcp", t.MCP != nil},
		{"cli", t.CLI != nil},
		{"skill", t.Skill != nil},
		{"subagent", t.Subagent != nil},
		{"command", t.Command != nil},
		{"bundle", t.Bundle != nil},
	}
	for _, s := range sections {
		if s.name == kind {
			continue
		}
		if s.present {
			return fmt.Errorf("manifest: kind=%s must not include %s section", kind, s.name)
		}
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
