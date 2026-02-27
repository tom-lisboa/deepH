import Link from "next/link";
import { CodeBlock } from "@/components/code-block";
import { calculatorTutorial } from "@/lib/site-data";

const universeChannels = [
  {
    path: "u_contract.openapi -> u_backend.context",
    type: "contract/openapi",
    weight: "6",
    moment: "backend_codegen"
  },
  {
    path: "u_backend.api_summary -> u_frontend.context",
    type: "summary/api",
    weight: "5",
    moment: "frontend_codegen"
  },
  {
    path: "u_backend.api_summary -> u_test.context",
    type: "summary/api",
    weight: "5",
    moment: "validate"
  },
  {
    path: "u_frontend.page -> u_synth.context",
    type: "frontend/page",
    weight: "4",
    moment: "synthesis"
  },
  {
    path: "u_test.routes_tests -> u_synth.context",
    type: "backend/route",
    weight: "4",
    moment: "synthesis"
  },
  {
    path: "u_contract.openapi -> u_synth.context",
    type: "contract/openapi",
    weight: "6",
    moment: "synthesis"
  }
];

const universeBuildSteps = [
  {
    title: "1. Defina os agentes-base por camada",
    text:
      "Antes do multiverso, garanta os agentes especialistas (contrato, backend, frontend, teste e síntese). Cada universo vai reaproveitar esses specs.",
    code: `deeph agent create calc_contract --provider deepseek --model deepseek-chat
deeph agent create calc_backend --provider deepseek --model deepseek-chat
deeph agent create calc_frontend --provider deepseek --model deepseek-chat
deeph agent create calc_tester --provider deepseek --model deepseek-chat
deeph agent create calc_synth --provider deepseek --model deepseek-chat`
  },
  {
    title: "2. Monte a crew com universos e dependências",
    text:
      "Modele a DAG entre universos com depends_on. Pense como pipeline: contrato -> implementação -> validação -> síntese.",
    code: `# crews/calc_multiverse.yaml
name: calc_multiverse
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
    title: "3. Conecte os channels tipados",
    text:
      "No deepH atual, os channels entre universos são inferidos por depends_on + ports. O tipo vem do output_kind do universo de origem.",
    code: `# crews/calc_multiverse.yaml (trecho)
universes:
  - name: u_contract
    output_port: openapi
    output_kind: contract/openapi

  - name: u_backend
    depends_on: [u_contract]
    input_port: context
    output_port: api_summary
    output_kind: summary/api

  - name: u_frontend
    depends_on: [u_backend]
    input_port: context
    output_port: page
    output_kind: frontend/page

  - name: u_test
    depends_on: [u_backend]
    input_port: context
    output_port: routes_tests
    output_kind: backend/route`
  },
  {
    title: "4. Aplique pesos e momentos por fase",
    text:
      "Dê mais peso ao que importa em cada etapa. Exemplo: contrato pesa mais no backend; UI pesa mais no synth final.",
    code: `# agents/calc_backend.yaml (trecho)
metadata:
  context_moment: "backend_codegen"
  type_weights:
    contract/openapi: "6"
    summary/api: "5"
    frontend/page: "1"
  context_max_input_tokens: "1000"
  context_max_channels: "10"

# agents/calc_synth.yaml (trecho)
metadata:
  context_moment: "synthesis"
  type_weights:
    frontend/page: "4"
    backend/route: "4"
    contract/openapi: "6"`
  },
  {
    title: "5. Trace antes de executar",
    text:
      "Valide a DAG, os channels e os handoffs no trace. Corrija contratos antes de gastar token no run.",
    code: `deeph validate
deeph trace --multiverse 0 @calc_multiverse "crie calculadora Next.js com API /api/calc"`
  },
  {
    title: "6. Rode com judge para reconciliar",
    text:
      "Execute os universos e use um judge-agent para consolidar a melhor saída final.",
    code: `deeph run --multiverse 0 --judge-agent guide @calc_multiverse "crie calculadora Next.js com API /api/calc"
deeph run --trace --multiverse 0 --judge-agent guide @calc_multiverse "adicione validação de expressão"`
  },
  {
    title: "7. Itere por porta (sem loop cego)",
    text:
      "Ajuste só o universo/porta com problema. Evite retrabalho global em todo pipeline.",
    code: `# exemplo de ajuste fino
# - aumentar max_tokens apenas em u_backend.context
# - trocar merge_policy de append -> latest em inputs específicos
# - reduzir handoff_max_chars no u_test para payload compacto`
  }
];

