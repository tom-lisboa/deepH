import Link from "next/link";
import { HelloWorldBrowser } from "@/components/hello-world-browser";

export default function HelloWorldsPage() {
  return (
    <>
      <section className="docs-section fade-up">
        <div className="section-kicker">Hello World Lab</div>
        <h2>Hello World Lab (workbench de onboarding)</h2>
        <p>
          Matriz de exemplos para aprender o `deepH` com baixo risco: escolha linguagem/estilo,
          copie prompt e comando, rode no seu agent/crew e compare o resultado.
        </p>
        <div className="callout" style={{ marginTop: "0.85rem" }}>
          <strong>Não é limite do deepH:</strong> Hello World aqui funciona como benchmark didático
          para treinar prompts, skills e orquestração antes de ir para calculadora fullstack ou
          repositório real.
        </div>
      </section>

      <section className="docs-section fade-up">
        <div className="section-kicker">Workbench</div>
        <h2>Matriz + painel de cópia</h2>
        <HelloWorldBrowser />
      </section>

      <section className="docs-section fade-up">
        <div className="section-kicker">Workflow</div>
        <h2>Fluxo de aprendizado (rápido)</h2>
        <div className="section-grid">
          <div className="panel">
            <h3>1. Escolha 1 linha da matriz</h3>
            <p>
              Filtre por linguagem/estilo e selecione um exemplo. Use o painel da direita para
              copiar prompt, comando e código de referência.
            </p>
          </div>
          <div className="panel">
            <h3>2. Rode no seu agent/crew</h3>
            <p>
              Troque <code>&lt;seu-agent-ou-spec&gt;</code> por um agent/crew seu. Se quiser gerar
              arquivos de verdade, habilite skills como `file_read_range` e `file_write_safe`.
            </p>
          </div>
          <div className="panel">
            <h3>3. Escale para projeto real</h3>
            <p>
              Depois leve a mesma mecânica para calculadora fullstack, seu `hello-world` real ou um
              repo maior. Aí vale adicionar crew/multiverso e judge.
            </p>
          </div>
        </div>
        <div className="mini-link-list" style={{ marginTop: "0.85rem" }}>
          <Link href="/docs/calculadora">Tutorial fullstack da calculadora</Link>
          <Link href="/docs#uso-em-projetos">Como usar em `hello-world` e projetos reais</Link>
          <Link href="/docs#customizacao">Criar seus próprios agents e skills</Link>
        </div>
      </section>
    </>
  );
}
