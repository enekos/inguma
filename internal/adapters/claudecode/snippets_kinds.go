package claudecode

import (
	"encoding/json"
	"strings"

	"github.com/enekos/inguma/internal/manifest"
)

// skillSnippet documents where a skill lands in ~/.claude/plugins.
func skillSnippet(m manifest.Tool) string {
	obj := map[string]any{
		"_comment": "Install path for skill files",
		"_path":    "~/.claude/plugins/" + pluginDirFor(m.Name) + "/skills/",
		"entry":    m.Skill.Entry,
		"files":    m.Skill.Files,
	}
	b, _ := json.MarshalIndent(obj, "", "  ")
	return string(b)
}

// subagentSnippet mirrors the ~/.claude/agents/<name>.md install layout.
func subagentSnippet(m manifest.Tool) string {
	obj := map[string]any{
		"_comment": "Install path for subagent file",
		"_path":    "~/.claude/agents/" + safeFilename(m.Name) + ".md",
		"entry":    m.Subagent.Entry,
		"model":    m.Subagent.Model,
		"tools":    m.Subagent.Tools,
	}
	b, _ := json.MarshalIndent(obj, "", "  ")
	return string(b)
}

func commandSnippet(m manifest.Tool) string {
	obj := map[string]any{
		"_comment": "Install path for slash command",
		"_path":    "~/.claude/commands/" + strings.TrimPrefix(m.Command.Name, "/") + ".md",
		"entry":    m.Command.Entry,
		"name":     m.Command.Name,
	}
	b, _ := json.MarshalIndent(obj, "", "  ")
	return string(b)
}

func bundleSnippet(m manifest.Tool) string {
	obj := map[string]any{
		"_comment": "Bundle members (installed individually)",
		"includes": m.Bundle.Includes,
	}
	b, _ := json.MarshalIndent(obj, "", "  ")
	return string(b)
}

// pluginDirFor rewrites "@foo/bar" → "foo-bar" per the Claude Code
// plugin layout. Bare slugs pass through unchanged.
func pluginDirFor(name string) string {
	if strings.HasPrefix(name, "@") {
		r := strings.NewReplacer("/", "-")
		return r.Replace(strings.TrimPrefix(name, "@"))
	}
	return name
}

func safeFilename(name string) string {
	return strings.NewReplacer("@", "", "/", "-").Replace(name)
}
