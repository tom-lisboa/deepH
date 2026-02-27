import Link from "next/link";
import { CodeBlock } from "@/components/code-block";

const universeChannels = [
  {
    path: "u_offer.offer_plan -> u_prospect.context",
    type: "plan/summary",
    weight: "6",
    moment: "discovery"
  },
  {
    path: "u_prospect.leads -> u_sdr.context",
    type: "data/table",
    weight: "6",
    moment: "outreach"
  },
  {
    path: "u_sdr.qualified_leads -> u_closer.context",
    type: "summary/text",
    weight: "5",
    moment: "closing"
  },
  {
    path: "u_sdr.qualified_leads -> u_revops.context",
    type: "summary/text",
    weight: "4",
    moment: "synthesis"
  },
  {
    path: "u_closer.deals -> u_revops.context",
    type: "plan/summary",
    weight: "6",
    moment: "synthesis"
  },
  {
    path: "u_offer.offer_plan -> u_revops.context",
    type: "plan/summary",
    weight: "5",
    moment: "synthesis"
  }
];

const universeBuildSteps = [
  {
    title: "1. Defina os papéis da operação comercial",
    text:
      "Crie especialistas para oferta (Hormozi-style), prospecção, SDR, closer e revops. Esse time é o core do funil.",
    code: `go run ./cmd/deeph agent create sales_offer --provider deepseek --model deepseek-chat
go run ./cmd/deeph agent create sales_prospect --provider deepseek --model deepseek-chat
go run ./cmd/deeph agent create sales_sdr --provider deepseek --model deepseek-chat
go run ./cmd/deeph agent create sales_closer --provider deepseek --model deepseek-chat
go run ./cmd/deeph agent create sales_revops --provider deepseek --model deepseek-chat`
  },
  {
    title: "2. Instale o kit de Whats e skills de operação",
    text:
      "Use o kit `watsmeouw` para a integração com Whats API não oficial e complete com skills de leitura/escrita para CRM local.",
    code: `# se o kit estiver no catalogo local
go run ./cmd/deeph kit add watsmeouw

# fallback por Git (troque pela URL real do seu kit)
go run ./cmd/deeph kit add https://github.com/seu-org/deeph-kit-watsmeouw.git#kit.yaml

go run ./cmd/deeph skill add echo
go run ./cmd/deeph skill add file_read_range
go run ./cmd/deeph skill add file_write_safe
go run ./cmd/deeph skill add http_request

# confira os nomes de skills do kit
ls skills | rg watsmeouw`
  },
  {
    title: "3. Modele a crew de vendas como DAG tipada",
    text:
      "No multiverso operacional, cada universo corresponde a um papel e depende apenas do necessário.",
    code: `# crews/sales_team_multiverse.yaml
name: sales_team_multiverse
description: Time comercial de alto escalao com oferta, prospeccao, SDR, closer e revops
spec: sales_offer
universes:
  - name: u_offer
    spec: sales_offer
    output_port: offer_plan
    output_kind: plan/summary

  - name: u_prospect
    spec: sales_prospect
    depends_on: [u_offer]
    input_port: context
    output_port: leads
    output_kind: data/table

  - name: u_sdr
    spec: sales_sdr
    depends_on: [u_prospect]
    input_port: context
    output_port: qualified_leads
    output_kind: summary/text

  - name: u_closer
    spec: sales_closer
    depends_on: [u_sdr]
    input_port: context
    output_port: deals
    output_kind: plan/summary

  - name: u_revops
    spec: sales_revops
    depends_on: [u_offer, u_prospect, u_sdr, u_closer]
    input_port: context
    output_port: revenue_board
    output_kind: plan/summary
    merge_policy: append
    handoff_max_chars: 280`
  },
  {
    title: "4. Use ports semanticas para handoff enxuto",
    text:
      "Defina portas claras (`offer_plan`, `leads`, `qualified_leads`, `deals`) para reduzir ruído e token waste.",
    code: `# exemplo de IO tipado (sales_sdr)
io:
  inputs:
    - name: context
      accepts: [data/table, plan/summary, summary/text]
      required: true
      merge_policy: latest
      max_tokens: 280
  outputs:
    - name: qualified_leads
      produces: [summary/text, data/table]`
  },
  {
    title: "5. Ajuste contexto por momento da fase comercial",
    text:
      "Cada agente prioriza tipos diferentes por fase: discovery, outreach, closing e synthesis.",
    code: `# agents/sales_sdr.yaml (trecho)
metadata:
  context_moment: "tool_loop"
  context_max_input_tokens: "1000"
  max_tool_rounds: "5"
  tool_max_calls: "10"

# agents/sales_closer.yaml (trecho)
metadata:
  context_moment: "synthesis"
  context_max_input_tokens: "900"`
  },
  {
    title: "6. Trace primeiro, run depois",
    text:
      "Valide DAG/channels antes de execução real do funil para evitar erro em cadeia.",
    code: `go run ./cmd/deeph validate
go run ./cmd/deeph crew show sales_team_multiverse
go run ./cmd/deeph trace --multiverse 0 @sales_team_multiverse "oferta high ticket para consultoria B2B"`
  },
  {
    title: "7. Feche com judge para reconciliar narrativa final",
    text:
      "Rode com `judge-agent` para consolidar plano final de receita, riscos e próximos passos comerciais.",
    code: `go run ./cmd/deeph run --multiverse 0 --judge-agent guide @sales_team_multiverse "oferta high ticket para consultoria B2B"`
  }
];

