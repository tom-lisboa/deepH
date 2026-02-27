export type CommandDoc = {
  path: string;
  category:
    | "meta"
    | "workspace"
    | "execution"
    | "agents"
    | "skills"
    | "providers"
    | "kits"
    | "types"
    | "crews"
    | "sessions"
    | "coach";
  summary: string;
  examples?: string[];
};

export type TypeItem = {
  kind: string;
  category: string;
  description: string;
};

export const projectPositioning = {
  title: "Typed agent runtime in Go, com foco em custo e orquestração.",
  subtitle:
    "deepH é um runtime leve para agentes definidos pelo usuário, com channels tipados, DAG, multiverse crews e integração DeepSeek-first."
};

export const valuePillars = [
  {
    title: "Baixo Token",
    body:
      "Context compiler com budgets, channels tipados, handoffs compactos e leitura por faixa reduzem replay de contexto.",
    tag: "token-aware"
  },
  {
    title: "Orquestração Séria",
    body:
      "DAG + depends_on_ports + channels + merge policy. O fluxo deixa de ser chat solto e vira pipeline observável.",
    tag: "dag_channels"
  },
  {
    title: "Agentes do Usuário",
    body:
      "O core não embute “agentes mágicos”. O usuário cria `agents/*.yaml`, define portas, tipos, tools e budgets.",
    tag: "user-owned"
  },
  {
    title: "Multiverso Prático",
    body:
      "Crews com universos, judge e agora channels entre universos permitem comparar e sintetizar estratégias sem duplicar tudo.",
    tag: "crews+judge"
  }
];

export const quickStartCommands = [
  "go run ./cmd/deeph init",
  "cp examples/agents/guide.yaml agents/guide.yaml",
  "go run ./cmd/deeph skill add echo",
  "go run ./cmd/deeph validate",
  'go run ./cmd/deeph trace guide "teste"',
  'go run ./cmd/deeph run guide "teste"'
];

export const commandDocs: CommandDoc[] = [
  { path: "help", category: "meta", summary: "Mostra uso da CLI e comandos disponíveis." },
  { path: "command list", category: "meta", summary: "Lista o dicionário de comandos (com `--json`)." },
  { path: "command explain", category: "meta", summary: "Explica um comando específico (com `--json`)." },
  { path: "init", category: "workspace", summary: "Inicializa workspace `deeph.yaml`, pastas e exemplos." },
  { path: "validate", category: "workspace", summary: "Valida root config, agents e skills YAML." },
  {
    path: "trace",
    category: "execution",
    summary: "Mostra plano de execução, stages, channels e handoffs. Suporta `--json` e `--multiverse`."
  },
  {
    path: "run",
    category: "execution",
    summary:
      "Executa agent(s) com orquestração DAG/channels. Suporta `--trace`, `--multiverse`, `--judge-agent` e coach."
  },
  {
    path: "chat",
    category: "execution",
    summary:
      "Sessão de conversa fluida no terminal com histórico persistente (`sessions/`) para 1 agent ou multi-agent spec."
  },
  { path: "session list", category: "sessions", summary: "Lista sessões de chat persistidas." },
  { path: "session show", category: "sessions", summary: "Mostra conteúdo/resumo de uma sessão salva." },
  {
    path: "crew list",
    category: "crews",
    summary: "Lista crews em `crews/` (aliases de spec e universos de multiverse)."
  },
  {
    path: "crew show",
    category: "crews",
    summary:
      "Mostra uma crew com universos, `depends_on`, ports, `output_kind`, merge e handoff chars."
  },
  { path: "agent create", category: "agents", summary: "Gera template de agent em `agents/`." },
  { path: "skill list", category: "skills", summary: "Lista templates de skills oficiais." },
  { path: "skill add", category: "skills", summary: "Instala skill template YAML em `skills/`." },
  { path: "provider list", category: "providers", summary: "Lista providers configurados no workspace." },
  {
    path: "provider add",
    category: "providers",
    summary: "Scaffold de provider (ex.: `deepseek`) em `deeph.yaml`, com `--set-default`."
  },
  {
    path: "kit list",
    category: "kits",
    summary: "Lista kits instaláveis por nome (starter bundles com agents/crews/skills)."
  },
  {
    path: "kit add",
    category: "kits",
    summary: "Instala kit por nome e aplica configurações necessárias (skills/agents/crews/provider)."
  },
  { path: "type list", category: "types", summary: "Lista tipos semânticos do runtime (com `--json`)." },
  { path: "type explain", category: "types", summary: "Explica um tipo/alias (`code/go`, `string`, `tools`)." },
  { path: "coach stats", category: "coach", summary: "Mostra aprendizado local do coach (com scope/kind/json)." },
  { path: "coach reset", category: "coach", summary: "Reseta total ou parcial o estado local do coach." }
];

