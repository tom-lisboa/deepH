import Link from "next/link";
import { CodeBlock } from "@/components/code-block";
import { calculatorTutorial } from "@/lib/site-data";

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
