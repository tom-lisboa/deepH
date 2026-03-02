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
		Description:    "Opinionated CRUD setup with typed multiverse crews for Go + Next.js + Postgres (plus document-db variants)",
		ProviderType:   "deepseek",
		RequiredSkills: []string{"file_read_range", "file_write_safe", "http_request", "echo"},
		Files: []KitFile{
			{
				Path: "agents/crud_contract.yaml",
				Content: `name: crud_contract
description: Produces the source-of-truth CRUD contract
provider: deepseek
model: deepseek-chat
system_prompt: |
  Define a concise OpenAPI-style contract for the requested CRUD feature.
  Be explicit about entity fields, ids, payloads, errors and the final CRUD route table.
  Assume the implementation will be evolved by the user later, so keep the contract practical and easy to extend.
skills:
  - echo
io:
  outputs:
    - name: openapi
      produces: [contract/openapi, summary/api]
`,
			},
			{
				Path: "agents/crud_schema.yaml",
				Content: `name: crud_schema
description: Designs relational tables or document collections for the CRUD
provider: deepseek
model: deepseek-chat
system_prompt: |
  Design persistence for the upstream CRUD contract.
  If the prompt says relational, produce table shape, ids, constraints, indexes and an initial migration plan.
  If the prompt says non-relational/document, produce collection shape, ids and indexes.
  Be explicit about fields and storage choices.
skills:
  - echo
io:
  inputs:
    - name: context
      accepts: [contract/openapi, summary/api, message/agent]
      merge_policy: latest
      max_tokens: 170
  outputs:
    - name: schema
      produces: [db/schema, db/migration, summary/text]
metadata:
  context_moment: "schema_design"
`,
			},
			{
				Path: "agents/crud_routes.yaml",
				Content: `name: crud_routes
description: Route specialist for CRUD HTTP endpoints
provider: deepseek
model: deepseek-chat
system_prompt: |
  You are the route specialist for CRUD features.
  Define exact HTTP routes, request/response payloads, validation rules and status codes.
  Always return a final route table that a Go backend can implement directly.
skills:
  - echo
io:
  inputs:
    - name: context
      accepts: [contract/openapi, summary/api, db/schema, summary/text, message/agent]
      merge_policy: append3
      max_tokens: 180
  outputs:
    - name: route_map
      produces: [backend/route, summary/api, summary/text]
metadata:
  context_moment: "route_design"
`,
			},
			{
				Path: "agents/crud_backend.yaml",
				Content: `name: crud_backend
description: Implements the Go backend from contract, schema and routes
provider: deepseek
model: deepseek-chat
system_prompt: |
  Implement the backend CRUD layers from the upstream contract, schema and route map.
  Prefer Go server bootstrap plus clear route/controller/service/repository separation.
  For relational storage, prefer explicit SQL over heavy ORMs.
  For document storage, keep repository code predictable and explicit.
  Make it easy for the user to understand how the server boots locally.
skills:
  - file_read_range
  - file_write_safe
io:
  inputs:
    - name: context
      accepts: [contract/openapi, summary/api, db/schema, backend/route, summary/text, message/agent]
      merge_policy: append4
      max_tokens: 220
  outputs:
    - name: api_summary
      produces: [summary/api, backend/route, backend/controller, backend/service, backend/repository]
metadata:
  context_moment: "backend_codegen"
`,
			},
			{
				Path: "agents/crud_infra.yaml",
				Content: `name: crud_infra
description: Prepares local infrastructure, Docker and startup UX for the CRUD
provider: deepseek
model: deepseek-chat
system_prompt: |
  Implement the local infrastructure for this CRUD app.
  Prefer Dockerfile, docker-compose, env example, healthcheck and smoke commands.
  Make local startup easy through short commands or scripts that the user can evolve later.
skills:
  - file_read_range
  - file_write_safe
io:
  inputs:
    - name: context
      accepts: [db/schema, db/migration, backend/route, summary/api, summary/text, message/agent]
      merge_policy: append4
      max_tokens: 200
  outputs:
    - name: infra_summary
      produces: [db/migration, summary/code, artifact/ref]
metadata:
  context_moment: "infra_codegen"
`,
			},
			{
				Path: "agents/crud_frontend.yaml",
				Content: `name: crud_frontend
description: Implements Next.js CRUD pages and forms from backend outputs
provider: deepseek
model: deepseek-chat
system_prompt: |
  Implement the frontend CRUD UI in Next.js from the backend API summary and route map.
  Focus on clear page structure, form states and API wiring that is easy for the user to evolve.
skills:
  - file_read_range
  - file_write_safe
io:
  inputs:
    - name: context
      accepts: [summary/api, backend/route, summary/text, message/agent]
      merge_policy: append3
      max_tokens: 170
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
description: Produces test strategy and smoke checks for CRUD routes
provider: deepseek
model: deepseek-chat
system_prompt: |
  Generate route-focused tests and smoke checks from the route map, schema and backend outputs.
  Prioritize happy path, validation errors, startup verification and data persistence checks.
skills:
  - file_read_range
io:
  inputs:
    - name: context
      accepts: [summary/api, backend/route, db/schema, summary/text, message/agent]
      merge_policy: append3
      max_tokens: 180
  outputs:
    - name: routes_tests
      produces: [test/integration, summary/text, backend/route]
`,
			},
			{
				Path: "agents/crud_synth.yaml",
				Content: `name: crud_synth
description: Reconciles contract, schema, routes, backend, infra, frontend and tests into the final output
provider: deepseek
model: deepseek-chat
system_prompt: |
  Reconcile upstream universes into one final implementation recommendation.
  Resolve conflicts and list final file-level action items.
  Always include the final CRUD route table and the exact commands to run the server locally.
skills:
  - echo
io:
  inputs:
    - name: context
      accepts: [contract/openapi, db/schema, db/migration, summary/api, frontend/page, backend/route, summary/code, summary/text, artifact/ref, message/agent]
      merge_policy: append4
      max_tokens: 260
  outputs:
    - name: result
      produces: [plan/summary, summary/text]
metadata:
  context_moment: "synthesis"
`,
			},
			{
				Path: "crews/crud_backend_relational.yaml",
				Content: `name: crud_backend_relational
description: Backend-only CRUD with relational data model (contract -> schema -> routes -> backend/infra/test -> synth)
spec: crud_contract

universes:
  - name: u_contract
    spec: crud_contract
    output_port: openapi
    output_kind: contract/openapi
    handoff_max_chars: 260

  - name: u_schema
    spec: crud_schema
    depends_on: [u_contract]
    input_port: context
    output_port: schema
    output_kind: db/schema
    merge_policy: latest
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      Design a relational schema and migration plan for a Go + Postgres CRUD.

  - name: u_routes
    spec: crud_routes
    depends_on: [u_contract, u_schema]
    input_port: context
    output_port: route_map
    output_kind: backend/route
    merge_policy: append
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      You are the route specialist.
      Produce exact CRUD REST routes for a Go server.

  - name: u_backend
    spec: crud_backend
    depends_on: [u_contract, u_schema, u_routes]
    input_port: context
    output_port: api_summary
    output_kind: summary/api
    merge_policy: append
    handoff_max_chars: 260
    input_prefix: |
      [universe_hint]
      Implement the Go backend from contract, relational schema and route map.
      Do not generate frontend.

  - name: u_infra
    spec: crud_infra
    depends_on: [u_schema, u_routes, u_backend]
    input_port: context
    output_port: infra_summary
    output_kind: summary/code
    merge_policy: append
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      Prepare Postgres, Docker Compose and local run UX for a backend-only CRUD.

  - name: u_test
    spec: crud_tester
    depends_on: [u_backend, u_routes, u_schema]
    input_port: context
    output_port: routes_tests
    output_kind: test/integration
    merge_policy: append
    handoff_max_chars: 220
    input_prefix: |
      [universe_hint]
      Produce backend validation, smoke test and route-focused checks.

  - name: u_synth
    spec: crud_synth
    depends_on: [u_contract, u_schema, u_routes, u_backend, u_infra, u_test]
    input_port: context
    output_port: result
    output_kind: plan/summary
    merge_policy: append
    handoff_max_chars: 260
    input_prefix: |
      [universe_hint]
      Reconcile the backend-only relational CRUD.
      Always include final routes and local startup commands.
`,
			},
			{
				Path: "crews/crud_fullstack_relational.yaml",
				Content: `name: crud_fullstack_relational
description: Fullstack CRUD with relational data model (contract -> schema -> routes -> backend/infra/frontend/test -> synth)
spec: crud_contract

universes:
  - name: u_contract
    spec: crud_contract
    output_port: openapi
    output_kind: contract/openapi
    handoff_max_chars: 260

  - name: u_schema
    spec: crud_schema
    depends_on: [u_contract]
    input_port: context
    output_port: schema
    output_kind: db/schema
    merge_policy: latest
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      Design a relational schema and migration plan for a Go + Postgres CRUD.

  - name: u_routes
    spec: crud_routes
    depends_on: [u_contract, u_schema]
    input_port: context
    output_port: route_map
    output_kind: backend/route
    merge_policy: append
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      You are the route specialist.
      Produce exact CRUD REST routes for a Go server.

  - name: u_backend
    spec: crud_backend
    depends_on: [u_contract, u_schema, u_routes]
    input_port: context
    output_port: api_summary
    output_kind: summary/api
    merge_policy: append
    handoff_max_chars: 260
    input_prefix: |
      [universe_hint]
      Implement the Go backend from contract, relational schema and route map.

  - name: u_infra
    spec: crud_infra
    depends_on: [u_schema, u_routes, u_backend]
    input_port: context
    output_port: infra_summary
    output_kind: summary/code
    merge_policy: append
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      Prepare Postgres, Docker Compose and local run UX for a fullstack CRUD.

  - name: u_frontend
    spec: crud_frontend
    depends_on: [u_backend, u_routes]
    input_port: context
    output_port: page
    output_kind: frontend/page
    merge_policy: append
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      Build the Next.js CRUD UI from backend API summary and route map.

  - name: u_test
    spec: crud_tester
    depends_on: [u_backend, u_routes, u_schema]
    input_port: context
    output_port: routes_tests
    output_kind: test/integration
    merge_policy: append
    handoff_max_chars: 220
    input_prefix: |
      [universe_hint]
      Produce backend validation, smoke test and route-focused checks.

  - name: u_synth
    spec: crud_synth
    depends_on: [u_contract, u_schema, u_routes, u_backend, u_infra, u_frontend, u_test]
    input_port: context
    output_port: result
    output_kind: plan/summary
    merge_policy: append
    handoff_max_chars: 260
    input_prefix: |
      [universe_hint]
      Reconcile the fullstack relational CRUD.
      Always include final routes and local startup commands.
`,
			},
			{
				Path: "crews/crud_backend_document.yaml",
				Content: `name: crud_backend_document
description: Backend-only CRUD with document data model (contract -> schema -> routes -> backend/infra/test -> synth)
spec: crud_contract

universes:
  - name: u_contract
    spec: crud_contract
    output_port: openapi
    output_kind: contract/openapi
    handoff_max_chars: 260

  - name: u_schema
    spec: crud_schema
    depends_on: [u_contract]
    input_port: context
    output_port: schema
    output_kind: db/schema
    merge_policy: latest
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      Design a document-oriented schema, ids and indexes for a Go + MongoDB CRUD.

  - name: u_routes
    spec: crud_routes
    depends_on: [u_contract, u_schema]
    input_port: context
    output_port: route_map
    output_kind: backend/route
    merge_policy: append
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      You are the route specialist.
      Produce exact CRUD REST routes for a Go server using a document data model.

  - name: u_backend
    spec: crud_backend
    depends_on: [u_contract, u_schema, u_routes]
    input_port: context
    output_port: api_summary
    output_kind: summary/api
    merge_policy: append
    handoff_max_chars: 260
    input_prefix: |
      [universe_hint]
      Implement the Go backend from contract, document schema and route map.
      Do not generate frontend.

  - name: u_infra
    spec: crud_infra
    depends_on: [u_schema, u_routes, u_backend]
    input_port: context
    output_port: infra_summary
    output_kind: summary/code
    merge_policy: append
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      Prepare MongoDB, Docker Compose and local run UX for a backend-only CRUD.

  - name: u_test
    spec: crud_tester
    depends_on: [u_backend, u_routes, u_schema]
    input_port: context
    output_port: routes_tests
    output_kind: test/integration
    merge_policy: append
    handoff_max_chars: 220
    input_prefix: |
      [universe_hint]
      Produce backend validation, smoke test and route-focused checks.

  - name: u_synth
    spec: crud_synth
    depends_on: [u_contract, u_schema, u_routes, u_backend, u_infra, u_test]
    input_port: context
    output_port: result
    output_kind: plan/summary
    merge_policy: append
    handoff_max_chars: 260
    input_prefix: |
      [universe_hint]
      Reconcile the backend-only document CRUD.
      Always include final routes and local startup commands.
`,
			},
			{
				Path: "crews/crud_fullstack_document.yaml",
				Content: `name: crud_fullstack_document
description: Fullstack CRUD with document data model (contract -> schema -> routes -> backend/infra/frontend/test -> synth)
spec: crud_contract

universes:
  - name: u_contract
    spec: crud_contract
    output_port: openapi
    output_kind: contract/openapi
    handoff_max_chars: 260

  - name: u_schema
    spec: crud_schema
    depends_on: [u_contract]
    input_port: context
    output_port: schema
    output_kind: db/schema
    merge_policy: latest
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      Design a document-oriented schema, ids and indexes for a Go + MongoDB CRUD.

  - name: u_routes
    spec: crud_routes
    depends_on: [u_contract, u_schema]
    input_port: context
    output_port: route_map
    output_kind: backend/route
    merge_policy: append
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      You are the route specialist.
      Produce exact CRUD REST routes for a Go server using a document data model.

  - name: u_backend
    spec: crud_backend
    depends_on: [u_contract, u_schema, u_routes]
    input_port: context
    output_port: api_summary
    output_kind: summary/api
    merge_policy: append
    handoff_max_chars: 260
    input_prefix: |
      [universe_hint]
      Implement the Go backend from contract, document schema and route map.

  - name: u_infra
    spec: crud_infra
    depends_on: [u_schema, u_routes, u_backend]
    input_port: context
    output_port: infra_summary
    output_kind: summary/code
    merge_policy: append
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      Prepare MongoDB, Docker Compose and local run UX for a fullstack CRUD.

  - name: u_frontend
    spec: crud_frontend
    depends_on: [u_backend, u_routes]
    input_port: context
    output_port: page
    output_kind: frontend/page
    merge_policy: append
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      Build the Next.js CRUD UI from backend API summary and route map.

  - name: u_test
    spec: crud_tester
    depends_on: [u_backend, u_routes, u_schema]
    input_port: context
    output_port: routes_tests
    output_kind: test/integration
    merge_policy: append
    handoff_max_chars: 220
    input_prefix: |
      [universe_hint]
      Produce backend validation, smoke test and route-focused checks.

  - name: u_synth
    spec: crud_synth
    depends_on: [u_contract, u_schema, u_routes, u_backend, u_infra, u_frontend, u_test]
    input_port: context
    output_port: result
    output_kind: plan/summary
    merge_policy: append
    handoff_max_chars: 260
    input_prefix: |
      [universe_hint]
      Reconcile the fullstack document CRUD.
      Always include final routes and local startup commands.
`,
			},
			{
				Path: "crews/crud_fullstack_multiverse.yaml",
				Content: `name: crud_fullstack_multiverse
description: Compatibility alias crew for the relational fullstack CRUD path
spec: crud_contract

universes:
  - name: u_contract
    spec: crud_contract
    output_port: openapi
    output_kind: contract/openapi
    handoff_max_chars: 260

  - name: u_schema
    spec: crud_schema
    depends_on: [u_contract]
    input_port: context
    output_port: schema
    output_kind: db/schema
    merge_policy: latest
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      Design a relational schema and migration plan for a Go + Postgres CRUD.

  - name: u_routes
    spec: crud_routes
    depends_on: [u_contract, u_schema]
    input_port: context
    output_port: route_map
    output_kind: backend/route
    merge_policy: append
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      You are the route specialist.
      Produce exact CRUD REST routes for a Go server.

  - name: u_backend
    spec: crud_backend
    depends_on: [u_contract, u_schema, u_routes]
    input_port: context
    output_port: api_summary
    output_kind: summary/api
    merge_policy: append
    handoff_max_chars: 260
    input_prefix: |
      [universe_hint]
      Implement the Go backend from contract, relational schema and route map.

  - name: u_infra
    spec: crud_infra
    depends_on: [u_schema, u_routes, u_backend]
    input_port: context
    output_port: infra_summary
    output_kind: summary/code
    merge_policy: append
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      Prepare Postgres, Docker Compose and local run UX for a fullstack CRUD.

  - name: u_frontend
    spec: crud_frontend
    depends_on: [u_backend, u_routes]
    input_port: context
    output_port: page
    output_kind: frontend/page
    merge_policy: append
    handoff_max_chars: 240
    input_prefix: |
      [universe_hint]
      Build the Next.js CRUD UI from backend API summary and route map.

  - name: u_test
    spec: crud_tester
    depends_on: [u_backend, u_routes, u_schema]
    input_port: context
    output_port: routes_tests
    output_kind: test/integration
    merge_policy: append
    handoff_max_chars: 220
    input_prefix: |
      [universe_hint]
      Produce backend validation, smoke test and route-focused checks.

  - name: u_synth
    spec: crud_synth
    depends_on: [u_contract, u_schema, u_routes, u_backend, u_infra, u_frontend, u_test]
    input_port: context
    output_port: result
    output_kind: plan/summary
    merge_policy: append
    handoff_max_chars: 260
    input_prefix: |
      [universe_hint]
      Reconcile the fullstack relational CRUD.
      Always include final routes and local startup commands.
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
