import { DocsSidebar } from "@/components/docs-sidebar";
import { docsSidebarItems } from "@/lib/site-data";

export default function DocsLayout({
  children
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="docs-shell">
      <DocsSidebar title="Documentação deepH" items={docsSidebarItems} />
      <div className="docs-content">{children}</div>
    </div>
  );
}
