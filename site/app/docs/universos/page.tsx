import Link from "next/link";
import { CodeBlock } from "@/components/code-block";

const universeSteps = [
  {
    title: "1. Defina o objetivo de cada universo",
    text:
      "Separe por responsabilidade real: contrato, backend, frontend, testes e síntese. Evite universos duplicados sem função clara.",
    code: `# exemplo de specs por responsabilidade
calc_contract
calc_backend
calc_frontend
calc_tester
calc_synth`
  },
  {
    title: "2. Crie a crew com universes",
    text:
      "A crew é o container do multiverso. Cada universe aponta para um spec e pode sobrescrever comportamento com prefix/suffix.",
    code: `# crews/calc_multiverse.yaml
name: calc_multiverse
description: Calculadora fullstack com universos colaborando
spec: calc_contract
universes:
  - name: u_contract
    spec: calc_contract
    output_port: openapi
    output_kind: contract/openapi

  - name: u_backend
    spec: calc_backend
    depends_on: [u_contract]
    input_port: context
    output_port: api_summary
    output_kind: summary/api

  - name: u_frontend
    spec: calc_frontend
    depends_on: [u_backend]
    input_port: context
    output_port: page
    output_kind: frontend/page

  - name: u_test
    spec: calc_tester
    depends_on: [u_backend]
    input_port: context
    output_port: routes_tests
    output_kind: backend/route

  - name: u_synth
    spec: calc_synth
    depends_on: [u_contract, u_backend, u_frontend, u_test]
    input_port: context
    output_port: result
    output_kind: plan/summary`
  },
  {
    title: "3. Entenda como os channels são formados",
    text:
      "No deepH atual, channels entre universos são inferidos de depends_on + output_port/input_port. O tipo vem do output_kind do universo de origem.",
    code: `# relações inferidas no exemplo acima:
# u_contract.openapi    -> u_backend.context#contract/openapi
# u_backend.api_summary -> u_frontend.context#summary/api
# u_backend.api_summary -> u_test.context#summary/api
# u_frontend.page       -> u_synth.context#frontend/page
# u_test.routes_tests   -> u_synth.context#backend/route`
  },
  {
    title: "4. Controle merge e tamanho do handoff",
    text:
      "Use merge_policy e handoff_max_chars para impedir payload inflado e replay desnecessário.",
    code: `# por universe consumidor
- name: u_synth
  depends_on: [u_contract, u_backend, u_frontend, u_test]
  input_port: context
  output_port: result
  output_kind: plan/summary
  merge_policy: append
  handoff_max_chars: 260`
  },
  {
    title: "5. Trace antes de run",
    text:
      "Sempre rode trace para validar DAG, scheduler e handoffs antes da execução real.",
    code: `deeph validate
deeph crew list
deeph crew show calc_multiverse
deeph trace --multiverse 0 @calc_multiverse "crie calculadora Next.js com API /api/calc"`
  },
  {
    title: "6. Rode com judge para consolidar",
    text:
      "Quando houver múltiplos outputs relevantes, use judge-agent para reconciliar e escolher a melhor síntese final.",
    code: `deeph run --multiverse 0 --judge-agent guide @calc_multiverse "crie calculadora Next.js com API /api/calc"`
  },
  {
    title: "7. Itere com ajuste fino por universe",
    text:
      "Se o problema está no backend, ajuste u_backend (prompt/skills/metadata) em vez de mexer em todo o pipeline.",
    code: `# exemplos de ajustes focados
# - aumentar handoff_max_chars só no u_backend
# - trocar merge_policy de append para latest no u_frontend
# - reforçar input_prefix de segurança no u_test`
  }
];

