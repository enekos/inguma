package claudecode

import "github.com/enekos/inguma/internal/manifest"

// SupportsKind reports which Track D kinds claude-code can host.
// Claude Code natively understands MCP servers, slash commands,
// skills (via the plugin folder layout), and subagents.
func (a *Adapter) SupportsKind(k manifest.Kind) bool {
	switch k {
	case manifest.KindMCP, manifest.KindCLI, manifest.KindSkill, manifest.KindSubagent, manifest.KindCommand, manifest.KindBundle:
		return true
	}
	return false
}

// CompatibilityNote returns a human-readable note per kind, used by
// the website compatibility grid.
func (a *Adapter) CompatibilityNote(k manifest.Kind) string {
	switch k {
	case manifest.KindMCP:
		return "installed as an MCP server in ~/.claude.json"
	case manifest.KindCLI:
		return "CLI package; install surfaces the bin path"
	case manifest.KindSkill:
		return "installed into ~/.claude/plugins/<owner>-<slug>/skills/"
	case manifest.KindSubagent:
		return "installed into ~/.claude/agents/"
	case manifest.KindCommand:
		return "installed into ~/.claude/commands/"
	case manifest.KindBundle:
		return "bundle members installed per their own rules"
	}
	return ""
}