const funnelSteps = [
  {
    title: "1. Setup inicial do workspace comercial",
    body:
      "Prepare o projeto com provider real e valide antes de modelar o time de vendas.",
    code: `go run ./cmd/deeph init
go run ./cmd/deeph provider add deepseek --set-default
export DEEPSEEK_API_KEY="sua_chave"
go run ./cmd/deeph validate`
  },
  {
    title: "2. Conectar Whats via kit watsmeouw",
    body:
      "A integração de Whats entra como skill/tool. O kit remoto instala arquivos de agent/skill para envio e leitura de mensagens.",
    code: `# se estiver no catalogo do workspace
go run ./cmd/deeph kit add watsmeouw

# fallback por Git (use a URL real do seu kit)
go run ./cmd/deeph kit add https://github.com/seu-org/deeph-kit-watsmeouw.git#kit.yaml

# confirmar instalacao
go run ./cmd/deeph skill list
ls skills | rg watsmeouw`
  },
  {
    title: "3. Banco simples em Git (crm-lite)",
    body:
      "Crie um CRM enxuto com JSONL versionado em Git. Isso gera trilha de auditoria para leads, contatos e deals.",
    code: `mkdir -p crm-lite/data
touch crm-lite/data/prospects.jsonl
touch crm-lite/data/conversations.jsonl
touch crm-lite/data/opportunities.jsonl

git -C crm-lite init
git -C crm-lite add data
git -C crm-lite commit -m "init crm-lite database"`
  },
  {
    title: "4. Criar os agentes do time de vendas",
    body:
      "Cada agente representa uma célula da operação: oferta, prospecção, SDR, closer e revops.",
    code: `go run ./cmd/deeph agent create sales_offer --provider deepseek --model deepseek-chat
go run ./cmd/deeph agent create sales_prospect --provider deepseek --model deepseek-chat
go run ./cmd/deeph agent create sales_sdr --provider deepseek --model deepseek-chat
go run ./cmd/deeph agent create sales_closer --provider deepseek --model deepseek-chat
go run ./cmd/deeph agent create sales_revops --provider deepseek --model deepseek-chat`
  },
  {
    title: "5. Agent de oferta (estilo Alex Hormozi)",
    body:
      "O primeiro agente constrói proposta de valor, garantia e estrutura de oferta. Isso alimenta todo o restante do funil.",
    code: `# agents/sales_offer.yaml
name: sales_offer
provider: deepseek
model: deepseek-chat
system_prompt: |
  You are the Offer Architect for a high-ticket funnel.
  Build a concise offer using:
  - dream outcome
  - perceived likelihood
  - time delay reduction
  - effort/sacrifice reduction
  Return: promise, guarantee, price anchoring, and qualification criteria.
skills:
  - echo
io:
  outputs:
    - name: offer_plan
      produces: [plan/summary, summary/text]
metadata:
  context_moment: "discovery"
  context_max_input_tokens: "900"`
  },
  {
    title: "6. Agent de prospecção (ICP + lista priorizada)",
    body:
      "Esse agente gera e organiza a lista de prospects priorizando fit com a oferta e potencial de fechamento.",
    code: `# agents/sales_prospect.yaml
name: sales_prospect
provider: deepseek
model: deepseek-chat
system_prompt: |
  Build a prioritized ICP lead list from the offer plan.
  Return lead segments, qualification tags and first-touch angle.
skills:
  - file_read_range
  - file_write_safe
depends_on: [sales_offer]
depends_on_ports:
  context: [sales_offer.offer_plan]
io:
  inputs:
    - name: context
      accepts: [plan/summary, summary/text]
      required: true
      merge_policy: latest
  outputs:
    - name: leads
      produces: [data/table, summary/text]
metadata:
  context_moment: "discovery"`
  },
  {
    title: "7. Agent SDR com tool de Whats",
    body:
      "O SDR dispara abordagem, qualifica lead e registra respostas. Aqui entra a tool de Whats do kit `watsmeouw`.",
    code: `# agents/sales_sdr.yaml
name: sales_sdr
provider: deepseek
model: deepseek-chat
system_prompt: |
  Run WhatsApp SDR outreach from prioritized leads.
  Goals:
  - send first-touch messages
  - qualify pain, urgency and budget
  - output only qualified leads for closer
skills:
  - watsmeouw_send_text
  - watsmeouw_read_inbox
  - file_write_safe
depends_on: [sales_prospect]
depends_on_ports:
  context: [sales_prospect.leads]
io:
  inputs:
    - name: context
      accepts: [data/table, summary/text]
      required: true
      merge_policy: latest
  outputs:
    - name: qualified_leads
      produces: [summary/text, data/table]
metadata:
  context_moment: "tool_loop"
  max_tool_rounds: "5"
  tool_max_calls: "12"`
  },
  {
    title: "8. Agent closer (roteiro de fechamento)",
    body:
      "O closer recebe apenas leads qualificados e prepara condução de call, tratamento de objeções e próximo compromisso.",
    code: `# agents/sales_closer.yaml
name: sales_closer
provider: deepseek
model: deepseek-chat
system_prompt: |
  Close qualified opportunities with a high-ticket consultative approach.
  Produce:
  - call script
  - objection handling tree
  - close conditions and next actions
skills:
  - watsmeouw_send_text
  - echo
depends_on: [sales_sdr]
depends_on_ports:
  context: [sales_sdr.qualified_leads]
io:
  inputs:
    - name: context
      accepts: [summary/text, data/table]
      required: true
      merge_policy: latest
  outputs:
    - name: deals
      produces: [plan/summary, summary/text]
metadata:
  context_moment: "synthesis"`
  },
  {
    title: "9. Agent RevOps (atualiza CRM em Git)",
    body:
      "RevOps consolida o funil e grava snapshots no `crm-lite/data/*.jsonl` para versionamento simples e auditável.",
    code: `# agents/sales_revops.yaml
name: sales_revops
provider: deepseek
model: deepseek-chat
system_prompt: |
  Consolidate funnel metrics and update crm-lite JSONL files.
  Output:
  - stage counts
  - conversion assumptions
  - next pipeline actions
skills:
  - file_read_range
  - file_write_safe
depends_on: [sales_offer, sales_prospect, sales_sdr, sales_closer]
depends_on_ports:
  context:
    - sales_offer.offer_plan
    - sales_prospect.leads
    - sales_sdr.qualified_leads
    - sales_closer.deals
io:
  inputs:
    - name: context
      accepts: [plan/summary, data/table, summary/text]
      required: true
      merge_policy: append3
  outputs:
    - name: revenue_board
      produces: [plan/summary, summary/text]
metadata:
  context_moment: "validate"
  context_max_input_tokens: "1200"`
  },
  {
    title: "10. Crew multiverso operacional (papel por universo)",
    body:
      "Essa crew transforma o funil em DAG observável com channels tipados entre os papéis do time.",
    code: `# crews/sales_team_multiverse.yaml
name: sales_team_multiverse
description: Time comercial completo com handoffs tipados e CRM git-backed
spec: sales_offer
universes:
  - name: u_offer
    spec: sales_offer
    output_port: offer_plan
    output_kind: plan/summary
    handoff_max_chars: 260

  - name: u_prospect
    spec: sales_prospect
    depends_on: [u_offer]
    input_port: context
    output_port: leads
    output_kind: data/table
    merge_policy: latest
    handoff_max_chars: 260

  - name: u_sdr
    spec: sales_sdr
    depends_on: [u_prospect]
    input_port: context
    output_port: qualified_leads
    output_kind: summary/text
    merge_policy: latest
    handoff_max_chars: 240

  - name: u_closer
    spec: sales_closer
    depends_on: [u_sdr]
    input_port: context
    output_port: deals
    output_kind: plan/summary
    merge_policy: latest
    handoff_max_chars: 240

  - name: u_revops
    spec: sales_revops
    depends_on: [u_offer, u_prospect, u_sdr, u_closer]
    input_port: context
    output_port: revenue_board
    output_kind: plan/summary
    merge_policy: append
    handoff_max_chars: 280`
  },
  {
    title: "11. Rodar trace/run com multiverso e judge",
    body:
      "Faça trace para validar contratos e execute com judge para ter síntese final (ganhador, riscos e follow-up).",
    code: `go run ./cmd/deeph validate
go run ./cmd/deeph trace --multiverse 0 @sales_team_multiverse "consultoria B2B para empresas de servico, ticket 5k-15k"
go run ./cmd/deeph run --multiverse 0 --judge-agent guide @sales_team_multiverse "consultoria B2B para empresas de servico, ticket 5k-15k"`
  },
  {
    title: "12. Multiverso de campanha (baseline vs value_stack vs risk_reversal)",
    body:
      "Para comparar estratégias de oferta, rode universos paralelos com variações de narrativa e deixe um universo synth reconciliar tudo.",
    code: `# crews/sales_hormozi_campaign.yaml
name: sales_hormozi_campaign
spec: "sales_offer>sales_prospect>sales_sdr>sales_closer>sales_revops"
universes:
  - name: baseline
    spec: "sales_offer>sales_prospect>sales_sdr>sales_closer>sales_revops"
    output_kind: summary/text

  - name: value_stack
    spec: "sales_offer>sales_prospect>sales_sdr>sales_closer>sales_revops"
    output_kind: summary/text
    input_prefix: |
      [universe_hint]
      Emphasize value stacking and premium positioning.

  - name: risk_reversal
    spec: "sales_offer>sales_prospect>sales_sdr>sales_closer>sales_revops"
    output_kind: summary/text
    input_prefix: |
      [universe_hint]
      Emphasize guarantee and risk-reversal in all touchpoints.

  - name: speed_to_lead
    spec: "sales_offer>sales_prospect>sales_sdr>sales_closer>sales_revops"
    output_kind: summary/text
    input_prefix: |
      [universe_hint]
      Optimize speed-to-lead, concise scripts and fast follow-up.

  - name: synth
    spec: "sales_offer>sales_prospect>sales_sdr>sales_closer>sales_revops"
    output_kind: plan/summary
    depends_on: [baseline, value_stack, risk_reversal, speed_to_lead]
    merge_policy: append
    handoff_max_chars: 280

go run ./cmd/deeph trace --multiverse 0 @sales_hormozi_campaign "escalar funil high ticket para consultoria"
go run ./cmd/deeph run --multiverse 0 --judge-agent guide @sales_hormozi_campaign "escalar funil high ticket para consultoria"`
  }
];

