package catalog

import (
	"fmt"
	"sort"
)

type KitTemplate struct {
	Name           string
	Description    string
	ProviderType   string
	RequiredSkills []string
	Files          []KitFile
}

type KitFile struct {
	Path    string
	Content string
}

var kitTemplates = map[string]KitTemplate{
	"hello-next-tailwind": {
		Name:           "hello-next-tailwind",
		Description:    "Next.js hello world with Tailwind, including planner/builder/reviewer agents and a simple crew",
		ProviderType:   "deepseek",
		RequiredSkills: []string{"file_read_range", "file_write_safe", "echo"},
		Files: []KitFile{
			{
				Path: "agents/hello_planner.yaml",
				Content: `name: hello_planner
description: Plans file changes for a Next.js Tailwind hello world
provider: deepseek
model: deepseek-chat
system_prompt: |
  You are the planner for a Next.js + Tailwind hello world task.
  Produce a concise implementation plan, list exact files to create/update, and state assumptions.
skills:
  - echo
io:
  outputs:
    - name: plan
      produces: [plan/summary, summary/text]
`,
			},
			{
				Path: "agents/hello_builder.yaml",
				Content: `name: hello_builder
description: Builds hello world files for Next.js + Tailwind
provider: deepseek
model: deepseek-chat
system_prompt: |
  You generate and update files for a Next.js App Router hello world with Tailwind styling.
  Prefer writing concise files and keep output deterministic.
skills:
  - file_read_range
  - file_write_safe
io:
  inputs:
    - name: context
      accepts: [plan/summary, summary/text, message/agent]
      merge_policy: append2
      max_tokens: 120
  outputs:
    - name: page
      produces: [frontend/page, summary/code]
metadata:
  context_moment: "synthesis"
`,
			},
			{
				Path: "agents/hello_reviewer.yaml",
				Content: `name: hello_reviewer
description: Reviews generated Next.js hello world output
provider: deepseek
model: deepseek-chat
system_prompt: |
  You review generated code for correctness, simplicity and readability.
  Return concrete fixes if needed.
skills:
  - file_read_range
io:
  inputs:
    - name: context
      accepts: [frontend/page, summary/code, message/agent]
      merge_policy: append2
      max_tokens: 140
  outputs:
    - name: review
      produces: [summary/text, diagnostic/lint]
`,
			},
			{
				Path: "crews/hello_next_tailwind.yaml",
				Content: `name: hello_next_tailwind
description: Baseline hello world flow for Next.js + Tailwind
spec: hello_planner>hello_builder>hello_reviewer
universes:
  - name: baseline
    spec: hello_planner>hello_builder>hello_reviewer
    output_kind: summary/text
  - name: strict
    spec: hello_planner>hello_builder>hello_reviewer
    output_kind: diagnostic/lint
    input_prefix: |
      [universe_hint]
      mode: strict
      enforce clear assumptions and explicit file-level checks.
  - name: synth
    spec: hello_planner>hello_builder>hello_reviewer
    output_kind: plan/summary
    depends_on: [baseline, strict]
    merge_policy: append
    handoff_max_chars: 220
    input_prefix: |
      [universe_hint]
      Compare upstream universes and provide a final concise recommendation.
`,
			},
		},
	},
	"hello-next-shadcn": {
		Name:           "hello-next-shadcn",
		Description:    "Next.js hello world with shadcn/ui style guidance, including agents and crew",
		ProviderType:   "deepseek",
		RequiredSkills: []string{"file_read_range", "file_write_safe", "echo"},
		Files: []KitFile{
			{
				Path: "agents/hello_planner.yaml",
				Content: `name: hello_planner
description: Plans file changes for a Next.js shadcn-styled hello world
provider: deepseek
model: deepseek-chat
system_prompt: |
  You are the planner for a Next.js hello world with shadcn/ui style conventions.
  Produce an implementation plan with exact file targets and assumptions.
skills:
  - echo
io:
  outputs:
    - name: plan
      produces: [plan/summary, summary/text]
`,
			},
			{
				Path: "agents/hello_builder.yaml",
				Content: `name: hello_builder
description: Builds hello world files for Next.js with shadcn-like UI conventions
provider: deepseek
model: deepseek-chat
system_prompt: |
  You generate and update files for a Next.js App Router hello world using shadcn-like component conventions.
  Keep components minimal and composable.
skills:
  - file_read_range
  - file_write_safe
io:
  inputs:
    - name: context
      accepts: [plan/summary, summary/text, message/agent]
      merge_policy: append2
      max_tokens: 120
  outputs:
    - name: page
      produces: [frontend/page, summary/code]
metadata:
  context_moment: "synthesis"
`,
			},
			{
				Path: "agents/hello_reviewer.yaml",
				Content: `name: hello_reviewer
description: Reviews generated Next.js shadcn-style output
provider: deepseek
model: deepseek-chat
system_prompt: |
  You review generated code for correctness, readability and UI consistency.
  Return concrete fixes if needed.
skills:
  - file_read_range
io:
  inputs:
    - name: context
      accepts: [frontend/page, summary/code, message/agent]
      merge_policy: append2
      max_tokens: 140
  outputs:
    - name: review
      produces: [summary/text, diagnostic/lint]
`,
			},
			{
				Path: "crews/hello_next_shadcn.yaml",
				Content: `name: hello_next_shadcn
description: Baseline hello world flow for Next.js with shadcn-style UI guidance
spec: hello_planner>hello_builder>hello_reviewer
universes:
  - name: baseline
    spec: hello_planner>hello_builder>hello_reviewer
    output_kind: summary/text
  - name: strict
    spec: hello_planner>hello_builder>hello_reviewer
    output_kind: diagnostic/lint
    input_prefix: |
      [universe_hint]
      mode: strict
      enforce accessible semantic HTML and clear component boundaries.
  - name: synth
    spec: hello_planner>hello_builder>hello_reviewer
    output_kind: plan/summary
    depends_on: [baseline, strict]
    merge_policy: append
    handoff_max_chars: 220
    input_prefix: |
      [universe_hint]
      Compare upstream universes and provide a final concise recommendation.
`,
			},
		},
	},
	"crud-next-multiverse": {
		Name:           "crud-next-multiverse",
		Description:    "CRUD fullstack setup with typed multiverse crew (contract -> backend -> frontend/test -> synth)",
		ProviderType:   "deepseek",
		RequiredSkills: []string{"file_read_range", "file_write_safe", "http_request", "echo"},
		Files: []KitFile{
			{
				Path: "agents/crud_contract.yaml",
				Content: `name: crud_contract
description: Produces API contract for CRUD feature
provider: deepseek
model: deepseek-chat
system_prompt: |
  Define a concise OpenAPI-style contract for the requested CRUD feature.
  Be explicit about entities, routes, payloads, errors and pagination/search when relevant.
skills:
  - echo
io:
  outputs:
    - name: openapi
      produces: [contract/openapi, summary/api]
`,
			},
			{
				Path: "agents/crud_backend.yaml",
				Content: `name: crud_backend
description: Implements backend routes/controllers/services from contract
provider: deepseek
model: deepseek-chat
system_prompt: |
  Implement backend CRUD layers from the upstream API contract.
  Prefer clear route/controller/service separation and predictable error handling.
skills:
  - file_read_range
  - file_write_safe
io:
  inputs:
    - name: context
      accepts: [contract/openapi, summary/api, message/agent]
      merge_policy: latest
      max_tokens: 160
  outputs:
    - name: api_summary
      produces: [summary/api, backend/route, backend/controller, backend/service]
metadata:
  context_moment: "backend_codegen"
`,
			},
			{
				Path: "agents/crud_frontend.yaml",
				Content: `name: crud_frontend
description: Implements frontend pages/components/forms from backend API summary
provider: deepseek
model: deepseek-chat
system_prompt: |
  Implement frontend CRUD UI from API summary.
  Focus on clear page structure, form states and API client wiring.
skills:
  - file_read_range
  - file_write_safe
io:
  inputs:
    - name: context
      accepts: [summary/api, backend/route, message/agent]
      merge_policy: latest
      max_tokens: 150
  outputs:
    - name: page
      produces: [frontend/page, frontend/form, frontend/component, summary/code]
metadata:
  context_moment: "frontend_codegen"
`,
			},
			{
				Path: "agents/crud_tester.yaml",
				Content: `name: crud_tester
description: Produces test strategy/checks for backend routes
provider: deepseek
model: deepseek-chat
system_prompt: |
  Generate route-focused test plan and checklist from backend outputs.
  Prioritize happy path, validation errors and authorization boundaries.
skills:
  - file_read_range
io:
  inputs:
    - name: context
      accepts: [summary/api, backend/route, message/agent]
      merge_policy: latest
      max_tokens: 140
  outputs:
    - name: routes_tests
      produces: [backend/route, test/integration, summary/text]
`,
			},
			{
				Path: "agents/crud_synth.yaml",
				Content: `name: crud_synth
description: Reconciles contract/backend/frontend/test universes into a final plan
provider: deepseek
model: deepseek-chat
system_prompt: |
  Reconcile upstream universes into one final implementation recommendation.
  Resolve conflicts and list final file-level action items.
skills:
  - echo
io:
  inputs:
    - name: context
      accepts: [contract/openapi, summary/api, frontend/page, backend/route, summary/text, message/agent]
      merge_policy: append3
      max_tokens: 220
  outputs:
    - name: result
      produces: [plan/summary, summary/text]
metadata:
  context_moment: "synthesis"
`,
			},
			{
				Path: "crews/crud_fullstack_multiverse.yaml",
				Content: `name: crud_fullstack_multiverse
description: CRUD fullstack with typed universe channels (contract -> backend -> frontend/test -> synth)
spec: crud_contract

universes:
  - name: u_contract
    spec: crud_contract
    output_port: openapi
    output_kind: contract/openapi
    handoff_max_chars: 260

  - name: u_backend
    spec: crud_backend
    depends_on: [u_contract]
    input_port: context
    output_port: api_summary
    output_kind: summary/api
    merge_policy: latest
    handoff_max_chars: 260
    input_prefix: |
      [universe_hint]
      Implement backend CRUD from upstream OpenAPI contract.

  - name: u_frontend
    spec: crud_frontend
    depends_on: [u_backend]
    input_port: context
    output_port: page
    output_kind: frontend/page
    merge_policy: latest
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      Build frontend CRUD UI from backend API summary.

  - name: u_test
    spec: crud_tester
    depends_on: [u_backend]
    input_port: context
    output_port: routes_tests
    output_kind: backend/route
    merge_policy: latest
    handoff_max_chars: 220
    input_prefix: |
      [universe_hint]
      Produce route-focused tests and backend validation checklist.

  - name: u_synth
    spec: crud_synth
    depends_on: [u_contract, u_backend, u_frontend, u_test]
    input_port: context
    output_port: result
    output_kind: plan/summary
    merge_policy: append
    handoff_max_chars: 260
    input_prefix: |
      [universe_hint]
      Reconcile contract, backend, frontend and tests into one implementation plan.
`,
			},
		},
	},
}

func ListKits() []KitTemplate {
	out := make([]KitTemplate, 0, len(kitTemplates))
	for _, k := range kitTemplates {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func GetKit(name string) (KitTemplate, error) {
	k, ok := kitTemplates[name]
	if !ok {
		return KitTemplate{}, fmt.Errorf("unknown kit %q", name)
	}
	return k, nil
}