export const allTypeKinds: TypeItem[] = [
  { kind: "artifact/blob", category: "artifact", description: "Blob opaco / binário." },
  { kind: "artifact/ref", category: "artifact", description: "Referência para artifact armazenado (preferido para payload grande)." },
  { kind: "artifact/summary", category: "artifact", description: "Resumo de artifact." },
  { kind: "capability/tools", category: "capability", description: "Capacidade/permissão de tools (não payload)." },
  { kind: "code/bash", category: "code", description: "Script shell." },
  { kind: "code/c", category: "code", description: "Código C." },
  { kind: "code/cpp", category: "code", description: "Código C++." },
  { kind: "code/go", category: "code", description: "Código Go." },
  { kind: "code/java", category: "code", description: "Código Java." },
  { kind: "code/js", category: "code", description: "Código JavaScript." },
  { kind: "code/jsx", category: "code", description: "Código JSX." },
  { kind: "code/python", category: "code", description: "Código Python." },
  { kind: "code/rust", category: "code", description: "Código Rust." },
  { kind: "code/sql", category: "code", description: "SQL." },
  { kind: "code/toml", category: "code", description: "Documento TOML." },
  { kind: "code/ts", category: "code", description: "Código TypeScript." },
  { kind: "code/tsx", category: "code", description: "Código TSX." },
  { kind: "code/yaml", category: "code", description: "Documento YAML." },
  { kind: "context/compiled", category: "context", description: "Janela de contexto compilada pelo runtime." },
  { kind: "data/csv", category: "data", description: "CSV." },
  { kind: "data/table", category: "data", description: "Tabela lógica (linhas/colunas)." },
  { kind: "contract/openapi", category: "contract", description: "Contrato OpenAPI." },
  { kind: "contract/json-schema", category: "contract", description: "Contrato JSON Schema." },
  { kind: "db/schema", category: "db", description: "Schema/modelo de banco." },
  { kind: "db/migration", category: "db", description: "Migração de banco." },
  { kind: "backend/route", category: "backend", description: "Camada de routes/handlers do backend." },
  { kind: "backend/controller", category: "backend", description: "Camada de controllers do backend." },
  { kind: "backend/service", category: "backend", description: "Camada de services/use-cases do backend." },
  { kind: "backend/repository", category: "backend", description: "Camada de repository/data access do backend." },
  { kind: "frontend/page", category: "frontend", description: "Página/tela de frontend." },
  { kind: "frontend/component", category: "frontend", description: "Componente de UI do frontend." },
  { kind: "frontend/form", category: "frontend", description: "Formulário/fluxo de entrada no frontend." },
  { kind: "frontend/client-api", category: "frontend", description: "Cliente de API no frontend." },
  { kind: "diagnostic/build", category: "diagnostic", description: "Diagnóstico de build/compile." },
  { kind: "diagnostic/lint", category: "diagnostic", description: "Diagnóstico de lint." },
  { kind: "diagnostic/test", category: "diagnostic", description: "Resultado/diagnóstico de testes." },
  { kind: "json/array", category: "json", description: "JSON array." },
  { kind: "json/object", category: "json", description: "JSON object." },
  { kind: "json/value", category: "json", description: "JSON value genérico." },
  { kind: "memory/fact", category: "memory", description: "Fato persistido." },
  { kind: "memory/question", category: "memory", description: "Pergunta aberta em memória." },
  { kind: "memory/summary", category: "memory", description: "Resumo persistido." },
  { kind: "message/agent", category: "message", description: "Mensagem agent -> agent." },
  { kind: "message/assistant", category: "message", description: "Mensagem de assistant/modelo." },
  { kind: "message/system", category: "message", description: "Mensagem de sistema." },
  { kind: "message/tool", category: "message", description: "Mensagem de tool." },
  { kind: "message/user", category: "message", description: "Mensagem de usuário." },
  { kind: "plan/summary", category: "plan", description: "Resumo de plano." },
  { kind: "plan/task", category: "plan", description: "Task/etapa de plano." },
  { kind: "primitive/bool", category: "primitive", description: "Booleano." },
  { kind: "primitive/float", category: "primitive", description: "Float." },
  { kind: "primitive/int", category: "primitive", description: "Inteiro." },
  { kind: "primitive/null", category: "primitive", description: "Null." },
  { kind: "primitive/number", category: "primitive", description: "Número genérico." },
  { kind: "primitive/string", category: "primitive", description: "String." },
  { kind: "summary/code", category: "summary", description: "Resumo compacto de código." },
  { kind: "summary/api", category: "summary", description: "Resumo compacto de contrato/endpoints de API." },
  { kind: "summary/text", category: "summary", description: "Resumo compacto de texto." },
  { kind: "test/unit", category: "test", description: "Teste unitário (código/plano)." },
  { kind: "test/integration", category: "test", description: "Teste de integração (código/plano)." },
  { kind: "test/e2e", category: "test", description: "Teste end-to-end (código/plano)." },
  { kind: "text/diff", category: "text", description: "Patch / diff." },
  { kind: "text/markdown", category: "text", description: "Markdown." },
  { kind: "text/path", category: "text", description: "Caminho de arquivo como texto." },
  { kind: "text/plain", category: "text", description: "Texto simples." },
  { kind: "text/prompt", category: "text", description: "Prompt/instrução." },
  { kind: "tool/error", category: "tool", description: "Erro de tool estruturado." },
  { kind: "tool/result", category: "tool", description: "Resultado de tool estruturado." }
];

