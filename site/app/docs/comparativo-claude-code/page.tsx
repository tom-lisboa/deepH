import Link from "next/link";
import {
  claudeCodeSources,
  comparisonMeta,
  comparisonRows,
  comparisonUseCases
} from "@/lib/site-data";

export default function ClaudeCodeComparisonPage() {
  return (
    <>
      <section className="docs-section fade-up">
        <div className="section-kicker">Positioning</div>
        <h2>{comparisonMeta.title}</h2>
        <p>{comparisonMeta.disclaimer}</p>
        <div className="callout" style={{ marginTop: "0.85rem" }}>
          <strong>Data do snapshot:</strong> {comparisonMeta.date}. O objetivo é ajudar no
          posicionamento do `deepH`, não “ganhar discussão” contra outra ferramenta.
        </div>
      </section>

      <section className="docs-section fade-up">
        <div className="section-kicker">Matrix</div>
        <h2>Matriz de comparação</h2>
        <div className="section-grid">
          <table className="split-table">
            <thead>
              <tr>
                <th>Tema</th>
                <th>deepH</th>
                <th>Claude Code</th>
              </tr>
            </thead>
            <tbody>
              {comparisonRows.map((row) => (
                <tr key={row.topic}>
                  <td>{row.topic}</td>
                  <td>{row.deeph}</td>
                  <td>{row.claude}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="docs-section fade-up">
        <div className="section-kicker">Use cases</div>
        <h2>Quando usar cada um</h2>
        <div className="grid-3" style={{ marginTop: "0.85rem" }}>
          <div className="panel">
            <h3>Escolha deepH</h3>
            <ul className="subtle-list">
              {comparisonUseCases.chooseDeeph.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </div>
          <div className="panel">
            <h3>Escolha Claude Code</h3>
            <ul className="subtle-list">
              {comparisonUseCases.chooseClaudeCode.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </div>
          <div className="panel">
            <h3>Use os dois</h3>
            <ul className="subtle-list">
              {comparisonUseCases.together.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </div>
        </div>
      </section>

      <section className="docs-section fade-up">
        <div className="section-kicker">Messaging</div>
        <h2>Como posicionar o deepH (mensagem de produto)</h2>
        <div className="section-grid">
          <div className="panel">
            <h3>Mensagem curta</h3>
            <p>
              <strong>deepH</strong> é um runtime tipado de agentes em Go para workflows
              definidos pelo usuário, com foco em orquestração, baixo custo de token e
              experimentação multiverso.
            </p>
          </div>
          <div className="panel">
            <h3>Mensagem para comunidade técnica</h3>
            <p>
              Se Claude Code é uma ótima experiência pronta de coding agent, o deepH quer ser
              a camada onde você modela o seu sistema de agentes: ports, channels, handoffs,
              budgets, crews e judge — com controle fino de custo e comportamento.
            </p>
          </div>
          <div className="panel">
            <h3>Referências oficiais usadas</h3>
            <ul className="subtle-list">
              {claudeCodeSources.map((src) => (
                <li key={src.href}>
                  <a href={src.href} target="_blank" rel="noreferrer">
                    {src.label}
                  </a>
                </li>
              ))}
            </ul>
          </div>
        </div>
        <p className="footnote">
          Você também pode usar esta página como base para README/landing pública do projeto.
        </p>
      </section>

      <section className="docs-section fade-up">
        <div className="section-kicker">Next</div>
        <h2>Próximos passos sugeridos</h2>
        <div className="mini-link-list">
          <Link href="/docs">Voltar para docs completas</Link>
          <Link href="/docs/calculadora">Fazer o tutorial da calculadora</Link>
          <Link href="/">Ver a landing page</Link>
        </div>
      </section>
    </>
  );
}
