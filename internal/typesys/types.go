package typesys

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type Kind string

type TypeDef struct {
	Kind        Kind
	Category    string
	Description string
	Aliases     []string
}

type TypedValue struct {
	Kind       Kind
	RefID      string            // artifact id, file path, or external ref
	InlineText string            // inline value for small payloads
	Meta       map[string]string // extra semantic info
	Bytes      int
	TokensHint int
}

const (
	KindPrimitiveString Kind = "primitive/string"
	KindPrimitiveInt    Kind = "primitive/int"
	KindPrimitiveFloat  Kind = "primitive/float"
	KindPrimitiveNumber Kind = "primitive/number"
	KindPrimitiveBool   Kind = "primitive/bool"
	KindPrimitiveNull   Kind = "primitive/null"

	KindTextPlain    Kind = "text/plain"
	KindTextMarkdown Kind = "text/markdown"
	KindTextPath     Kind = "text/path"
	KindTextPrompt   Kind = "text/prompt"
	KindTextDiff     Kind = "text/diff"

	KindCodeGo     Kind = "code/go"
	KindCodeTS     Kind = "code/ts"
	KindCodeJS     Kind = "code/js"
	KindCodeTSX    Kind = "code/tsx"
	KindCodeJSX    Kind = "code/jsx"
	KindCodePython Kind = "code/python"
	KindCodeRust   Kind = "code/rust"
	KindCodeJava   Kind = "code/java"
	KindCodeC      Kind = "code/c"
	KindCodeCPP    Kind = "code/cpp"
	KindCodeBash   Kind = "code/bash"
	KindCodeSQL    Kind = "code/sql"
	KindCodeYAML   Kind = "code/yaml"
	KindCodeTOML   Kind = "code/toml"

	KindJSONValue  Kind = "json/value"
	KindJSONObject Kind = "json/object"
	KindJSONArray  Kind = "json/array"

	KindDataCSV   Kind = "data/csv"
	KindDataTable Kind = "data/table"

	KindContractOpenAPI    Kind = "contract/openapi"
	KindContractJSONSchema Kind = "contract/json-schema"

	KindDBSchema    Kind = "db/schema"
	KindDBMigration Kind = "db/migration"

	KindBackendRoute      Kind = "backend/route"
	KindBackendController Kind = "backend/controller"
	KindBackendService    Kind = "backend/service"
	KindBackendRepository Kind = "backend/repository"

	KindFrontendPage      Kind = "frontend/page"
	KindFrontendComponent Kind = "frontend/component"
	KindFrontendForm      Kind = "frontend/form"
	KindFrontendClientAPI Kind = "frontend/client-api"

	KindArtifactRef     Kind = "artifact/ref"
	KindArtifactBlob    Kind = "artifact/blob"
	KindArtifactSummary Kind = "artifact/summary"

	KindToolResult      Kind = "tool/result"
	KindToolError       Kind = "tool/error"
	KindCapabilityTools Kind = "capability/tools"

	KindPlanTask    Kind = "plan/task"
	KindPlanSummary Kind = "plan/summary"

	KindDiagnosticLint  Kind = "diagnostic/lint"
	KindDiagnosticTest  Kind = "diagnostic/test"
	KindDiagnosticBuild Kind = "diagnostic/build"

	KindMemoryFact     Kind = "memory/fact"
	KindMemoryQuestion Kind = "memory/question"
	KindMemorySummary  Kind = "memory/summary"

	KindMessageUser      Kind = "message/user"
	KindMessageAgent     Kind = "message/agent"
	KindMessageSystem    Kind = "message/system"
	KindMessageTool      Kind = "message/tool"
	KindMessageAssistant Kind = "message/assistant"

	KindSummaryCode Kind = "summary/code"
	KindSummaryText Kind = "summary/text"
	KindSummaryAPI  Kind = "summary/api"

	KindTestUnit        Kind = "test/unit"
	KindTestIntegration Kind = "test/integration"
	KindTestE2E         Kind = "test/e2e"

	KindContextCompiled Kind = "context/compiled"
)