export const bestPractices = [
  "Rode `deeph validate` antes de `run` em qualquer mudança de YAML.",
  "Use `deeph trace` (ou `trace --json`) para inspecionar stages, handoffs, channels e budgets antes de workflows maiores.",
  "Tipifique `io.inputs` e `io.outputs` dos agents; evite deixar tudo como texto genérico.",
  "Use `depends_on_ports` para handoff por porta em vez de broadcasting entre agents.",
  "Prefira `summary/*` + `artifact/ref` em handoffs, não texto bruto grande.",
  "Use `file_read_range` antes de `file_read` quando a tarefa for leitura de código/arquivo longo.",
  "Aplique `max_tokens`, `merge_policy` e `channel_priority` nas portas que recebem muitos handoffs.",
  "Defina `context_max_input_tokens`, `context_max_channels` e `context_max_channel_tokens` por agent em workflows grandes.",
  "Aplique `publish_max_channels` e `publish_max_channel_tokens` para conter fan-out de handoffs.",
  "Defina `tool_max_calls` / `tool_max_exec_ms` por agent e `stage_tool_max_*` em stages paralelos.",
  "Ative `cache_http_get_tools` apenas quando GET puder ser cacheado com segurança.",
  "Mantenha `DEEPSEEK_API_KEY` em variável de ambiente; não versionar chave no YAML.",
  "Use `chat` para trabalho iterativo e `run` para pipelines determinísticos/repetíveis.",
  "Use crews e multiverso para comparar estratégias; use `judge-agent` para reconciliar resultados."
];

export const docsConcepts = [
  {
    title: "Agents (YAML do usuário)",
    body:
      "Cada agent declara provider, prompt, skills, metadata e portas tipadas. O runtime conhece contratos, não personalidades embutidas."
  },
  {
    title: "Skills (opcionais)",
    body:
      "Skills são executores locais instaláveis (`skill add`) e controladas por permissões/budgets. O catálogo pode crescer sem fechar o core."
  },
  {
    title: "DAG + Channels + Ports",
    body:
      "O plano usa stages e dependências explícitas. Handoffs são porta->porta, com `merge_policy`, `channel_priority` e budgets por canal."
  },
  {
    title: "Type System semântico",
    body:
      "Tipos como `code/go`, `summary/text`, `diagnostic/test` orientam context compiler, merge, prioridade e handoffs — inclusive no multiverso."
  },
  {
    title: "Context Compiler",
    body:
      "Compila a janela por `type + moment` (`tool_loop`, `synthesis`, etc.), com scoring, budgets e drops rastreáveis no `run`."
  },
  {
    title: "Crews + Multiverse",
    body:
      "Crews definem universos e variações. Agora universos também podem formar DAG e compartilhar handoffs tipados (`u1.result -> u3.context`)."
  }
];

export const architectureHighlights = [
  "Go runtime leve (core sem framework Python pesado).",
  "Providers abstratos, com DeepSeek real e OpenAI/Anthropic/Ollama preparados para adapters.",
  "Chat completions + tool calls DeepSeek controlados pelo runtime (não delega orquestração).",
  "Tool broker compartilhado por execução (cache + coalescing + locks por recurso).",
  "Budgets por agent, stage e channel; anti-loop em tool loop e publish.",
  "Coach local sem LLM para onboarding progressivo no terminal."
];

export const customizationGuide = {
  title: "Como o usuário cria agents e skills no deepH",
  summary:
    "No deepH, agent é configuração do usuário (YAML). Skill pode ser: (1) instância configurada em YAML de um tipo suportado, ou (2) novo tipo de skill implementado no core em Go.",
  agentSteps: [
    {
      title: "Criar o arquivo base do agent",
      text: "Use `agent create` para gerar `agents/<nome>.yaml` com provider/model e template de IO.",
      code: `deeph agent create calc_backend --provider deepseek --model deepseek-chat`
    },
    {
      title: "Definir prompt e contrato",
      text: "Edite `system_prompt`, `skills`, `io.inputs`, `io.outputs`, timeouts e metadata (context/tool budgets).",
      code: `# agents/calc_backend.yaml (trecho)
system_prompt: |
  Build the backend API route and controller for a Next.js calculator.
skills:
  - file_read_range
  - file_write_safe
io:
  inputs:
    - name: plan_input
      accepts: [plan/summary, json/object]
  outputs:
    - name: backend_patch
      produces: [text/diff, summary/code]`
    },
    {
      title: "Validar e testar o fluxo",
      text: "Use `validate`, `trace`, `run` e `chat` para iterar no comportamento do agent.",
      code: `deeph validate
deeph trace calc_backend "crie route.ts e controller.ts"
deeph chat calc_backend`
    }
  ],
  skillSteps: [
    {
      title: "Skill configurada (YAML) — mais comum",
      text: "Crie `skills/*.yaml` apontando para um tipo já suportado (`file_read`, `file_read_range`, `file_write_safe`, `http`, `echo`).",
      code: `# skills/write_code.yaml
name: write_code
type: file_write_safe
description: Writes source files safely inside the workspace
params:
  max_bytes: 131072
  create_dirs: true
  create_if_missing: true
  overwrite_default: false`
    },
    {
      title: "Usar a skill no agent",
      text: "Adicione o nome da skill em `skills:` no YAML do agent. O deepH expõe a skill para tool calling da DeepSeek automaticamente.",
      code: `# agents/calc_frontend.yaml (trecho)
skills:
  - file_read_range
  - write_code`
    },
    {
      title: "Skill nova (tipo novo em Go) — avançado",
      text: "Se precisar de comportamento novo (ex.: `npm_install`, `run_tests`), implemente no core Go e registre no runtime/catalog/validate.",
      code: `# pontos a editar (resumo)
internal/runtime/skills.go        # implementar Execute()
internal/runtime/engine.go        # schema da tool
internal/project/validate.go      # liberar type
internal/catalog/skills.go        # template opcional`
    }
  ]
};

