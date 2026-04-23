package manifest

// Kind enumerates the supported tool kinds.
type Kind string

const (
	KindMCP Kind = "mcp"
	KindCLI Kind = "cli"
)

// Tool is the canonical in-memory representation of an agentpop.yaml manifest.
type Tool struct {
	Name          string        `yaml:"name" json:"name"`
	Version       string        `yaml:"version,omitempty" json:"version,omitempty"`
	DisplayName   string        `yaml:"display_name" json:"display_name"`
	Description   string        `yaml:"description" json:"description"`
	Readme        string        `yaml:"readme" json:"readme"`
	Homepage      string        `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	License       string        `yaml:"license" json:"license"`
	Authors       []Author      `yaml:"authors,omitempty" json:"authors,omitempty"`
	Categories    []string      `yaml:"categories,omitempty" json:"categories,omitempty"`
	Tags          []string      `yaml:"tags,omitempty" json:"tags,omitempty"`
	Kind          Kind          `yaml:"kind" json:"kind"`
	MCP           *MCPConfig    `yaml:"mcp,omitempty" json:"mcp,omitempty"`
	CLI           *CLIConfig    `yaml:"cli,omitempty" json:"cli,omitempty"`
	Compatibility Compatibility `yaml:"compatibility" json:"compatibility"`
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
