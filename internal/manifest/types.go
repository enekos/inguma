package manifest

import "github.com/enekos/inguma/internal/permissions"

// Kind enumerates the supported tool kinds.
type Kind string

const (
	KindMCP      Kind = "mcp"
	KindCLI      Kind = "cli"
	KindSkill    Kind = "skill"
	KindSubagent Kind = "subagent"
	KindCommand  Kind = "command"
	KindBundle   Kind = "bundle"
)

// AllKinds is used by adapters and the search facet.
func AllKinds() []Kind {
	return []Kind{KindMCP, KindCLI, KindSkill, KindSubagent, KindCommand, KindBundle}
}

// Tool is the canonical in-memory representation of an inguma.yaml manifest.
type Tool struct {
	Name          string             `yaml:"name" json:"name"`
	Version       string             `yaml:"version,omitempty" json:"version,omitempty"`
	DisplayName   string             `yaml:"display_name" json:"display_name"`
	Description   string             `yaml:"description" json:"description"`
	Readme        string             `yaml:"readme" json:"readme"`
	Homepage      string             `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	License       string             `yaml:"license" json:"license"`
	Authors       []Author           `yaml:"authors,omitempty" json:"authors,omitempty"`
	Categories    []string           `yaml:"categories,omitempty" json:"categories,omitempty"`
	Tags          []string           `yaml:"tags,omitempty" json:"tags,omitempty"`
	Kind          Kind               `yaml:"kind" json:"kind"`
	MCP           *MCPConfig         `yaml:"mcp,omitempty" json:"mcp,omitempty"`
	CLI           *CLIConfig         `yaml:"cli,omitempty" json:"cli,omitempty"`
	Skill         *SkillConfig       `yaml:"skill,omitempty" json:"skill,omitempty"`
	Subagent      *SubagentConfig    `yaml:"subagent,omitempty" json:"subagent,omitempty"`
	Command       *CommandConfig     `yaml:"command,omitempty" json:"command,omitempty"`
	Bundle        *BundleConfig      `yaml:"bundle,omitempty" json:"bundle,omitempty"`
	Permissions   *permissions.Block `yaml:"permissions,omitempty" json:"permissions,omitempty"`
	Compatibility Compatibility      `yaml:"compatibility" json:"compatibility"`
	Companions    []Companion        `yaml:"companions,omitempty" json:"companions,omitempty"`
	Synthetic     bool               `yaml:"synthetic,omitempty" json:"synthetic,omitempty"`
	SyntheticRef  string             `yaml:"synthetic_ref,omitempty" json:"synthetic_ref,omitempty"`
}

// Companion is a soft "you probably also want this" pointer published
// alongside a tool. Unlike bundle includes, companions are individually
// opt-in at install time and never participate in lockfile resolution.
type Companion struct {
	Slug   string `yaml:"slug" json:"slug"`     // "@owner/slug" or "@owner/slug@v1.2.3" or "@owner/slug@^1.2"
	Reason string `yaml:"reason" json:"reason"` // one short line shown to the user; required
	Kind   Kind   `yaml:"kind,omitempty" json:"kind,omitempty"`
}

// SkillConfig describes a markdown-plus-references package installed
// into the host's skill registry.
type SkillConfig struct {
	Entry string   `yaml:"entry" json:"entry"`
	Files []string `yaml:"files,omitempty" json:"files,omitempty"`
}

// SubagentConfig describes a Claude Code subagent definition.
type SubagentConfig struct {
	Entry string   `yaml:"entry" json:"entry"`
	Model string   `yaml:"model,omitempty" json:"model,omitempty"` // opus|sonnet|haiku|inherit
	Tools []string `yaml:"tools,omitempty" json:"tools,omitempty"`
}

// CommandConfig describes a slash-command package.
type CommandConfig struct {
	Entry string `yaml:"entry" json:"entry"`
	Name  string `yaml:"name" json:"name"` // "/my-command"
}

// BundleConfig names member packages installed as one set. Bundles are
// flat in v2.0: they may not include other bundles.
type BundleConfig struct {
	Includes []string          `yaml:"includes" json:"includes"`
	Defaults map[string]Member `yaml:"defaults,omitempty" json:"defaults,omitempty"`
}

// Member holds per-member install overrides. Only env/flags are
// honored; permissions and sources can never be overridden.
type Member struct {
	Env   map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Flags []string          `yaml:"flags,omitempty" json:"flags,omitempty"`
}

type Author struct {
	Name string `yaml:"name" json:"name"`
	URL  string `yaml:"url,omitempty" json:"url,omitempty"`
}

type MCPConfig struct {
	Transport string   `yaml:"transport" json:"transport"` // "stdio" | "http"
	Command   string   `yaml:"command,omitempty" json:"command,omitempty"`
	Args      []string `yaml:"args,omitempty" json:"args,omitempty"`
	URL       string   `yaml:"url,omitempty" json:"url,omitempty"`
	Env       []EnvVar `yaml:"env,omitempty" json:"env,omitempty"`
}

type EnvVar struct {
	Name        string `yaml:"name" json:"name"`
	Required    bool   `yaml:"required" json:"required"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type CLIConfig struct {
	Install []InstallSource `yaml:"install" json:"install"`
	Bin     string          `yaml:"bin" json:"bin"`
}

type InstallSource struct {
	Type           string `yaml:"type" json:"type"` // "npm" | "go" | "binary"
	Package        string `yaml:"package,omitempty" json:"package,omitempty"`
	Module         string `yaml:"module,omitempty" json:"module,omitempty"`
	URLTemplate    string `yaml:"url_template,omitempty" json:"url_template,omitempty"`
	SHA256Template string `yaml:"sha256_template,omitempty" json:"sha256_template,omitempty"`
}

type Compatibility struct {
	Harnesses []string `yaml:"harnesses" json:"harnesses"`
	Platforms []string `yaml:"platforms" json:"platforms"`
}