export const projectUsageGuide = {
  title: "Como usar o deepH dentro de um projeto real (ex.: hello-world)",
  summary:
    "As skills de arquivo do deepH são limitadas ao `workspace`. Então, para analisar/gerar código em `hello-world`, esse projeto precisa estar dentro do workspace (normalmente o próprio root do projeto).",
  modes: [
    {
      title: "Modo A (recomendado): deepH dentro do repo alvo",
      body:
        "Inicialize o deepH no root do projeto `hello-world`. Agents e skills ficam versionados junto com o código, e `file_read_range` / `file_write_safe` conseguem atuar em todos os arquivos do repo.",
      code: `cd ~/projetos/hello-world
# com binário instalado (recomendado)
deeph init
deeph provider add deepseek --set-default
deeph skill add file_write_safe
deeph skill add file_read_range
deeph agent create planner --provider deepseek --model deepseek-chat
deeph validate

# analisar código da pasta hello-world
deeph trace planner "analise a estrutura do projeto e proponha melhorias"

# gerar código dentro da própria pasta hello-world
deeph run "calc_planner>calc_backend+calc_frontend>calc_reviewer" "crie uma calculadora Next.js simples"`
    },
    {
      title: "Modo B: workspace pai contendo vários projetos",
      body:
        "Crie um workspace deepH no diretório pai e mantenha projetos como subpastas. As skills continuam limitadas ao workspace, então `hello-world/` pode ser analisado/alterado se estiver dentro desse pai.",
      code: `mkdir ~/labs/deeph-workspace
cd ~/labs/deeph-workspace
deeph init

# estrutura
# ~/labs/deeph-workspace/
#   deeph.yaml
#   agents/
#   skills/
#   hello-world/

deeph run planner "analise os códigos em hello-world/ e proponha refatorações"
deeph run backend_builder "crie hello-world/app/api/calc/route.ts e controller"`
    }
  ],
  examples: [
    {
      title: "Analisar código da pasta hello-world",
      code: `deeph trace reader "use file_read_range para ler hello-world/app/page.tsx e resumir"
deeph run reader "use file_read_range para ler hello-world/app/page.tsx e resumir"`
    },
    {
      title: "Construir código dentro da pasta hello-world",
      code: `deeph run codegen "crie hello-world/lib/math/evaluator.ts com parse/eval seguro"
# com file_write_safe habilitada, a skill grava no arquivo`
    }
  ]
};

export const deepseekToolingResearch = {
  title: "Pesquisa rápida: DeepSeek tools vs skills do deepH",
  findings: [
    "DeepSeek Chat Completions suporta `tools` e `tool_choice` (`none`, `auto`, `required`) no endpoint `/chat/completions`.",
    "A resposta pode vir com `finish_reason=tool_calls` e `message.tool_calls[].function.arguments` (string JSON).",
    "A própria docs reforça que a função precisa ser executada pelo usuário/aplicação; o modelo não executa a tool sozinho.",
    "No deepH, isso mapeia para: DeepSeek tool call -> execução de skill local (`file_read_range`, `file_write_safe`, `http`, etc.).",
    "Logo, para gerar código real em projeto, o usuário deve habilitar skills de filesystem; só tool calling da DeepSeek não escreve arquivo por conta própria."
  ],
  sources: [
    {
      label: "DeepSeek Tool Calls Guide (official)",
      href: "https://api-docs.deepseek.com/guides/tool_calls"
    },
    {
      label: "DeepSeek Create Chat Completion (official)",
      href: "https://api-docs.deepseek.com/api/create-chat-completion"
    }
  ]
};

