import type { Metadata } from "next";
import "./globals.css";
import { SiteHeader } from "@/components/site-header";

export const metadata: Metadata = {
  title: "deepH | Working-Set CLI for Code Operations",
  description:
    "deepH turns any codebase into a focused working set for diagnose, edit and review by selecting diffs, symbols, tests and typed handoffs instead of replaying the whole repo."
};

export default function RootLayout({
  children
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="pt-BR">
      <body>
        <div className="site-shell">
          <div className="site-backdrop" aria-hidden="true" />
          <SiteHeader />
          <main className="site-main">{children}</main>
        </div>
      </body>
    </html>
  );
}