var registry = []TypeDef{
	{KindPrimitiveString, "primitive", "String scalar value.", []string{"string", "str"}},
	{KindPrimitiveInt, "primitive", "Integer scalar value.", []string{"int", "integer"}},
	{KindPrimitiveFloat, "primitive", "Float scalar value.", []string{"float"}},
	{KindPrimitiveNumber, "primitive", "Generic numeric scalar.", []string{"number", "num"}},
	{KindPrimitiveBool, "primitive", "Boolean scalar value.", []string{"bool", "boolean"}},
	{KindPrimitiveNull, "primitive", "Null/empty scalar.", []string{"null", "nil"}},

	{KindTextPlain, "text", "Plain text content.", []string{"text", "plain", "string/text"}},
	{KindTextMarkdown, "text", "Markdown text.", []string{"md", "markdown"}},
	{KindTextPath, "text", "Filesystem path as text.", []string{"path", "filepath"}},
	{KindTextPrompt, "text", "Prompt/instruction text.", []string{"prompt"}},
	{KindTextDiff, "text", "Patch/diff text.", []string{"diff", "patch"}},

	{KindCodeGo, "code", "Go source code.", []string{"go", "code.go", "code_go", "code/go"}},
	{KindCodeTS, "code", "TypeScript source code.", []string{"ts", "typescript", "code.ts"}},
	{KindCodeJS, "code", "JavaScript source code.", []string{"js", "javascript", "code.js"}},
	{KindCodeTSX, "code", "TypeScript JSX source code.", []string{"tsx", "code.tsx"}},
	{KindCodeJSX, "code", "JavaScript JSX source code.", []string{"jsx", "code.jsx"}},
	{KindCodePython, "code", "Python source code.", []string{"py", "python", "code.py"}},
	{KindCodeRust, "code", "Rust source code.", []string{"rs", "rust", "code.rs"}},
	{KindCodeJava, "code", "Java source code.", []string{"java", "code.java"}},
	{KindCodeC, "code", "C source/header code.", []string{"c", "code.c"}},
	{KindCodeCPP, "code", "C++ source/header code.", []string{"cpp", "cc", "cxx", "hpp", "code.cpp"}},
	{KindCodeBash, "code", "Shell script code.", []string{"sh", "bash", "zsh", "shell", "code.sh"}},
	{KindCodeSQL, "code", "SQL code/query.", []string{"sql", "code.sql"}},
	{KindCodeYAML, "code", "YAML document/config.", []string{"yaml", "yml", "code.yaml"}},
	{KindCodeTOML, "code", "TOML document/config.", []string{"toml", "code.toml"}},

	{KindJSONValue, "json", "Generic JSON value.", []string{"json", "code.json"}},
	{KindJSONObject, "json", "JSON object.", []string{"json.object", "json_object"}},
	{KindJSONArray, "json", "JSON array.", []string{"json.array", "json_array"}},

	{KindDataCSV, "data", "CSV tabular data.", []string{"csv", "data.csv"}},
	{KindDataTable, "data", "Logical tabular data (rows/cols).", []string{"table", "tabular"}},

	{KindContractOpenAPI, "contract", "OpenAPI contract/specification.", []string{"openapi", "contract.openapi", "api_contract"}},
	{KindContractJSONSchema, "contract", "JSON Schema contract for API/domain payloads.", []string{"json-schema", "jsonschema", "contract.jsonschema", "contract.json-schema"}},

	{KindDBSchema, "db", "Database schema/model definition.", []string{"schema", "db.schema", "database.schema"}},
	{KindDBMigration, "db", "Database migration script/change.", []string{"migration", "db.migration", "database.migration"}},

	{KindBackendRoute, "backend", "Backend route/handler registration layer.", []string{"route", "routes", "backend.route"}},
	{KindBackendController, "backend", "Backend controller layer.", []string{"controller", "controllers", "backend.controller"}},
	{KindBackendService, "backend", "Backend service/use-case layer.", []string{"service", "services", "backend.service"}},
	{KindBackendRepository, "backend", "Backend repository/data-access layer.", []string{"repository", "repo", "backend.repository"}},

	{KindFrontendPage, "frontend", "Frontend page/screen entrypoint.", []string{"page", "frontend.page"}},
	{KindFrontendComponent, "frontend", "Frontend UI component.", []string{"component", "components", "frontend.component"}},
	{KindFrontendForm, "frontend", "Frontend form component/flow.", []string{"form", "frontend.form"}},
	{KindFrontendClientAPI, "frontend", "Frontend API client integration layer.", []string{"client-api", "client_api", "frontend.client-api"}},

	{KindArtifactRef, "artifact", "Reference to stored artifact (preferred for large payloads).", []string{"artifact", "ref", "artifact.ref"}},
	{KindArtifactBlob, "artifact", "Opaque binary/blob artifact.", []string{"blob", "artifact.blob"}},
	{KindArtifactSummary, "artifact", "Summary of artifact content.", []string{"artifact.summary"}},

	{KindToolResult, "tool", "Structured tool execution result.", []string{"tool_result", "tool.result"}},
	{KindToolError, "tool", "Structured tool execution error.", []string{"tool_error", "tool.error"}},
	{KindCapabilityTools, "capability", "Tool capability descriptor (permissions/capabilities, not payload).", []string{"tools", "toolset", "capability.tools"}},

	{KindPlanTask, "plan", "Task spec/step description.", []string{"task", "plan.task"}},
	{KindPlanSummary, "plan", "Plan summary.", []string{"plan", "plan.summary"}},

	{KindDiagnosticLint, "diagnostic", "Lint diagnostics.", []string{"lint", "diagnostic.lint"}},
	{KindDiagnosticTest, "diagnostic", "Test diagnostics/results.", []string{"test_result", "diagnostic.test"}},
	{KindDiagnosticBuild, "diagnostic", "Build/compile diagnostics.", []string{"build_result", "diagnostic.build"}},

	{KindMemoryFact, "memory", "Persisted fact item.", []string{"fact", "memory.fact"}},
	{KindMemoryQuestion, "memory", "Open question tracked in memory.", []string{"question", "memory.question"}},
	{KindMemorySummary, "memory", "Persisted summary memory.", []string{"memory.summary"}},

	{KindMessageUser, "message", "User message payload.", []string{"msg.user", "user_message"}},
	{KindMessageAgent, "message", "Agent-to-agent message payload.", []string{"msg.agent", "agent_message"}},
	{KindMessageAssistant, "message", "Assistant/model message payload.", []string{"assistant_message", "msg.assistant"}},
	{KindMessageSystem, "message", "System message payload.", []string{"system_message", "msg.system"}},
	{KindMessageTool, "message", "Tool message payload.", []string{"tool_message", "msg.tool"}},

	{KindSummaryCode, "summary", "Compressed summary of code content.", []string{"code_summary", "summary.code"}},
	{KindSummaryText, "summary", "Compressed summary of plain text.", []string{"text_summary", "summary.text"}},
	{KindSummaryAPI, "summary", "Compressed summary of API contract/endpoints.", []string{"api_summary", "summary.api"}},

	{KindTestUnit, "test", "Unit test code or plan.", []string{"unit_test", "test.unit"}},
	{KindTestIntegration, "test", "Integration test code or plan.", []string{"integration_test", "test.integration"}},
	{KindTestE2E, "test", "End-to-end test code or plan.", []string{"e2e_test", "test.e2e"}},

	{KindContextCompiled, "context", "Compiled context window emitted by the runtime compiler.", []string{"compiled_context", "context.compiled"}},
}

