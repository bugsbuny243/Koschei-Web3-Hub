import type { Metadata } from "next";
import Link from "next/link";
import "./globals.css";

export const metadata: Metadata = {
  title: "TradePi Globall Machinery",
  description:
    "Quote-based B2B machinery supply and RFQ workflow for agricultural processing equipment.",
};

const navItems = [
  { href: "/", label: "Ana Sayfa" },
  { href: "/products", label: "Ürünler" },
  { href: "/request-quote", label: "Teklif Al" },
  { href: "/about", label: "Hakkımızda" },
  { href: "/contact", label: "İletişim" },
];

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="tr">
      <body>
        <header className="site-header">
          <div className="container header-content">
            <Link href="/" className="brand">
              TradePi Globall Machinery
            </Link>
            <nav className="main-nav" aria-label="Ana navigasyon">
              {navItems.map((item) => (
                <Link key={item.href} href={item.href}>
                  {item.label}
                </Link>
              ))}
            </nav>
          </div>
        </header>

        <main className="site-main">{children}</main>

        <footer className="site-footer">
          <div className="container footer-content">
            <p className="footer-brand">TradePi Globall Machinery</p>
            <p>Quote-based B2B agricultural machinery supply.</p>
            <p>© 2026 TradePi Globall Machinery.</p>
          </div>
        </footer>
      </body>
    </html>
  );
}