export default function UniversesDocsPage() {
  return (
    <>
      <section className="docs-section fade-up">
        <div className="section-kicker">Universes</div>
        <h2>Documentação de Universos (multiverso no deepH)</h2>
        <p>
          Universos são branches de execução controladas em <code className="code-inline">crews/*.yaml</code>.
          Eles permitem explorar estratégias diferentes e compartilhar handoffs tipados com baixo custo.
        </p>
        <div className="callout" style={{ marginTop: "0.85rem" }}>
          <strong>Regra prática:</strong> universo bom tem responsabilidade clara, output_kind útil e
          depende só do necessário.
        </div>
      </section>

      <section className="docs-section fade-up" id="universes-modelo">
        <div className="section-kicker">Modelo mental</div>
        <h2>Como pensar universos corretamente</h2>
        <div className="section-grid">
          <div className="panel">
            <h3>1 universo = 1 papel</h3>
            <p>
              Evite universo genérico. Prefira nomes como <code className="code-inline">u_contract</code>,{" "}
              <code className="code-inline">u_backend</code>, <code className="code-inline">u_frontend</code>,{" "}
              <code className="code-inline">u_test</code> e <code className="code-inline">u_synth</code>.
            </p>
          </div>
          <div className="panel">
            <h3>Channels são inferidos</h3>
            <p>
              No formato atual, você não declara um bloco <code className="code-inline">channels:</code>
              na crew. O runtime monta os channels automaticamente a partir de
              <code className="code-inline"> depends_on</code> + ports + output_kind.
            </p>
          </div>
          <div className="panel">
            <h3>Payload compacto sempre</h3>
            <p>
              Use <code className="code-inline">summary/*</code> e portas semânticas para circular contexto.
              Menos texto bruto, mais sinal para o próximo universo.
            </p>
          </div>
          <div className="panel">
            <h3>DAG antes de paralelismo</h3>
            <p>
              Primeiro garanta dependências corretas. Depois paralelize onde não há acoplamento
              (ex.: frontend e tests após backend).
            </p>
          </div>
        </div>
      </section>

      <section className="docs-section fade-up" id="universes-campos">
        <div className="section-kicker">Schema</div>
        <h2>Campos de universe que importam</h2>
        <div className="section-grid">
          <CodeBlock
            language="yaml"
            code={`universes:
  - name: u_backend
    spec: calc_backend
    input_prefix: |
      [universe_hint]
      Priorize segurança e validação de entrada.
    input_suffix: ""
    depends_on: [u_contract]
    input_port: context
    output_port: api_summary
    output_kind: summary/api
    merge_policy: latest
    handoff_max_chars: 260`}
          />
          <ul className="subtle-list">
            <li><code className="code-inline">name</code>: nome único do universo.</li>
            <li><code className="code-inline">spec</code>: agent/spec a executar nesse universo.</li>
            <li><code className="code-inline">depends_on</code>: universos upstream (forma a DAG).</li>
            <li><code className="code-inline">input_port</code> / <code className="code-inline">output_port</code>: portas de handoff.</li>
            <li><code className="code-inline">output_kind</code>: tipo semântico emitido no handoff.</li>
            <li><code className="code-inline">merge_policy</code>: estratégia de merge no consumidor (<code className="code-inline">append</code> ou <code className="code-inline">latest</code>).</li>
            <li><code className="code-inline">handoff_max_chars</code>: limite de caracteres no handoff entre universos.</li>
          </ul>
        </div>
      </section>

      <section className="docs-section fade-up" id="universes-step-by-step">
        <div className="section-kicker">Step by step</div>
        <h2>Passo a passo completo</h2>
        <div className="section-grid">
          <div className="stack">
            {universeSteps.map((step, index) => (
              <div key={step.title} className="panel">
                <div className="tag accent">Universos · passo {index + 1}</div>
                <h3 style={{ marginTop: "0.45rem" }}>{step.title}</h3>
                <p>{step.text}</p>
                <div style={{ marginTop: "0.6rem" }}>
                  <CodeBlock code={step.code} language={step.code.trim().startsWith("#") ? "yaml" : "bash"} />
                </div>
              </div>
            ))}
          </div>
          <div className="panel">
            <h3>Comandos de operação diária</h3>
            <CodeBlock
              language="bash"
              code={`# validar estrutura
deeph validate

# inspecionar crews
deeph crew list
deeph crew show calc_multiverse

# planejar execução
deeph trace --multiverse 0 @calc_multiverse "task"

# executar multiverso + judge
deeph run --multiverse 0 --judge-agent guide @calc_multiverse "task"`}
            />
            <div className="mini-link-list" style={{ marginTop: "0.75rem" }}>
              <Link href="/docs/calculadora">Ver tutorial da calculadora</Link>
              <Link href="/docs#multiverso-codigo">Voltar para visão geral de multiverso</Link>
            </div>
          </div>
        </div>
      </section>

      <section className="docs-section fade-up" id="universes-debug">
        <div className="section-kicker">Troubleshooting</div>
        <h2>Erros comuns e correção rápida</h2>
        <div className="section-grid">
          <div className="panel">
            <h3>depends_on unknown universe</h3>
            <p>
              Nome no <code className="code-inline">depends_on</code> não bate com
              <code className="code-inline"> name</code> de outro universo.
            </p>
          </div>
          <div className="panel">
            <h3>dependency cycle detected</h3>
            <p>
              A DAG tem ciclo. Quebre o loop (ex.: A depende de B e B depende de A).
            </p>
          </div>
          <div className="panel">
            <h3>Handoff grande demais</h3>
            <p>
              Reduza <code className="code-inline">handoff_max_chars</code> e force saída em
              <code className="code-inline"> summary/*</code> para não explodir token.
            </p>
          </div>
          <div className="panel">
            <h3>Saída inconsistente entre universos</h3>
            <p>
              Padronize contratos de porta e use <code className="code-inline">output_kind</code>
              semântico por camada.
            </p>
          </div>
        </div>
      </section>
    </>
  );
}
