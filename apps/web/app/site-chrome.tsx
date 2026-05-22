"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

const navItems = [
  { href: "/koschei", label: "Koschei" },
];

export function SiteChrome({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const isOwner = pathname?.startsWith("/owner");
  if (isOwner) return <>{children}</>;

  return (
    <>
      <header className="site-header">
        <div className="container header-content">
          <Link href="/koschei" className="brand">
            Koschei AI
          </Link>
          <nav className="main-nav" aria-label="Main navigation">
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
          <p className="footer-brand">Koschei AI</p>
          <p>AI super-app platform.</p>
          <p>© 2026 Koschei AI.</p>
        </div>
      </footer>
    </>
  );
}