export const kitsGuide = {
  title: "Starter Kits: instalar por nome (ou Git URL)",
  summary:
    "Kits são bundles de produtividade que instalam skills + agents + crews e, quando necessário, configuram provider. O objetivo é zero fricção: digite o nome e rode.",
  quickCommands: `# listar kits locais embutidos
deeph kit list

# instalar kit por nome
deeph kit add hello-next-tailwind
deeph kit add hello-next-shadcn
deeph kit add crud-next-multiverse

# instalar kit remoto por git URL
deeph kit add https://github.com/acme/deeph-kits.git

# escolher manifesto específico dentro do repo
deeph kit add https://github.com/acme/deeph-kits.git#kits/next/kit.yaml`,
  behavior: [
    "Instala automaticamente as skills requeridas do catálogo local (`skill add` implícito).",
    "Escreve templates de `agents/*.yaml` e `crews/*.yaml` no workspace.",
    "Para kits DeepSeek-first, scaffolda provider `deepseek` por padrão (pode desligar com `--skip-provider`).",
    "Preserva arquivos existentes por padrão; use `--force` para sobrescrever mudanças.",
    "Valida o projeto após instalação e mostra próximos passos (`validate`, `crew list`, `run`)."
  ],
  manifest: `# deeph-kit.yaml (ou kit.yaml)
name: next-crud-kit
description: Next CRUD starter
provider_type: deepseek
required_skills:
  - file_read_range
  - file_write_safe
  - echo

files:
  # carregar conteúdo de arquivo do próprio repo do kit
  - path: agents/crud_backend.yaml
    source: templates/agents/crud_backend.yaml

  # ou inline direto no manifesto
  - path: crews/crud_fullstack_multiverse.yaml
    content: |
      name: crud_fullstack_multiverse
      spec: crud_contract`,
  notes: [
    "Modo Git espera `deeph-kit.yaml` ou `kit.yaml` no root do repo.",
    "Use `#path/do/manifest.yaml` quando o manifesto estiver em subpasta.",
    "O parser bloqueia path traversal em `source` e mantém leitura dentro do repo clonado."
  ]
};

