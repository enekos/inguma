package manifest

// Kind enumerates the supported tool kinds.
type Kind string

const (
	KindMCP Kind = "mcp"
	KindCLI Kind = "cli"
)

// Tool is the canonical in-memory representation of an agentpop.yaml manifest.
type Tool struct {
	Name          string        `yaml:"name"`
	DisplayName   string        `yaml:"display_name"`
	Description   string        `yaml:"description"`
	Readme        string        `yaml:"readme"`
	Homepage      string        `yaml:"homepage,omitempty"`
	License       string        `yaml:"license"`
	Authors       []Author      `yaml:"authors,omitempty"`
	Categories    []string      `yaml:"categories,omitempty"`
	Tags          []string      `yaml:"tags,omitempty"`
	Kind          Kind          `yaml:"kind"`
	MCP           *MCPConfig    `yaml:"mcp,omitempty"`
	CLI           *CLIConfig    `yaml:"cli,omitempty"`
	Compatibility Compatibility `yaml:"compatibility"`
}

type Author struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url,omitempty"`
}

type MCPConfig struct {
	Transport string   `yaml:"transport"` // "stdio" | "http"
	Command   string   `yaml:"command,omitempty"`
	Args      []string `yaml:"args,omitempty"`
	URL       string   `yaml:"url,omitempty"`
	Env       []EnvVar `yaml:"env,omitempty"`
}

type EnvVar struct {
	Name        string `yaml:"name"`
	Required    bool   `yaml:"required"`
	Description string `yaml:"description,omitempty"`
}

type CLIConfig struct {
	Install []InstallSource `yaml:"install"`
	Bin     string          `yaml:"bin"`
}

type InstallSource struct {
	Type           string `yaml:"type"` // "npm" | "go" | "binary"
	Package        string `yaml:"package,omitempty"`
	Module         string `yaml:"module,omitempty"`
	URLTemplate    string `yaml:"url_template,omitempty"`
	SHA256Template string `yaml:"sha256_template,omitempty"`
}

type Compatibility struct {
	Harnesses []string `yaml:"harnesses"`
	Platforms []string `yaml:"platforms"`
}