var byCanonical map[Kind]TypeDef
var byAlias map[string]Kind

func init() {
	byCanonical = make(map[Kind]TypeDef, len(registry))
	byAlias = map[string]Kind{}
	for _, def := range registry {
		byCanonical[def.Kind] = def
		byAlias[normalize(def.Kind.String())] = def.Kind
		for _, a := range def.Aliases {
			byAlias[normalize(a)] = def.Kind
		}
	}
}

func (k Kind) String() string { return string(k) }

func List() []TypeDef {
	out := make([]TypeDef, len(registry))
	copy(out, registry)
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out
}

func Categories() []string {
	seen := map[string]struct{}{}
	cats := make([]string, 0)
	for _, d := range registry {
		if _, ok := seen[d.Category]; ok {
			continue
		}
		seen[d.Category] = struct{}{}
		cats = append(cats, d.Category)
	}
	sort.Strings(cats)
	return cats
}

func Lookup(raw string) (TypeDef, bool) {
	k, ok := NormalizeKind(raw)
	if !ok {
		return TypeDef{}, false
	}
	d, ok := byCanonical[k]
	return d, ok
}

func NormalizeKind(raw string) (Kind, bool) {
	n := normalize(raw)
	if n == "" {
		return "", false
	}
	k, ok := byAlias[n]
	return k, ok
}

func MustNormalize(raw string) Kind {
	k, ok := NormalizeKind(raw)
	if !ok {
		panic(fmt.Sprintf("unknown type kind %q", raw))
	}
	return k
}

func IsKnown(raw string) bool {
	_, ok := NormalizeKind(raw)
	return ok
}

func InferKindFromPath(path string) Kind {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return KindCodeGo
	case ".ts":
		return KindCodeTS
	case ".js":
		return KindCodeJS
	case ".tsx":
		return KindCodeTSX
	case ".jsx":
		return KindCodeJSX
	case ".py":
		return KindCodePython
	case ".rs":
		return KindCodeRust
	case ".java":
		return KindCodeJava
	case ".c", ".h":
		return KindCodeC
	case ".cc", ".cpp", ".cxx", ".hpp", ".hh":
		return KindCodeCPP
	case ".sh", ".bash", ".zsh":
		return KindCodeBash
	case ".sql":
		return KindCodeSQL
	case ".yaml", ".yml":
		return KindCodeYAML
	case ".toml":
		return KindCodeTOML
	case ".json":
		return KindJSONValue
	case ".csv":
		return KindDataCSV
	case ".md", ".markdown":
		return KindTextMarkdown
	case ".diff", ".patch":
		return KindTextDiff
	case ".txt", "":
		return KindTextPlain
	default:
		return KindTextPlain
	}
}

func normalize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "_", "/")
	s = strings.ReplaceAll(s, ".", "/")
	s = strings.ReplaceAll(s, " ", "")
	if strings.HasPrefix(s, "code/") && strings.Count(s, "/") == 1 {
		return s
	}
	// Accept CODE.GO and CODE/GO style while preserving canonical slash separator.
	return s
}