export const calculatorTutorial = {
  title: "Step by step: calculadora no deepH",
  premise:
    "Este tutorial mostra como usar o deepH para ORQUESTRAR a criação de uma calculadora fullstack em Next.js: frontend (UI), backend (API route/controller) e integração. O foco é onde entram os prompts, como usar skills e como a DeepSeek participa via tool calling.",
  steps: [
    {
      title: "1. Inicialize o workspace",
      body:
        "Crie o workspace do deepH e instale as skills mínimas. `echo` ajuda no debug e `file_write_safe` permite geração real de arquivos no app Next alvo.",
      code: `go run ./cmd/deeph init
go run ./cmd/deeph skill add echo
go run ./cmd/deeph skill add file_write_safe
go run ./cmd/deeph validate`
    },
    {
      title: "2. Configure provider (mock para desenho de fluxo, DeepSeek para geração real)",
      body:
        "Para validar a orquestração sem custo, use `local_mock`. Para gerar código de verdade, use DeepSeek via `chat completions` + `tool calls` (mapeados para skills do deepH).",
      code: `# opção rápida (mock já vem no init)
go run ./cmd/deeph provider list

# opção DeepSeek (scaffold)
go run ./cmd/deeph provider add deepseek --set-default
export DEEPSEEK_API_KEY="sua_chave"`
    },
    {
      title: "3. Estrutura do app alvo (Next)",
      body:
        "Crie (ou tenha) um app Next alvo onde os agents vão trabalhar. O deepH orquestra a geração/edição dos arquivos; o projeto final é a calculadora em Next.",
      code: `# exemplo (fora do deepH) para criar o app alvo
npx create-next-app@latest calc-app --ts --app --eslint

# estrutura alvo esperada (simplificada)
calc-app/
  app/
    page.tsx
    api/calc/route.ts
  lib/calc/
    controller.ts
    evaluator.ts`
    },
    {
      title: "4. Crie os agents da pipeline de geração (planner -> backend -> frontend -> reviewer)",
      body:
        "Vamos criar agents especializados. A ideia é o planner definir contrato da API, backend gerar `route/controller`, frontend gerar UI e reviewer validar integração.",
      code: `go run ./cmd/deeph agent create calc_planner
go run ./cmd/deeph agent create calc_backend
go run ./cmd/deeph agent create calc_frontend
go run ./cmd/deeph agent create calc_reviewer`
    },
    {
      title: "5. Onde a pessoa coloca os prompts dos agents",
      body:
        "Os prompts principais ficam em `agents/*.yaml` no campo `system_prompt`. Variações de universo (multiverso) entram em `crews/*.yaml` com `input_prefix`/`input_suffix`.",
      code: `# agents/calc_planner.yaml
name: calc_planner
provider: deepseek
model: deepseek-chat
system_prompt: |
  You are the planner for a Next.js calculator app.
  Define:
  - API contract (request/response)
  - file plan
  - responsibilities for backend and frontend agents
  Be explicit about assumptions and keep outputs compact.
skills:
  - echo
io:
  inputs:
    - name: task
      accepts: [text/plain]
      required: true
      max_tokens: 220
  outputs:
    - name: plan
      produces: [plan/summary, json/object]
metadata:
  context_moment: "discovery"
  context_max_input_tokens: "900"

# crews/calc_next.yaml (variação de universo)
name: calc_next
spec: "calc_planner>calc_backend+calc_frontend>calc_reviewer"
universes:
  - name: strict
    spec: "calc_planner>calc_backend+calc_frontend>calc_reviewer"
    input_prefix: |
      [universe_hint]
      Enforce input validation and error handling.`
    },
    {
      title: "6. Defina o agent de backend (API route + controller)",
      body:
        "O backend consome o plano e gera arquivos de API. Se você quiser criar arquivos automaticamente, ele deve usar skills de filesystem (leitura e escrita).",
      code: `# agents/calc_backend.yaml
name: calc_backend
provider: deepseek
model: deepseek-chat
system_prompt: |
  Build the backend for a Next.js calculator.
  Create:
  - app/api/calc/route.ts
  - lib/calc/controller.ts
  - lib/calc/evaluator.ts
  Requirements:
  - support +, -, *, /
  - validate invalid expressions
  - return JSON { ok, result, error? }
skills:
  - file_read_range
  - file_write_safe
  - echo
depends_on: [calc_planner]
depends_on_ports:
  plan_input: [calc_planner.plan]
io:
  inputs:
    - name: plan_input
      accepts: [plan/summary, json/object, text/plain]
      required: true
      merge_policy: latest
      channel_priority: 4
      max_tokens: 300
  outputs:
    - name: backend_patch
      produces: [text/diff, summary/code]
metadata:
  context_moment: "tool_loop"
  max_tool_rounds: "4"
  tool_max_calls: "8"`
    },
    {
      title: "7. Defina o agent de frontend (UI em Next)",
      body:
        "O frontend consome o plano (e opcionalmente a saída do backend) para criar uma UI simples com input, botão e resultado.",
      code: `# agents/calc_frontend.yaml
name: calc_frontend
provider: deepseek
model: deepseek-chat
system_prompt: |
  Build a simple Next.js calculator UI in app/page.tsx.
  Requirements:
  - expression input
  - submit button
  - result panel
  - error state
  - fetch POST /api/calc
skills:
  - file_read_range
  - file_write_safe
  - echo
depends_on: [calc_planner]
depends_on_ports:
  plan_input: [calc_planner.plan]
io:
  inputs:
    - name: plan_input
      accepts: [plan/summary, json/object, text/plain]
      required: true
      merge_policy: latest
      channel_priority: 5
      max_tokens: 260
  outputs:
    - name: frontend_patch
      produces: [text/diff, summary/code]
metadata:
  context_moment: "tool_loop"
  max_tool_rounds: "4"`
    },
    {
      title: "8. Defina o reviewer/integrator (conecta backend + frontend)",
      body:
        "O reviewer consome patches/resumos dos dois lados e verifica consistência (payload, rota, UX, erros). Ele pode sugerir correções ou consolidar um patch final.",
      code: `# agents/calc_reviewer.yaml
name: calc_reviewer
provider: deepseek
model: deepseek-chat
system_prompt: |
  Review and integrate Next.js calculator backend + frontend.
  Check API payload contract, fetch call compatibility, error handling and UX clarity.
  Produce a compact review and final patch suggestions.
skills:
  - echo
depends_on: [calc_backend, calc_frontend]
depends_on_ports:
  backend: [calc_backend.backend_patch]
  frontend: [calc_frontend.frontend_patch]
io:
  inputs:
    - name: backend
      accepts: [text/diff, summary/code, text/plain]
      required: true
      merge_policy: latest
      channel_priority: 4
      max_tokens: 260
    - name: frontend
      accepts: [text/diff, summary/code, text/plain]
      required: true
      merge_policy: latest
      channel_priority: 4
      max_tokens: 260
  outputs:
    - name: review
      produces: [summary/text, text/markdown]
metadata:
  context_moment: "validate"`
    },
    {
      title: "9. Trace e run da pipeline fullstack",
      body:
        "Use `trace` para ver handoffs/ports e depois `run` para executar o fluxo de geração. No `chat`, você itera refinando prompts e saídas.",
      code: `go run ./cmd/deeph validate
go run ./cmd/deeph trace "calc_planner>calc_backend+calc_frontend>calc_reviewer" "crie uma calculadora Next.js simples"
go run ./cmd/deeph run "calc_planner>calc_backend+calc_frontend>calc_reviewer" "crie uma calculadora Next.js simples"

# modo iterativo
go run ./cmd/deeph chat "calc_planner>calc_backend+calc_frontend>calc_reviewer"`
    },
    {
      title: "10. (Opcional) Multiverso para comparar implementações",
      body:
        "Rode múltiplos universos (baseline/strict/fast) e use `judge-agent` para escolher a melhor implementação. Universos podem trocar handoffs entre si via `depends_on`.",
      code: `# crews/calcpack.yaml (resumo)
name: calcpack
spec: "calc_planner>calc_backend+calc_frontend>calc_reviewer"
universes:
  - name: baseline
    spec: "calc_planner>calc_backend+calc_frontend>calc_reviewer"
    output_kind: summary/text
  - name: strict
    spec: "calc_planner>calc_backend+calc_frontend>calc_reviewer"
    output_kind: diagnostic/test
    input_prefix: |
      Enforce stricter validation, safer parsing and explicit error states.
  - name: synth
    spec: "calc_planner>calc_backend+calc_frontend>calc_reviewer"
    output_kind: plan/summary
    depends_on: [baseline, strict]
    merge_policy: append
    handoff_max_chars: 220

go run ./cmd/deeph trace --multiverse 0 @calcpack "crie uma calculadora Next.js simples"
go run ./cmd/deeph run --multiverse 0 --judge-agent guide @calcpack "crie uma calculadora Next.js simples"`
    },
    {
      title: "11. Pode usar tool da DeepSeek? Sim, mas via skills do deepH",
      body:
        "A DeepSeek suporta tool/function calling, mas as ferramentas são definidas por você e executadas pela sua aplicação. No deepH, isso significa: model -> pede tool -> deepH executa skill local. Para geração de app, skills são a ponte certa (read/write/list/test).",
      code: `# visão prática no deepH
# DeepSeek tool call  --->  deepH skill execution
# "read_file"         --->  file_read / file_read_range
# "write_file"        --->  file_write_safe
# "run_tests"         --->  shell_gated or test_runner (skill custom)

# prompt dos agents continua em agents/*.yaml (system_prompt)
# tools disponíveis continuam em skills: [...]`
    },
    {
      title: "12. Resultado final esperado (qual seria)",
      body:
        "No final da pipeline, o reviewer deve entregar patch/resumo e os arquivos da calculadora Next criados/ajustados. A UI faz POST para a API e exibe resultado/erro.",
      code: `# resultado esperado (exemplo)
calc-app/
  app/
    page.tsx                # formulário + fetch para /api/calc
    api/
      calc/
        route.ts            # POST { expression } -> { ok, result | error }
  lib/
    calc/
      controller.ts         # valida request e chama evaluator
      evaluator.ts          # parse/eval de + - * /

# comportamento final da UI
Input: "12*(7+5)-9"
Click: "Calcular"
Output: "135"
Error example: "Expressão inválida"`
    }
  ],
  notes: [
    "Prompts dos agents: `agents/*.yaml` em `system_prompt`. Variações de universo: `crews/*.yaml` com `input_prefix`/`input_suffix`.",
    "DeepSeek tools no deepH = tool/function calling da DeepSeek mapeado para skills locais do runtime. A DeepSeek não executa seu filesystem diretamente.",
    "Para gerar arquivos de verdade, habilite skills de filesystem (especialmente `file_write_safe`). Sem skill de escrita, o agent só descreve/gera patch textual.",
    "Com `local_mock`, você valida orquestração (trace/handoffs/channels). Para geração real, troque para DeepSeek e habilite skills apropriadas."
  ]
};

