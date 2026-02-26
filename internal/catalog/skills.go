package catalog

import (
	"fmt"
	"sort"
)

type SkillTemplate struct {
	Name        string
	Description string
	Filename    string
	Content     string
}

var skillTemplates = map[string]SkillTemplate{
	"echo": {
		Name:        "echo",
		Description: "Debug skill that echoes input and args",
		Filename:    "echo.yaml",
		Content: `name: echo
type: echo
description: Echoes input and args back to the runtime
`,
	},
	"file_read": {
		Name:        "file_read",
		Description: "Reads a local file inside the workspace",
		Filename:    "file_read.yaml",
		Content: `name: file_read
type: file_read
description: Reads a local file from the workspace
params:
  max_bytes: 32768
`,
	},
	"file_read_range": {
		Name:        "file_read_range",
		Description: "Reads a line range from a local file inside the workspace",
		Filename:    "file_read_range.yaml",
		Content: `name: file_read_range
type: file_read_range
description: Reads a specific line range from a local file (lower token cost than full reads)
params:
  max_bytes: 32768
`,
	},
	"file_write_safe": {
		Name:        "file_write_safe",
		Description: "Writes a text file inside the workspace (safe defaults, no overwrite by default)",
		Filename:    "file_write_safe.yaml",
		Content: `name: file_write_safe
type: file_write_safe
description: Writes a text file inside the workspace (relative path only, no overwrite by default)
params:
  max_bytes: 131072
  create_dirs: true
  create_if_missing: true
  overwrite_default: false
`,
	},
	"http_request": {
		Name:        "http_request",
		Description: "Generic HTTP request skill (GET by default)",
		Filename:    "http_request.yaml",
		Content: `name: http_request
type: http
description: Makes an HTTP request (safe defaults; user controls URL)
method: GET
timeout_ms: 5000
params:
  max_bytes: 65536
`,
	},
}

func List() []SkillTemplate {
	out := make([]SkillTemplate, 0, len(skillTemplates))
	for _, s := range skillTemplates {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func Get(name string) (SkillTemplate, error) {
	s, ok := skillTemplates[name]
	if !ok {
		return SkillTemplate{}, fmt.Errorf("unknown catalog skill %q", name)
	}
	return s, nil
}
