import type { Metadata } from "next";
import "./globals.css";
import Link from "next/link";

export const metadata: Metadata = {
  title: "KOSCEI Dropshop",
  description: "Modern dropshipping mağazası",
};

const navItems = [
  { href: "/", label: "Ana Sayfa" },
  { href: "/products", label: "Ürünler" },
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
              KOSCEI DROPSHOP
            </Link>
            <nav className="main-nav">
              {navItems.map((item) => (
                <Link key={item.href} href={item.href}>
                  {item.label}
                </Link>
              ))}
            </nav>
          </div>
        </header>
        <main className="container">{children}</main>
        <footer className="site-footer">
          <div className="container">© 2026 KOSCEI Dropshop. Tüm hakları saklıdır.</div>
        </footer>
      </body>
    </html>
  );
}