export const crudMultiverseCrewGuide = {
  title: "Crew CRUD Fullstack com universos colaborando por channels tipados",
  summary:
    "Use multiverso para explorar arquitetura e implementação por camadas (contract, backend, frontend, tests, synth) com handoffs compactos e tipados entre universos.",
  crewYaml: `# crews/crud_fullstack_multiverse.yaml
name: crud_fullstack_multiverse
description: CRUD fullstack com universos colaborando por contrato, backend, frontend, testes e síntese
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
      Implement backend CRUD from the upstream OpenAPI contract.

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
      Build UI from backend API summary. Prefer artifact refs + summaries.

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
      Generate route-focused tests and validation checklist for backend CRUD.

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
      Reconcile contract, backend, frontend and tests into one implementation plan.`,
  channels: [
    "u_contract.openapi -> u_backend.context#contract/openapi",
    "u_backend.api_summary -> u_frontend.context#summary/api",
    "u_backend.api_summary -> u_test.context#summary/api",
    "u_frontend.page -> u_synth.context#frontend/page",
    "u_test.routes_tests -> u_synth.context#backend/route",
    "u_contract.openapi -> u_synth.context#contract/openapi"
  ],
  weightsExample: `# agents/crud_backend.yaml (trecho)
metadata:
  context_moment: "backend_codegen"
  # proposta de pesos por tipo (para uma futura/extended policy)
  type_weights:
    contract/openapi: "6"
    db/schema: "5"
    db/migration: "4"
    backend/route: "5"
    backend/controller: "5"
    backend/service: "4"
    summary/api: "4"
    summary/code: "3"
    text/prompt: "1"`,
  notes: [
    "Crie os agents correspondentes (`crud_contract`, `crud_backend`, `crud_frontend`, `crud_tester`, `crud_synth`) antes de rodar o crew.",
    "Branch cedo, converge tarde: explore arquitetura e camada, não cada arquivo individual.",
    "Use payload compacto no handoff (`summary/*` + `artifact/ref`) e só releia código bruto quando necessário.",
    "Separe portas semânticas (`openapi`, `api_summary`, `page`, `routes_tests`) em vez de usar sempre `result`."
  ]
};

export const comparisonMeta = {
  title: "deepH vs Claude Code (comparativo prático)",
  date: "26 de fevereiro de 2026",
  disclaimer:
    "Comparativo orientado a arquitetura e controle do usuário. Claude Code evolui rápido; trate isso como snapshot baseado em docs públicas oficiais."
};

