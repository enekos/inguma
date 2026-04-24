package cursor

import "github.com/enekos/inguma/internal/manifest"

// SupportsKind reports what Cursor can host. Cursor is MCP-native and
// has an `.cursor/rules` convention we can best-effort-wrap skills and
// slash commands into. Subagents are Claude-Code-only.
func (a *Adapter) SupportsKind(k manifest.Kind) bool {
	switch k {
	case manifest.KindMCP, manifest.KindCLI, manifest.KindSkill, manifest.KindCommand, manifest.KindBundle:
		return true
	}
	return false
}

func (a *Adapter) CompatibilityNote(k manifest.Kind) string {
	switch k {
	case manifest.KindMCP:
		return "installed as an MCP server in ~/.cursor/mcp.json"
	case manifest.KindSkill:
		return "wrapped as a .cursor/rules/<slug>.mdc rule (best-effort)"
	case manifest.KindSubagent:
		return "not supported on Cursor"
	case manifest.KindCommand:
		return "exposed as an IDE snippet (best-effort)"
	case manifest.KindBundle:
		return "members installed per their own rules"
	}
	return ""
}