const funnelNotes = [
  "A API de Whats usada pelo kit `watsmeouw` e nao oficial. Garanta consentimento, opt-in e conformidade local antes de operar em producao.",
  "Se os nomes das skills do kit diferirem (`watsmeouw_send_text`, `watsmeouw_read_inbox`), ajuste os YAMLs dos agents.",
  "Para banco simples, JSONL + Git funciona bem no inicio. Se volume crescer, evolua para SQLite/Postgres sem quebrar a modelagem de agentes.",
  "Use `trace` antes de cada mudanca de prompt/ports. A maior parte dos erros de funil aparece na camada de handoff e nao no provider."
];

export default function SalesFunnelTutorialPage() {
  return (
    <>
      <section className="docs-section fade-up">
        <div className="section-kicker">Tutorial</div>
        <h2>Step by step: time de vendas high ticket (estilo Alex Hormozi) no deepH</h2>
        <p>
          Este tutorial mostra como montar uma operacao comercial com{" "}
          <code className="code-inline">prospeccao</code>,{" "}
          <code className="code-inline">SDR</code>,{" "}
          <code className="code-inline">Closer</code> e{" "}
          <code className="code-inline">RevOps</code>, conectada ao Whats via kit{" "}
          <code className="code-inline">watsmeouw</code> e com CRM simples versionado em Git.
        </p>
        <div className="callout" style={{ marginTop: "0.85rem" }}>
          <strong>Objetivo:</strong> sair de chat ad hoc e operar um funil repetivel, tipado e
          auditavel com multiverso, channels e judge.
        </div>
      </section>

      <section className="docs-section fade-up" id="sales-universe-map">
        <div className="section-kicker">Universe map</div>
        <h2>Mapa visual do time comercial (multiverso operacional)</h2>
        <p>
          Cada universo cuida de um papel do funil. O runtime conecta os handoffs por{" "}
          <code className="code-inline">depends_on</code>,{" "}
          <code className="code-inline">input_port</code> e{" "}
          <code className="code-inline">output_kind</code>.
        </p>
        <div className="section-grid">
          <div className="kid-universe-scroll">
            <div className="kid-universe-map" role="img" aria-label="Fluxo entre universos do time comercial">
              <svg className="kid-universe-lines" viewBox="0 0 900 460" aria-hidden="true">
                <defs>
                  <marker
                    id="kid-arrow-sales"
                    markerWidth="10"
                    markerHeight="8"
                    refX="8"
                    refY="4"
                    orient="auto"
                    markerUnits="strokeWidth"
                  >
                    <path d="M0,0 L10,4 L0,8 z" fill="currentColor" />
                  </marker>
                </defs>

                <path className="kid-line kid-line-contract" d="M170 110 C220 130 248 172 290 220" markerEnd="url(#kid-arrow-sales)" />
                <path className="kid-line kid-line-backend" d="M350 220 C468 168 568 158 660 172" markerEnd="url(#kid-arrow-sales)" />
                <path className="kid-line kid-line-backend" d="M350 238 C470 286 568 320 660 334" markerEnd="url(#kid-arrow-sales)" />
                <path className="kid-line kid-line-frontend" d="M720 180 C662 248 566 308 482 350" markerEnd="url(#kid-arrow-sales)" />
                <path className="kid-line kid-line-test" d="M720 334 C654 350 560 358 484 360" markerEnd="url(#kid-arrow-sales)" />
                <path className="kid-line kid-line-contract" d="M170 110 C256 218 324 318 434 352" markerEnd="url(#kid-arrow-sales)" />
              </svg>

              <article className="kid-node kid-node-contract">
                <h3>u_offer</h3>
                <p>Define oferta high ticket e criterios de qualificacao.</p>
                <span>out: plan/summary</span>
              </article>

              <article className="kid-node kid-node-backend">
                <h3>u_prospect</h3>
                <p>Monta ICP e lista priorizada de prospects.</p>
                <span>out: data/table</span>
              </article>

              <article className="kid-node kid-node-frontend">
                <h3>u_sdr</h3>
                <p>Abordagem no Whats, qualificacao e passagem ao closer.</p>
                <span>out: summary/text</span>
              </article>

              <article className="kid-node kid-node-test">
                <h3>u_closer</h3>
                <p>Conducao de fechamento e tratamento de objecoes.</p>
                <span>out: plan/summary</span>
              </article>

              <article className="kid-node kid-node-synth">
                <h3>u_revops</h3>
                <p>Consolida pipeline, atualiza CRM e define proximo ciclo.</p>
                <span>out: plan/summary</span>
              </article>
            </div>
          </div>

          <ul className="kid-channel-list">
            {universeChannels.map((edge) => (
              <li key={edge.path}>
                <code className="code-inline">{edge.path}</code>
                <span>
                  type=<code className="code-inline">{edge.type}</code> | peso={edge.weight} |
                  momento=<code className="code-inline">{edge.moment}</code>
                </span>
              </li>
            ))}
          </ul>

          <CodeBlock
            language="yaml"
            code={`# relacoes declaradas em universes (deepH atual)
universes:
  - name: u_offer
    output_port: offer_plan
    output_kind: plan/summary

  - name: u_prospect
    depends_on: [u_offer]
    input_port: context
    output_port: leads
    output_kind: data/table

  - name: u_sdr
    depends_on: [u_prospect]
    input_port: context
    output_port: qualified_leads
    output_kind: summary/text

  - name: u_closer
    depends_on: [u_sdr]
    input_port: context
    output_port: deals
    output_kind: plan/summary

  - name: u_revops
    depends_on: [u_offer, u_prospect, u_sdr, u_closer]
    input_port: context
    output_port: revenue_board
    output_kind: plan/summary`}
          />
        </div>
      </section>

      <section className="docs-section fade-up" id="sales-universe-steps">
        <div className="section-kicker">Universe steps</div>
        <h2>Passo a passo do multiverso comercial</h2>
        <p>
          Sequencia recomendada para montar um funil de vendas de alto nivel sem perder controle
          de contexto, custo e handoffs.
        </p>
        <div className="section-grid">
          <div className="stack">
            {universeBuildSteps.map((step, index) => (
              <div key={step.title} className="panel">
                <div className="tag accent">Universo · passo {index + 1}</div>
                <h3 style={{ marginTop: "0.45rem" }}>{step.title}</h3>
                <p>{step.text}</p>
                <div style={{ marginTop: "0.65rem" }}>
                  <CodeBlock code={step.code} language={step.code.trim().startsWith("#") ? "yaml" : "bash"} />
                </div>
              </div>
            ))}
          </div>
          <div className="panel">
            <h3>Complementos recomendados</h3>
            <p>
              Para dominar contratos de universos e troubleshooting de DAG/channels, use as docs
              dedicadas e depois volte para este tutorial.
            </p>
            <div className="mini-link-list">
              <Link href="/docs/universos">Abrir docs de Universos</Link>
              <Link href="/docs/calculadora">Ver tutorial da calculadora</Link>
            </div>
          </div>
        </div>
      </section>

      {funnelSteps.map((step, index) => (
        <section key={step.title} className="docs-section fade-up" id={`sales-step-${index + 1}`}>
          <div className="section-kicker">Passo {index + 1}</div>
          <h2>{step.title}</h2>
          <p>{step.body}</p>
          <div className="section-grid">
            <CodeBlock code={step.code} language={step.code.trim().startsWith("#") ? "yaml" : "bash"} />
          </div>
        </section>
      ))}

      <section className="docs-section fade-up">
        <div className="section-kicker">Notas</div>
        <h2>Observacoes importantes</h2>
        <div className="section-grid">
          <ul className="subtle-list">
            {funnelNotes.map((note) => (
              <li key={note}>{note}</li>
            ))}
          </ul>
          <div className="panel">
            <h3>Proximo passo recomendado</h3>
            <p>
              Rode o tutorial completo com 2 ou 3 briefs reais do seu negocio e compare o output
              de cada universo antes de promover para operacao recorrente.
            </p>
            <div className="mini-link-list">
              <Link href="/docs">Voltar para docs completas</Link>
              <Link href="/docs/comparativo-claude-code">
                Comparativo com Claude Code
              </Link>
            </div>
          </div>
        </div>
      </section>
    </>
  );
}
