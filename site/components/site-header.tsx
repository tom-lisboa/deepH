import Link from "next/link";

export function SiteHeader() {
  return (
    <header className="top-nav">
      <Link href="/" className="brand" aria-label="deepH home">
        <span className="brand-mark">dH</span>
        <span>deepH</span>
      </Link>
      <nav className="top-nav-links" aria-label="Main navigation">
        <Link href="/">Landing</Link>
        <Link href="/docs">Docs</Link>
        <Link href="/docs/hello-worlds">Hello World Lab</Link>
        <Link href="/docs/calculadora">Tutorial</Link>
        <Link href="/docs/comparativo-claude-code">Claude Code</Link>
        <a
          className="cta"
          href="https://api-docs.deepseek.com/"
          target="_blank"
          rel="noreferrer"
        >
          DeepSeek Docs
        </a>
      </nav>
    </header>
  );
}
