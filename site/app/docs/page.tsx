import Link from "next/link";
import { CodeBlock } from "@/components/code-block";
import {
  allTypeKinds,
  architectureHighlights,
  bestPractices,
  crudMultiverseCrewGuide,
  customizationGuide,
  deepseekToolingResearch,
  docsConcepts,
  groupCommandsByCategory,
  groupTypesByCategory,
  kitsGuide,
  projectUsageGuide,
  quickStartCommands
} from "@/lib/site-data";

export default function DocsPage() {
  const commandGroups = groupCommandsByCategory();
  const typeGroups = groupTypesByCategory();

  return (
    <>
      <section id="overview" className="docs-section fade-up">
        <div className="section-kicker">Overview</div>
        <h2>Visão geral</h2>
        <p>
          O `deepH` é um runtime de agentes em Go com foco em leveza, orquestração e baixo
          consumo de token. O usuário define agents/crews em YAML, e o core fornece
          scheduler, context compiler, tool loop, budgets e observabilidade.
        </p>
        <div className="section-grid">
          <div className="grid-2" style={{ marginTop: 0 }}>
            {docsConcepts.slice(0, 4).map((concept) => (
              <div key={concept.title} className="panel">
                <h3>{concept.title}</h3>
                <p>{concept.body}</p>
              </div>
            ))}
          </div>
          <div className="grid-2" style={{ marginTop: 0 }}>
            {docsConcepts.slice(4).map((concept) => (
              <div key={concept.title} className="panel">
                <h3>{concept.title}</h3>
                <p>{concept.body}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <section id="quickstart" className="docs-section fade-up">
        <div className="section-kicker">Quick start</div>
        <h2>Quick Start</h2>
        <p>
          Fluxo mínimo para validar workspace, instalar skill, traçar e executar um agent.
        </p>
        <div className="section-grid">
          <CodeBlock code={quickStartCommands.join("\n")} language="bash" />
          <div className="callout">
            <strong>Dica:</strong> para DeepSeek real, rode{" "}
            <code className="code-inline">deeph provider add deepseek --set-default</code> e
            exporte <code className="code-inline">DEEPSEEK_API_KEY</code>.
          </div>
        </div>
      </section>

      <section id="kits" className="docs-section fade-up">
        <div className="section-kicker">Starter kits</div>
        <h2>{kitsGuide.title}</h2>
        <p>{kitsGuide.summary}</p>
        <div className="section-grid">
          <CodeBlock code={kitsGuide.quickCommands} language="bash" />
          <div className="panel">
            <h3>Comportamento do instalador</h3>
            <ul className="subtle-list">
              {kitsGuide.behavior.map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          </div>
          <div className="panel">
            <h3>Manifesto remoto (`deeph-kit.yaml`)</h3>
            <CodeBlock code={kitsGuide.manifest} language="yaml" />
          </div>
          <div className="panel">
            <h3>Notas importantes</h3>
            <ul className="subtle-list">
              {kitsGuide.notes.map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          </div>
        </div>
      </section>

      <section id="conceitos" className="docs-section fade-up">
        <div className="section-kicker">Concepts</div>
        <h2>Conceitos-chave</h2>
        <div className="section-grid">
          <div className="panel">
            <h3>Spec de execução</h3>
            <p>
              O spec pode ser simples (<code className="code-inline">guide</code>), paralelo (
              <code className="code-inline">a+b</code>) ou em estágios (
              <code className="code-inline">a+b&gt;c&gt;d</code>).
            </p>
          </div>
          <div className="panel">
            <h3>Handoffs por porta</h3>
            <p>
              Use <code className="code-inline">io.inputs</code>,{" "}
              <code className="code-inline">io.outputs</code> e{" "}
              <code className="code-inline">depends_on_ports</code> para roteamento fino e
              merge por porta.
            </p>
          </div>
          <div className="panel">
            <h3>Channels entre universos</h3>
            <p>
              Em crews, universos podem declarar <code className="code-inline">depends_on</code>{" "}
              e trocar handoffs compactos tipados (
              <code className="code-inline">u1.result-&gt;u3.context#summary/text</code>).
            </p>
          </div>
          <div className="panel">
            <h3>Momentos de contexto</h3>
            <p>
              O compiler usa <code className="code-inline">context_moment</code> (ex.:{" "}
              <code className="code-inline">tool_loop</code>,{" "}
              <code className="code-inline">synthesis</code>) para priorizar tipos certos no
              prompt.
            </p>
          </div>
        </div>
      </section>

      <section id="arquitetura" className="docs-section fade-up">
        <div className="section-kicker">Architecture</div>
        <h2>Arquitetura (resumo prático)</h2>
        <p>
          O runtime já entrega uma base forte de orquestração e token economy para workflows
          sérios em Go.
        </p>
        <div className="section-grid">
          <ul className="subtle-list">
            {architectureHighlights.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
          <CodeBlock
            language="bash"
            code={`# Inspecione antes de rodar
deeph trace "planner+reader>coder>reviewer" "implemente feature X"

# Rode com multiverso + judge
deeph run --multiverse 0 --judge-agent guide @reviewpack "implemente feature X"

# Debug/export para UI
deeph trace --json "planner+reader>coder>reviewer" "feature X"`}
          />
        </div>
      </section>

      <section id="comandos" className="docs-section fade-up">
        <div className="section-kicker">Commands</div>
        <h2>Comandos (referência completa)</h2>
        <p>
          O dicionário abaixo resume o CLI atual. Para detalhes por comando, use também{" "}
          <code className="code-inline">deeph command explain</code> e{" "}
          <code className="code-inline">deeph command list --json</code>.
        </p>
        <div className="section-grid">
          {commandGroups.map((group) => (
            <div key={group.category} className="panel">
              <h3 style={{ textTransform: "capitalize" }}>{group.category}</h3>
              <table className="split-table">
                <thead>
                  <tr>
                    <th>Comando</th>
                    <th>Resumo</th>
                  </tr>
                </thead>
                <tbody>
                  {group.items.map((cmd) => (
                    <tr key={cmd.path}>
                      <td>
                        <code className="code-inline">{cmd.path}</code>
                      </td>
                      <td>{cmd.summary}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ))}
          <div className="callout">
            <strong>Comandos úteis de export:</strong> use{" "}
            <code className="code-inline">command list --json</code>,{" "}
            <code className="code-inline">command explain --json</code>,{" "}
            <code className="code-inline">type list --json</code> e{" "}
            <code className="code-inline">trace --json</code> para docs internas, UI e automação.
          </div>
        </div>
      </section>

      <section id="tipos" className="docs-section fade-up">
        <div className="section-kicker">Types</div>
        <h2>Tipos semânticos (referência completa)</h2>
        <p>
          O type system orienta contexto, handoffs, channels e merge. O runtime também aceita
          aliases (ex.: <code className="code-inline">go</code>,{" "}
          <code className="code-inline">string</code>) e normaliza para o tipo canônico.
        </p>
        <div className="section-grid">
          <div className="panel">
            <h3>Resumo</h3>
            <p>
              Total de tipos canônicos: <strong>{allTypeKinds.length}</strong>
            </p>
            <div className="pill-list">
              {typeGroups.map((g) => (
                <span key={g.category} className="tag">
                  {g.category}: {g.items.length}
                </span>
              ))}
            </div>
          </div>
          {typeGroups.map((group) => (
            <div key={group.category} className="panel">
              <h3 style={{ textTransform: "capitalize" }}>{group.category}</h3>
              <table className="split-table">
                <thead>
                  <tr>
                    <th>Tipo</th>
                    <th>Descrição</th>
                  </tr>
                </thead>
                <tbody>
                  {group.items.map((item) => (
                    <tr key={item.kind}>
                      <td>
                        <code className="code-inline">{item.kind}</code>
                      </td>
                      <td>{item.description}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ))}
        </div>
      </section>

      <section id="boas-praticas" className="docs-section fade-up">
        <div className="section-kicker">Best practices</div>
        <h2>Boas práticas (deepH)</h2>
        <p>
          Se você quer resultados “clínicos” em custo e orquestração, essas regras pagam
          muito rápido.
        </p>
        <div className="section-grid">
          <ul className="subtle-list">
            {bestPractices.map((tip) => (
              <li key={tip}>{tip}</li>
            ))}
          </ul>
          <div className="grid-2" style={{ marginTop: 0 }}>
            <div className="panel">
              <h3>Próximo passo recomendado</h3>
              <p>
                Comece pelo tutorial da calculadora para internalizar ports/channels e depois
                evolua para crew multiverso com judge.
              </p>
              <div className="mini-link-list">
                <Link href="/docs/calculadora">Abrir tutorial da calculadora</Link>
                <Link href="/docs/comparativo-claude-code">
                  Ver comparação com Claude Code
                </Link>
              </div>
            </div>
            <div className="panel">
              <h3>Comandos de debug preferidos</h3>
              <CodeBlock
                language="bash"
                code={`deeph validate
deeph trace --json "a+b>c" "task"
deeph run --trace "a+b>c" "task"
deeph coach stats --json`}
              />
            </div>
          </div>
        </div>
      </section>

      <section id="customizacao" className="docs-section fade-up">
        <div className="section-kicker">Customization</div>
        <h2>Passo a passo: criar seus próprios agents e skills</h2>
        <p>
          No `deepH`, o usuário controla os agents. E skills têm dois níveis: configuração em
          YAML (mais comum) e novo tipo de skill no core em Go (avançado).
        </p>
        <div className="section-grid">
          <div className="panel">
            <h3>Agents (YAML do usuário)</h3>
            <div className="stack" style={{ marginTop: "0.6rem" }}>
              {customizationGuide.agentSteps.map((step, idx) => (
                <div key={step.title} className="panel" style={{ background: "rgba(255,255,255,0.015)" }}>
                  <div className="tag accent">Passo {idx + 1}</div>
                  <h3 style={{ marginTop: "0.45rem" }}>{step.title}</h3>
                  <p>{step.text}</p>
                  <div style={{ marginTop: "0.6rem" }}>
                    <CodeBlock code={step.code} language="yaml" />
                  </div>
                </div>
              ))}
            </div>
          </div>

          <div className="panel">
            <h3>Skills (configuradas e novas)</h3>
            <div className="stack" style={{ marginTop: "0.6rem" }}>
              {customizationGuide.skillSteps.map((step, idx) => (
                <div key={step.title} className="panel" style={{ background: "rgba(255,255,255,0.015)" }}>
                  <div className="tag warn">Skill · passo {idx + 1}</div>
                  <h3 style={{ marginTop: "0.45rem" }}>{step.title}</h3>
                  <p>{step.text}</p>
                  <div style={{ marginTop: "0.6rem" }}>
                    <CodeBlock code={step.code} language="yaml" />
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      <section id="uso-em-projetos" className="docs-section fade-up">
        <div className="section-kicker">Projects</div>
        <h2>Como usar o deepH dentro de um projeto (ex.: `hello-world`)</h2>
        <p>{projectUsageGuide.summary}</p>
        <div className="section-grid">
          {projectUsageGuide.modes.map((mode) => (
            <div key={mode.title} className="panel">
              <h3>{mode.title}</h3>
              <p>{mode.body}</p>
              <div style={{ marginTop: "0.6rem" }}>
                <CodeBlock code={mode.code} language="bash" />
              </div>
            </div>
          ))}

          <div className="grid-2" style={{ marginTop: 0 }}>
            {projectUsageGuide.examples.map((ex) => (
              <div key={ex.title} className="panel">
                <h3>{ex.title}</h3>
                <CodeBlock code={ex.code} language="bash" />
              </div>
            ))}
          </div>
        </div>
      </section>

      <section id="multiverso-codigo" className="docs-section fade-up">
        <div className="section-kicker">Multiverse codegen</div>
        <h2>{crudMultiverseCrewGuide.title}</h2>
        <p>{crudMultiverseCrewGuide.summary}</p>
        <div className="section-grid">
          <div className="panel">
            <h3>Crew YAML (exemplo)</h3>
            <CodeBlock code={crudMultiverseCrewGuide.crewYaml} language="yaml" />
          </div>

          <div className="panel">
            <h3>Channels tipados entre universos</h3>
            <ul className="subtle-list">
              {crudMultiverseCrewGuide.channels.map((line) => (
                <li key={line}>
                  <code className="code-inline">{line}</code>
                </li>
              ))}
            </ul>
            <div className="callout" style={{ marginTop: "0.8rem" }}>
              <strong>Ideia central:</strong> um universo contribui para outro com o tipo certo e
              payload compacto (<code className="code-inline">summary/*</code> /{" "}
              <code className="code-inline">artifact/ref</code>), sem replay bruto.
            </div>
          </div>

          <div className="panel">
            <h3>Pesos por tipificação (proposta)</h3>
            <p>
              Para CRUD/backend/frontend, vale priorizar tipos diferentes por fase
              (<code className="code-inline">contract_phase</code>,{" "}
              <code className="code-inline">backend_codegen</code>,{" "}
              <code className="code-inline">frontend_codegen</code>,{" "}
              <code className="code-inline">validate</code>).
            </p>
            <div style={{ marginTop: "0.6rem" }}>
              <CodeBlock code={crudMultiverseCrewGuide.weightsExample} language="yaml" />
            </div>
          </div>

          <div className="panel">
            <h3>Boas práticas de multiverso para codegen</h3>
            <ul className="subtle-list">
              {crudMultiverseCrewGuide.notes.map((note) => (
                <li key={note}>{note}</li>
              ))}
            </ul>
          </div>
        </div>
      </section>

      <section id="deepseek-tools" className="docs-section fade-up">
        <div className="section-kicker">DeepSeek</div>
        <h2>DeepSeek tools vs skills do deepH (pesquisa + decisão prática)</h2>
        <p>
          A DeepSeek suporta tool/function calling no `chat/completions`, mas a execução real da
          ferramenta precisa acontecer na sua aplicação. No `deepH`, isso é feito pelas skills.
        </p>
        <div className="section-grid">
          <ul className="subtle-list">
            {deepseekToolingResearch.findings.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
          <div className="panel">
            <h3>Links oficiais (DeepSeek)</h3>
            <div className="mini-link-list">
              {deepseekToolingResearch.sources.map((src) => (
                <a key={src.href} href={src.href} target="_blank" rel="noreferrer">
                  {src.label}
                </a>
              ))}
            </div>
            <div className="callout" style={{ marginTop: "0.8rem" }}>
              <strong>Conclusão:</strong> para construir/analisar código em um projeto real, use
              skills de filesystem no agent (`file_read_range`, `file_write_safe`). Tool calling da
              DeepSeek sozinho não escreve arquivos.
            </div>
          </div>
        </div>
      </section>
    </>
  );
}