export default function CalculatorTutorialPage() {
  return (
    <>
      <section className="docs-section fade-up">
        <div className="section-kicker">Tutorial</div>
        <h2>{calculatorTutorial.title}</h2>
        <p>{calculatorTutorial.premise}</p>
        <div className="callout" style={{ marginTop: "0.85rem" }}>
          <strong>Objetivo:</strong> aprender ports/channels tipados e DAG no `deepH` usando
          um caso simples e útil. Depois você pode trocar o solver por uma skill determinística.
        </div>
      </section>

      <section className="docs-section fade-up" id="calculator-universe-map">
        <div className="section-kicker">Universe map</div>
        <h2>Mapa visual da calculadora (estilo caderno)</h2>
        <p>
          Cada universo cuida de uma parte da calculadora e conversa por channels tipados.
          No runtime atual, esses channels são inferidos automaticamente via{" "}
          <code className="code-inline">depends_on</code> + ports.
        </p>
        <div className="section-grid">
          <div className="kid-universe-scroll">
            <div className="kid-universe-map" role="img" aria-label="Fluxo entre universos da calculadora">
              <svg className="kid-universe-lines" viewBox="0 0 900 460" aria-hidden="true">
                <defs>
                  <marker
                    id="kid-arrow"
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

                <path className="kid-line kid-line-contract" d="M170 110 C220 130 248 172 290 220" markerEnd="url(#kid-arrow)" />
                <path className="kid-line kid-line-backend" d="M350 220 C468 168 568 158 660 172" markerEnd="url(#kid-arrow)" />
                <path className="kid-line kid-line-backend" d="M350 238 C470 286 568 320 660 334" markerEnd="url(#kid-arrow)" />
                <path className="kid-line kid-line-frontend" d="M720 180 C662 248 566 308 482 350" markerEnd="url(#kid-arrow)" />
                <path className="kid-line kid-line-test" d="M720 334 C654 350 560 358 484 360" markerEnd="url(#kid-arrow)" />
                <path className="kid-line kid-line-contract" d="M170 110 C256 218 324 318 434 352" markerEnd="url(#kid-arrow)" />
              </svg>

              <article className="kid-node kid-node-contract">
                <h3>u_contract</h3>
                <p>Define o contrato da API da calculadora.</p>
                <span>out: contract/openapi</span>
              </article>

              <article className="kid-node kid-node-backend">
                <h3>u_backend</h3>
                <p>Cria route/controller/evaluator do backend.</p>
                <span>out: summary/api</span>
              </article>

              <article className="kid-node kid-node-frontend">
                <h3>u_frontend</h3>
                <p>Monta a UI e integração com POST /api/calc.</p>
                <span>out: frontend/page</span>
              </article>

              <article className="kid-node kid-node-test">
                <h3>u_test</h3>
                <p>Gera testes e checklist das rotas.</p>
                <span>out: backend/route</span>
              </article>

              <article className="kid-node kid-node-synth">
                <h3>u_synth</h3>
                <p>Concilia tudo em um plano final enxuto.</p>
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
            code={`# Forma correta hoje: relações declaradas em universes
universes:
  - name: u_contract
    output_port: openapi
    output_kind: contract/openapi

  - name: u_backend
    depends_on: [u_contract]
    input_port: context
    output_port: api_summary
    output_kind: summary/api
    merge_policy: latest
    handoff_max_chars: 260

  - name: u_frontend
    depends_on: [u_backend]
    input_port: context
    output_port: page
    output_kind: frontend/page

  - name: u_test
    depends_on: [u_backend]
    input_port: context
    output_port: routes_tests
    output_kind: backend/route

  - name: u_synth
    depends_on: [u_contract, u_backend, u_frontend, u_test]
    input_port: context
    output_port: result
    output_kind: plan/summary`}
          />
        </div>
      </section>

      <section className="docs-section fade-up" id="calculator-universe-steps">
        <div className="section-kicker">Universe steps</div>
        <h2>Passo a passo dos universos (correto e completo)</h2>
        <p>
          Fluxo recomendado para construir a calculadora com multiverso, sem loop inútil e com
          consumo de token previsível.
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
            <h3>Quer detalhamento completo de universos?</h3>
            <p>
              A documentação dedicada cobre arquitetura, contratos, patterns de channels,
              troubleshooting e templates prontos para backend/frontend/CRUD.
            </p>
            <div className="mini-link-list">
              <Link href="/docs/universos">Abrir docs de Universos</Link>
              <Link href="/docs#multiverso-codigo">Ver seção multiverso no overview</Link>
            </div>
          </div>
        </div>
      </section>

      {calculatorTutorial.steps.map((step, index) => (
        <section key={step.title} className="docs-section fade-up" id={`step-${index + 1}`}>
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
        <h2>Observações importantes</h2>
        <div className="section-grid">
          <ul className="subtle-list">
            {calculatorTutorial.notes.map((note) => (
              <li key={note}>{note}</li>
            ))}
          </ul>
          <div className="panel">
            <h3>O que melhorar depois</h3>
            <p>
              Se você quiser transformar essa calculadora em algo confiável para produção,
              o próximo passo é implementar uma skill determinística de matemática (ex.: parser/eval)
              e manter o formatter como agent separado.
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
