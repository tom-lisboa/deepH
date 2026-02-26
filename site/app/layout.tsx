import type { Metadata } from "next";
import "./globals.css";
import { SiteHeader } from "@/components/site-header";

export const metadata: Metadata = {
  title: "deepH | Typed Agent Runtime in Go",
  description:
    "Lightweight typed agent runtime in Go for user-defined agents, low-token orchestration, multiverse crews, and DeepSeek-first workflows."
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
