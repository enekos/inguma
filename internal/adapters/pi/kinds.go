package pi

import "github.com/enekos/inguma/internal/manifest"

// SupportsKind reports what the Pi coding agent can host. Pi has no
// built-in MCP support but pi-mcp-adapter bridges it; skills and
// prompts live as files under ~/.pi/agent/. Subagents are not part of
// Pi's minimal model.
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
		return "installed as an MCP server in ~/.pi/agent/mcp.json (consumed by pi-mcp-adapter)"
	case manifest.KindCLI:
		return "CLI package; install surfaces the bin path"
	case manifest.KindSkill:
		return "dropped into ~/.pi/agent/skills/<slug>/ (best-effort)"
	case manifest.KindSubagent:
		return "not supported on Pi"
	case manifest.KindCommand:
		return "dropped into ~/.pi/agent/prompts/<slug>.md (best-effort)"
	case manifest.KindBundle:
		return "members installed per their own rules"
	}
	return ""
}
