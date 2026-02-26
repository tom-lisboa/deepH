package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
)

const RootConfigTemplate = `version: 1
default_provider: local_mock
providers:
  - name: local_mock
    type: mock
    model: mock-small
    timeout_ms: 3000
`

const ExampleAgentTemplate = `name: guide
description: Example guide agent (stored in examples/, not loaded automatically)
provider: local_mock
model: mock-small
system_prompt: |
  You are a guide agent. Explain what will run and be explicit about assumptions.
skills:
  - echo
startup_calls:
  - skill: echo
    args:
      note: "hello from startup_calls"
`

const ExampleCrewTemplate = `name: reviewpack
description: Example crew with universe channels (baseline/strict -> synth)
spec: guide
universes:
  - name: baseline
    spec: guide
    output_kind: summary/text
  - name: strict
    spec: guide
    output_kind: diagnostic/test
    input_prefix: |
      [universe_hint]
      mode: strict
      be explicit about assumptions and tradeoffs.
  - name: synth
    spec: guide
    output_kind: plan/summary
    depends_on: [baseline, strict]
    merge_policy: append
    handoff_max_chars: 220
    input_prefix: |
      [universe_hint]
      Compare upstream universes and synthesize the best answer.
`

const ExampleCRUDCrewTemplate = `name: crud_fullstack_multiverse
description: CRUD fullstack with typed universe channels (contract -> backend -> frontend/test -> synth)
spec: crud_contract
# Requires agents such as:
#   agents/crud_contract.yaml
#   agents/crud_backend.yaml
#   agents/crud_frontend.yaml
#   agents/crud_tester.yaml
#   agents/crud_synth.yaml
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
`

func InitWorkspace(workspace string) error {
	dirs := []string{
		filepath.Join(workspace, "agents"),
		filepath.Join(workspace, "crews"),
		filepath.Join(workspace, "skills"),
		filepath.Join(workspace, "examples", "agents"),
		filepath.Join(workspace, "examples", "crews"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	rootPath := filepath.Join(workspace, "deeph.yaml")
	if err := writeIfMissing(rootPath, RootConfigTemplate); err != nil {
		return err
	}
	examplePath := filepath.Join(workspace, "examples", "agents", "guide.yaml")
	if err := writeIfMissing(examplePath, ExampleAgentTemplate); err != nil {
		return err
	}
	crewExamplePath := filepath.Join(workspace, "examples", "crews", "reviewpack.yaml")
	if err := writeIfMissing(crewExamplePath, ExampleCrewTemplate); err != nil {
		return err
	}
	crudCrewExamplePath := filepath.Join(workspace, "examples", "crews", "crud_fullstack_multiverse.yaml")
	if err := writeIfMissing(crudCrewExamplePath, ExampleCRUDCrewTemplate); err != nil {
		return err
	}
	return nil
}

func writeIfMissing(path, content string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