export const comparisonRows = [
  {
    topic: "Objetivo principal",
    deeph:
      "Runtime tipado de agentes em Go para o usuário montar workflows, ports/channels, multiverso e orquestração sob custo controlado.",
    claude:
      "Agente de coding no terminal focado em fluxo de desenvolvimento com Claude, experiência pronta e forte integração de uso diário."
  },
  {
    topic: "Posse dos agentes",
    deeph:
      "Usuário cria `agents/*.yaml` e `crews/*.yaml`; o core não depende de agentes embutidos.",
    claude:
      "Experiência vem pronta com o produto; personalização ocorre mais por prompts/config e fluxo de uso do CLI."
  },
  {
    topic: "Tipagem semântica / ports",
    deeph:
      "Nativo: `code/*`, `summary/*`, `diagnostic/*`, `message/*`, ports e merge policies por input.",
    claude:
      "Não é o foco principal da UX pública; a abstração principal é o agente de coding, não um runtime tipado de workflows."
  },
  {
    topic: "Orquestração multi-agent",
    deeph:
      "DAG/channels/depends_on_ports, handoffs tipados, budgets por channel e multiverso com channels entre universos.",
    claude:
      "Foco maior em produtividade de coding assistido; não se apresenta como framework de orquestração aberto de multi-agent DAG."
  },
  {
    topic: "Custo/token tuning",
    deeph:
      "Design explícito para custo: context compiler, `type + moment weighting`, budgets, `file_read_range`, publish/channel budgets.",
    claude:
      "Otimizações existem no produto, mas tuning detalhado de orquestração/token não é a proposta central para o usuário final."
  },
  {
    topic: "Experiência pronta / time-to-value",
    deeph:
      "Exige modelagem (YAML, agents, ports, crews). Melhor para quem quer controle e arquitetura.",
    claude:
      "Excelente para começar rápido no terminal com um agente de coding já pronto."
  },
  {
    topic: "Quando usar",
    deeph:
      "Quando você quer construir um runtime/agente platform próprio, experimentar workflows e controlar custo/orquestração.",
    claude:
      "Quando você quer produtividade imediata de coding com UX pronta, forte assistente de terminal e pouca configuração inicial."
  }
];

export const comparisonUseCases = {
  chooseDeeph: [
    "Você quer uma plataforma de agentes, não só um CLI de coding.",
    "Você precisa de DAG/channels/ports e handoffs tipados entre agentes.",
    "Você quer trabalhar agressivamente custo/token e observabilidade.",
    "Você quer multiverso/crews e depois plugar merge/judge custom."
  ],
  chooseClaudeCode: [
    "Você quer produtividade de coding imediata, sem modelar runtime/agents YAML.",
    "Seu foco é execução de tarefas de desenvolvimento no terminal com UX pronta.",
    "Você não precisa (agora) de orquestração multi-agent explícita com ports/channels."
  ],
  together: [
    "Use Claude Code para fluxo pessoal de coding e prototipagem rápida.",
    "Use deepH para orquestrar pipelines reprodutíveis, agents do time e experimentos multiverso."
  ]
};

export const claudeCodeSources = [
  {
    label: "Anthropic Docs — Claude Code overview",
    href: "https://docs.anthropic.com/en/docs/claude-code/overview"
  },
  {
    label: "Anthropic Docs — Claude Code CLI reference",
    href: "https://docs.anthropic.com/en/docs/claude-code/cli-reference"
  }
];

export function groupCommandsByCategory() {
  const grouped = new Map<string, CommandDoc[]>();
  for (const item of commandDocs) {
    const list = grouped.get(item.category) ?? [];
    list.push(item);
    grouped.set(item.category, list);
  }
  return Array.from(grouped.entries()).map(([category, items]) => ({
    category,
    items: items.sort((a, b) => a.path.localeCompare(b.path))
  }));
}

export function groupTypesByCategory() {
  const grouped = new Map<string, TypeItem[]>();
  for (const item of allTypeKinds) {
    const list = grouped.get(item.category) ?? [];
    list.push(item);
    grouped.set(item.category, list);
  }
  return Array.from(grouped.entries())
    .map(([category, items]) => ({
      category,
      items: items.sort((a, b) => a.kind.localeCompare(b.kind))
    }))
    .sort((a, b) => a.category.localeCompare(b.category));
}

export const docsSidebarItems = [
  { href: "/docs#overview", label: "Visão Geral" },
  { href: "/docs#quickstart", label: "Quick Start" },
  { href: "/docs#kits", label: "Starter Kits" },
  { href: "/docs#conceitos", label: "Conceitos" },
  { href: "/docs#arquitetura", label: "Arquitetura" },
  { href: "/docs/hello-worlds", label: "Hello World Lab" },
  { href: "/docs#customizacao", label: "Criar Agents/Skills" },
  { href: "/docs#uso-em-projetos", label: "Usar em hello-world" },
  { href: "/docs#multiverso-codigo", label: "Multiverso para Codegen CRUD" },
  { href: "/docs/universos", label: "Universos (Docs Dedicada)" },
  { href: "/docs#deepseek-tools", label: "DeepSeek Tools vs Skills" },
  { href: "/docs#comandos", label: "Comandos" },
  { href: "/docs#tipos", label: "Tipos" },
  { href: "/docs#boas-praticas", label: "Boas Práticas" },
  { href: "/docs/calculadora", label: "Tutorial: Calculadora" },
  { href: "/docs/comparativo-claude-code", label: "Comparativo Claude Code" }
];
