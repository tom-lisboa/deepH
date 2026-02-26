import Link from "next/link";
import { CodeBlock } from "@/components/code-block";
import {
  architectureHighlights,
  comparisonMeta,
  comparisonRows,
  customizationGuide,
  deepseekToolingResearch,
  projectUsageGuide,
  projectPositioning,
  quickStartCommands,
  valuePillars
} from "@/lib/site-data";

export default function HomePage() {
  return (
    <div className="stack home-page">
      <section className="hero fade-up">
        <div className="hero-grid">
          <div>
            <div className="eyebrow">
              <span className="eyebrow-dot" />
              Go runtime + typed orchestration + DeepSeek-first
            </div>
            <h1 className="hero-title">
              Let&apos;s{" "}
              <span className="hero-go-word" aria-label="Go (Go language wordplay)">
                Go
              </span>{" "}
              Deeph.
            </h1>
            <p className="hero-headline-note">
              Universos colaboram um com o outro.
            </p>
            <p className="lead">{projectPositioning.subtitle}</p>
            <div className="hero-actions">
              <Link className="btn primary" href="/docs">
                Ler documentação completa
              </Link>
              <Link className="btn" href="/docs/calculadora">
                Tutorial: calculadora
              </Link>
              <Link className="btn" href="/docs/hello-worlds">
                Hello World Lab
              </Link>
              <Link className="btn" href="/docs/comparativo-claude-code">
                Comparativo com Claude Code
              </Link>
            </div>
            <div className="pill-list" style={{ marginTop: "0.9rem" }}>
              <span className="tag accent">Go</span>
              <span className="tag">Typed ports/channels</span>
              <span className="tag">Multiverse crews</span>
              <span className="tag warn">Low-token focus</span>
              <span className="tag">DeepSeek tools</span>
            </div>

            <div className="hero-rail" aria-label="Rotas rápidas para começar">
              <Link href="/docs/hello-worlds" className="hero-rail-card">
                <span className="hero-rail-label">Aprender</span>
                <strong>Hello World Lab</strong>
                <p>Escolha linguagem/estilo, copie prompt e rode com seu agent.</p>
                <code>deeph run "&lt;spec&gt;" "hello world ..."</code>
              </Link>
              <Link href="/docs/calculadora" className="hero-rail-card">
                <span className="hero-rail-label">Construir</span>
                <strong>Calculadora Fullstack</strong>
                <p>Step-by-step para Next + API route + skills de arquivo.</p>
                <code>planner&gt;backend+frontend&gt;reviewer</code>
              </Link>
              <Link href="/docs#customizacao" className="hero-rail-card">
                <span className="hero-rail-label">Customizar</span>
                <strong>Agents + Skills + Universos</strong>
                <p>Crie seus contratos, portas, channels e crews em YAML.</p>
                <code>agents/*.yaml · skills/*.yaml · crews/*.yaml</code>
              </Link>
            </div>
          </div>

          <div className="hero-aside">
            <div className="panel">
              <div className="section-kicker">Start</div>
              <h3>Quick Start (real)</h3>
              <p>Fluxo mínimo para validar o runtime localmente antes de partir para DeepSeek.</p>
              <div style={{ marginTop: "0.65rem" }}>
                <CodeBlock code={quickStartCommands.join("\n")} language="bash" />
              </div>
            </div>
            <div className="metric-grid">
              <div className="metric-card">
                <div className="label">Arquitetura</div>
                <div className="value">DAG + Channels</div>
              </div>
              <div className="metric-card">
                <div className="label">Contexto</div>
                <div className="value">type + moment</div>
              </div>
              <div className="metric-card">
                <div className="label">Multiverso</div>
                <div className="value">Crews + Judge</div>
              </div>
              <div className="metric-card">
                <div className="label">UX Terminal</div>
                <div className="value">Chat + Coach</div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="grid-2">
        <div className="section-card fade-up">
          <div className="section-kicker">Why deepH</div>
          <h2>Por que alguém vai usar o deepH?</h2>
          <p>
            Porque ele não tenta ser só “mais um chat CLI”. O foco é virar um runtime
            de agentes tipado em Go, com custo/token sob controle, orquestração observável
            e liberdade para o usuário criar seus próprios agents.
          </p>
          <div className="stack" style={{ marginTop: "0.7rem" }}>
            {valuePillars.map((pillar) => (
              <div key={pillar.title} className="panel">
                <div style={{ display: "flex", justifyContent: "space-between", gap: "0.6rem" }}>
                  <h3>{pillar.title}</h3>
                  <span className="tag">{pillar.tag}</span>
                </div>
                <p>{pillar.body}</p>
              </div>
            ))}
          </div>
        </div>

        <div className="section-card fade-up">
          <div className="section-kicker">Pitch</div>
          <h2>Pitch rápido</h2>
          <p>
            <strong>deepH</strong> é um runtime leve de agentes em Go para workflows
            definidos pelo usuário, com ports/channels tipados, budgets de contexto/tools
            e multiverso com crews.
          </p>
          <hr className="divider" />
          <h3>Se você quer…</h3>
          <ul className="subtle-list">
            <li>controlar custo/token com arquitetura (não só modelo)</li>
            <li>montar pipelines multi-agent reprodutíveis</li>
            <li>tipar handoffs e fazer merge por porta</li>
            <li>testar universos alternativos e reconciliar com judge</li>
          </ul>
          <p style={{ marginTop: "0.8rem" }}>
            …o deepH está no caminho certo para isso.
          </p>
          <div className="mini-link-list">
            <Link href="/docs">Docs completas</Link>
            <Link href="/docs/calculadora">Tutorial da calculadora (step-by-step)</Link>
            <Link href="/docs/comparativo-claude-code">
              Comparativo com Claude Code ({comparisonMeta.date})
            </Link>
          </div>
        </div>
      </section>

      <section className="grid-2">
        <div className="section-card fade-up">
          <div className="section-kicker">Create</div>
          <h2>{customizationGuide.title}</h2>
          <p>{customizationGuide.summary}</p>
          <div className="stack" style={{ marginTop: "0.8rem" }}>
            {customizationGuide.agentSteps.map((step, idx) => (
              <div key={step.title} className="panel">
                <div className="tag accent">Agents · passo {idx + 1}</div>
                <h3 style={{ marginTop: "0.45rem" }}>{step.title}</h3>
                <p>{step.text}</p>
                {idx === 0 ? (
                  <div style={{ marginTop: "0.6rem" }}>
                    <CodeBlock code={step.code} language="bash" />
                  </div>
                ) : null}
              </div>
            ))}
            <div className="panel">
              <div className="tag warn">Skills · passo a passo</div>
              <h3 style={{ marginTop: "0.45rem" }}>{customizationGuide.skillSteps[0].title}</h3>
              <p>{customizationGuide.skillSteps[0].text}</p>
              <div style={{ marginTop: "0.6rem" }}>
                <CodeBlock code={customizationGuide.skillSteps[0].code} language="yaml" />
              </div>
            </div>
          </div>
          <div style={{ marginTop: "0.8rem" }}>
            <Link className="btn" href="/docs#customizacao">
              Ver passo a passo completo
            </Link>
          </div>
        </div>

        <div className="section-card fade-up">
          <div className="section-kicker">Use in projects</div>
          <h2>{projectUsageGuide.title}</h2>
          <p>{projectUsageGuide.summary}</p>
          <div className="stack" style={{ marginTop: "0.8rem" }}>
            {projectUsageGuide.modes.map((mode, idx) => (
              <div key={mode.title} className="panel">
                <h3>{mode.title}</h3>
                <p>{mode.body}</p>
                {idx === 0 ? (
                  <div style={{ marginTop: "0.6rem" }}>
                    <CodeBlock code={mode.code} language="bash" />
                  </div>
                ) : null}
              </div>
            ))}
          </div>
          <div className="mini-link-list" style={{ marginTop: "0.8rem" }}>
            <Link href="/docs/hello-worlds">Catálogo de Hello Worlds (filtros + estilos)</Link>
            <Link href="/docs#uso-em-projetos">Como usar dentro de hello-world</Link>
            <Link href="/docs/calculadora">Tutorial fullstack da calculadora</Link>
          </div>
        </div>
      </section>

      <section className="grid-3">
        <div className="section-card fade-up">
          <div className="section-kicker">Orchestration</div>
          <h3>Orquestração que importa</h3>
          <p>
            `depends_on_ports`, `channel_priority`, `merge_policy`, budgets por stage e por
            channel, e agora channels entre universos (`u1.result -&gt; u3.context`).
          </p>
        </div>
        <div className="section-card fade-up">
          <div className="section-kicker">Token economy</div>
          <h3>Token economy real</h3>
          <p>
            `ContextCompiler`, scoring por tipo/momento, `file_read_range`, `artifact/ref`,
            publish budget e anti-loop de tools. O foco é menos replay, mais sinal.
          </p>
        </div>
        <div className="section-card fade-up">
          <div className="section-kicker">Terminal UX</div>
          <h3>UX terminal evolutiva</h3>
          <p>
            `chat` com histórico persistente, `session show/list`, coach semântico local e
            dicionários de `command`/`type` com JSON para UI/automação.
          </p>
        </div>
      </section>

      <section className="grid-2">
        <div className="section-card fade-up">
          <div className="section-kicker">Architecture</div>
          <h2>Arquitetura (resumo)</h2>
          <ul className="subtle-list">
            {architectureHighlights.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        </div>

        <div className="section-card fade-up">
          <div className="section-kicker">Compare</div>
          <h2>Comparativo rápido com Claude Code</h2>
          <table className="split-table">
            <thead>
              <tr>
                <th>Tema</th>
                <th>deepH</th>
                <th>Claude Code</th>
              </tr>
            </thead>
            <tbody>
              {comparisonRows.slice(0, 3).map((row) => (
                <tr key={row.topic}>
                  <td>{row.topic}</td>
                  <td>{row.deeph}</td>
                  <td>{row.claude}</td>
                </tr>
              ))}
            </tbody>
          </table>
          <p className="footnote">
            Comparativo completo e datado em{" "}
            <Link href="/docs/comparativo-claude-code">
              /docs/comparativo-claude-code
            </Link>
            .
          </p>
        </div>
      </section>

      <section className="grid-2">
        <div className="section-card fade-up">
          <div className="section-kicker">DeepSeek tools</div>
          <h2>{deepseekToolingResearch.title}</h2>
          <ul className="subtle-list" style={{ marginTop: "0.8rem" }}>
            {deepseekToolingResearch.findings.map((f) => (
              <li key={f}>{f}</li>
            ))}
          </ul>
          <p className="footnote">
            Resumo prático: DeepSeek faz <em>tool calling</em>, mas quem executa a ação real é o
            `deepH` via skills.
          </p>
        </div>
        <div className="section-card fade-up">
          <div className="section-kicker">Research links</div>
          <h2>Links oficiais (pesquisa)</h2>
          <div className="mini-link-list">
            {deepseekToolingResearch.sources.map((src) => (
              <a key={src.href} href={src.href} target="_blank" rel="noreferrer">
                {src.label}
              </a>
            ))}
          </div>
          <div className="callout" style={{ marginTop: "0.8rem" }}>
            <strong>Para gerar código de verdade:</strong> habilite skills de filesystem (
            <code className="code-inline">file_write_safe</code>,{" "}
            <code className="code-inline">file_read_range</code>) no agent.
          </div>
        </div>
      </section>
    </div>
  );
}
