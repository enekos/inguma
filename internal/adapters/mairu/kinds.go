package mairu

import "github.com/enekos/inguma/internal/manifest"

// SupportsKind reports what Mairu can host. Mairu consumes MCP servers
// via ~/.config/mairu/mcp.json and stores skills/memories in its
// Meilisearch index (managed through the `mairu` CLI). Subagents and
// slash commands are not part of its model today.
func (a *Adapter) SupportsKind(k manifest.Kind) bool {
	switch k {
	case manifest.KindMCP, manifest.KindCLI, manifest.KindSkill, manifest.KindBundle:
		return true
	}
	return false
}

func (a *Adapter) CompatibilityNote(k manifest.Kind) string {
	switch k {
	case manifest.KindMCP:
		return "installed as an MCP server in ~/.config/mairu/mcp.json"
	case manifest.KindCLI:
		return "CLI package; install surfaces the bin path"
	case manifest.KindSkill:
		return "registered via `mairu skill store ...` in the Meilisearch skills index (best-effort)"
	case manifest.KindSubagent:
		return "not supported on Mairu"
	case manifest.KindCommand:
		return "not supported on Mairu"
	case manifest.KindBundle:
		return "members installed per their own rules"
	}
	return ""
}
