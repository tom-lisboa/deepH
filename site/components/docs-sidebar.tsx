import Link from "next/link";

export type DocsSidebarItem = {
  href: string;
  label: string;
};

export function DocsSidebar({
  title,
  items
}: {
  title: string;
  items: DocsSidebarItem[];
}) {
  return (
    <aside className="docs-sidebar">
      <div className="section-kicker">Docs map</div>
      <h3>{title}</h3>
      <nav>
        {items.map((item) => (
          <Link key={item.href} href={item.href}>
            {item.label}
          </Link>
        ))}
      </nav>
    </aside>
  );
}
