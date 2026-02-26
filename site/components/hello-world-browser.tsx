"use client";

import { useEffect, useState } from "react";
import { CodeBlock } from "@/components/code-block";
import {
  helloWorldCategoryLabels,
  helloWorldEntries,
  helloWorldLanguages,
  helloWorldStyleLabels,
  type HelloWorldCategory,
  type HelloWorldEntry,
  type HelloWorldStyle
} from "@/lib/hello-worlds";

type CategoryFilter = "all" | HelloWorldCategory;
type StyleFilter = "all" | HelloWorldStyle;

function optionClass(active: boolean) {
  return active ? "filter-btn is-active" : "filter-btn";
}

function buildRunCommand(entry: HelloWorldEntry) {
  const spec = entry.deephSpec ? `"${entry.deephSpec}"` : '"<seu-agent-ou-spec>"';
  return `deeph run ${spec} "${entry.deephPrompt}"`;
}

function codeLanguage(entry: HelloWorldEntry) {
  const raw = entry.language.toLowerCase();
  if (raw === "c++") return "cpp";
  if (raw === "c#") return "csharp";
  return raw;
}

export function HelloWorldBrowser() {
  const [category, setCategory] = useState<CategoryFilter>("all");
  const [style, setStyle] = useState<StyleFilter>("all");
  const [language, setLanguage] = useState<string>("all");
  const [selectedId, setSelectedId] = useState<string>(helloWorldEntries[0]?.id ?? "");
  const [copiedKey, setCopiedKey] = useState<string>("");
  const langs = helloWorldLanguages();

  const filtered = helloWorldEntries.filter((entry) => {
    if (category !== "all" && entry.category !== category) return false;
    if (style !== "all" && entry.style !== style) return false;
    if (language !== "all" && entry.language !== language) return false;
    return true;
  });

  const selected = filtered.find((entry) => entry.id === selectedId) ?? filtered[0] ?? null;

  useEffect(() => {
    if (!filtered.length) return;
    if (!filtered.some((entry) => entry.id === selectedId)) {
      setSelectedId(filtered[0].id);
    }
  }, [filtered, selectedId]);

  async function copyText(kind: "prompt" | "command" | "code" | "files", text: string) {
    if (typeof navigator === "undefined" || !navigator.clipboard) return;
    try {
      await navigator.clipboard.writeText(text);
      const key = `${selected?.id ?? "none"}:${kind}`;
      setCopiedKey(key);
      window.setTimeout(() => {
        setCopiedKey((current) => (current === key ? "" : current));
      }, 1200);
    } catch {
      // no-op; page remains usable without clipboard permission
    }
  }

  function copiedLabel(kind: "prompt" | "command" | "code" | "files") {
    const key = `${selected?.id ?? "none"}:${kind}`;
    return copiedKey === key ? "Copiado" : null;
  }

  return (
    <div className="lab-shell">
      <section className="lab-panel">
        <div className="lab-panel-head">
          <div>
            <h3>Hello World Lab</h3>
            <p className="lab-panel-copy">
              Benchmark de onboarding. Escolha um alvo, copie prompt/comando e evolua para o seu
              projeto real.
            </p>
          </div>
          <div className="lab-stats">
            <div className="lab-stat">
              <span>resultados</span>
              <strong>{filtered.length}</strong>
            </div>
            <div className="lab-stat">
              <span>linguagens</span>
              <strong>{langs.length}</strong>
            </div>
          </div>
        </div>

        <div className="filter-group lab-filters">
          <div className="filter-row" role="toolbar" aria-label="Filtrar por categoria">
            {(Object.keys(helloWorldCategoryLabels) as Array<keyof typeof helloWorldCategoryLabels>).map(
              (key) => (
                <button
                  key={key}
                  type="button"
                  className={optionClass(category === key)}
                  onClick={() => setCategory(key as CategoryFilter)}
                >
                  {helloWorldCategoryLabels[key]}
                </button>
              )
            )}
          </div>
          <div className="filter-row" role="toolbar" aria-label="Filtrar por estilo">
            {(Object.keys(helloWorldStyleLabels) as Array<keyof typeof helloWorldStyleLabels>).map(
              (key) => (
                <button
                  key={key}
                  type="button"
                  className={optionClass(style === key)}
                  onClick={() => setStyle(key as StyleFilter)}
                >
                  {helloWorldStyleLabels[key]}
                </button>
              )
            )}
          </div>
          <div className="filter-row wrap" role="toolbar" aria-label="Filtrar por linguagem">
            <button
              type="button"
              className={optionClass(language === "all")}
              onClick={() => setLanguage("all")}
            >
              Todas linguagens
            </button>
            {langs.map((lang) => (
              <button
                key={lang}
                type="button"
                className={optionClass(language === lang)}
                onClick={() => setLanguage(lang)}
              >
                {lang}
              </button>
            ))}
          </div>
        </div>
      </section>

      <div className="lab-layout">
        <section className="lab-list-panel" aria-label="Matriz de exemplos Hello World">
          <div className="lab-table-wrap">
            <table className="lab-table">
              <thead>
                <tr>
                  <th>Exemplo</th>
                  <th>Linguagem</th>
                  <th>Categoria</th>
                  <th>Estilo</th>
                  <th>Arquivos</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((entry) => {
                  const active = selected?.id === entry.id;
                  return (
                    <tr
                      key={entry.id}
                      className={active ? "is-active" : undefined}
                      onClick={() => setSelectedId(entry.id)}
                    >
                      <td>
                        <button
                          type="button"
                          className={active ? "lab-row-link is-active" : "lab-row-link"}
                          onClick={(event) => {
                            event.stopPropagation();
                            setSelectedId(entry.id);
                          }}
                        >
                          <span className="lab-row-title">{entry.title}</span>
                          <span className="lab-row-subtitle">{entry.summary}</span>
                        </button>
                      </td>
                      <td>
                        <div className="lab-cell-stack">
                          <span>{entry.language}</span>
                          {entry.framework ? (
                            <span className="lab-cell-muted">{entry.framework}</span>
                          ) : null}
                        </div>
                      </td>
                      <td>{helloWorldCategoryLabels[entry.category]}</td>
                      <td>{helloWorldStyleLabels[entry.style]}</td>
                      <td>{entry.files.length}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
            {!filtered.length ? (
              <div className="lab-empty">Nenhum exemplo encontrado com os filtros atuais.</div>
            ) : null}
          </div>
        </section>

        <aside className="lab-detail-panel" aria-label="Painel de detalhes e cópia">
          {selected ? (
            <>
              <div className="lab-detail-head">
                <div>
                  <h3>{selected.title}</h3>
                  <p>
                    {selected.language}
                    {selected.framework ? ` · ${selected.framework}` : ""} ·{" "}
                    {helloWorldCategoryLabels[selected.category]} ·{" "}
                    {helloWorldStyleLabels[selected.style]}
                  </p>
                </div>
                <div className="pill-list">
                  <span className="tag accent">{selected.language}</span>
                  <span className="tag">{helloWorldStyleLabels[selected.style]}</span>
                </div>
              </div>

              <p className="lab-detail-summary">{selected.summary}</p>

              <div className="lab-copy-actions" role="group" aria-label="Ações de cópia">
                <button
                  type="button"
                  className="copy-btn"
                  onClick={() => copyText("prompt", selected.deephPrompt)}
                >
                  Copiar prompt {copiedLabel("prompt") ? <span>{copiedLabel("prompt")}</span> : null}
                </button>
                <button
                  type="button"
                  className="copy-btn"
                  onClick={() => copyText("command", buildRunCommand(selected))}
                >
                  Copiar comando {copiedLabel("command") ? <span>{copiedLabel("command")}</span> : null}
                </button>
                <button
                  type="button"
                  className="copy-btn"
                  onClick={() => copyText("files", selected.files.join("\n"))}
                >
                  Copiar arquivos {copiedLabel("files") ? <span>{copiedLabel("files")}</span> : null}
                </button>
                <button
                  type="button"
                  className="copy-btn"
                  onClick={() => copyText("code", selected.code)}
                >
                  Copiar código {copiedLabel("code") ? <span>{copiedLabel("code")}</span> : null}
                </button>
              </div>

              <div className="lab-detail-block">
                <div className="lab-detail-label">Arquivos esperados</div>
                <ul className="lab-file-list">
                  {selected.files.map((file) => (
                    <li key={file}>
                      <code>{file}</code>
                    </li>
                  ))}
                </ul>
              </div>

              <div className="lab-detail-block">
                <div className="lab-detail-label">Prompt (deepH)</div>
                <CodeBlock code={selected.deephPrompt} language="text" />
              </div>

              <div className="lab-detail-block">
                <div className="lab-detail-label">Comando sugerido</div>
                <CodeBlock code={buildRunCommand(selected)} language="bash" />
                {!selected.deephSpec ? (
                  <p className="lab-detail-note">
                    Troque <code>&lt;seu-agent-ou-spec&gt;</code> por um agent ou crew seu.
                  </p>
                ) : null}
              </div>

              <div className="lab-detail-block">
                <div className="lab-detail-label">Código (exemplo)</div>
                <CodeBlock code={selected.code} language={codeLanguage(selected)} />
              </div>

              <div className="lab-detail-block">
                <div className="lab-detail-label">Resultado esperado</div>
                <p className="lab-detail-note">{selected.expectedOutput}</p>
              </div>
            </>
          ) : (
            <div className="lab-empty">
              Nenhum exemplo encontrado. Ajuste os filtros para ver entradas disponíveis.
            </div>
          )}
        </aside>
      </div>
    </div>
  );
}
