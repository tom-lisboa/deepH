type CodeBlockProps = {
  code: string;
  language?: string;
};

export function CodeBlock({ code, language }: CodeBlockProps) {
  return (
    <div className="code" aria-label={language ? `${language} code block` : "code block"}>
      <pre>
        <code>{code}</code>
      </pre>
    </div>
  );
}
