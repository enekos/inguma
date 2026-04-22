// Shared types that mirror the apid JSON contract.
// Kept deliberately narrow — only the fields the UI reads.

export type Kind = 'mcp' | 'cli';

export interface IndexEntry {
  slug: string;
  display_name: string;
  description: string;
  kind: Kind;
  categories?: string[];
  tags?: string[];
  harnesses?: string[];
  platforms?: string[];
}

export interface Manifest {
  name: string;
  display_name: string;
  description: string;
  readme: string;
  license: string;
  kind: Kind;
  homepage?: string;
  categories?: string[];
  tags?: string[];
  compatibility: { harnesses: string[]; platforms: string[] };
  mcp?: {
    transport: 'stdio' | 'http';
    command?: string;
    args?: string[];
    url?: string;
    env?: { name: string; required: boolean; description?: string }[];
  };
  cli?: {
    bin: string;
    install: { type: 'npm' | 'go' | 'binary'; package?: string; module?: string; url_template?: string }[];
  };
}

export interface ToolResponse {
  slug: string;
  manifest: Manifest;
  readme: string;
}

export interface SearchHit {
  slug: string;
  score: number;
  tool: IndexEntry;
}

export interface Snippet {
  harness_id: string;
  display_name: string;
  format: 'json' | 'toml' | 'yaml' | 'shell';
  path?: string;
  content: string;
}

export interface InstallResponse {
  slug: string;
  cli: { command: string };
  snippets: Snippet[];
}

export interface CategoryCount {
  name: string;
  count: number;
}
