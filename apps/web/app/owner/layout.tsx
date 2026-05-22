"use client";

import Link from "next/link";
import { useState } from "react";

const ownerLinks = [
  { href: "/owner/command-center", label: "Komuta Merkezi" },
  { href: "/owner/command-center#rfq-inbox", label: "Teklif Talepleri" },
  { href: "/owner/supplier-outreach", label: "Tedarikçi Arama" },
  { href: "/owner/media-library", label: "Medya Kütüphanesi" },
  { href: "/owner/command-center", label: "Teklifler" },
  { href: "/owner/command-center", label: "Escrow" },
  { href: "/owner/login", label: "Çıkış" },
];

export default function OwnerLayout({ children }: { children: React.ReactNode }) {
  const [menuOpen, setMenuOpen] = useState(false);

  return (
    <div className="owner-shell">
      <header className="owner-mobile-header">
        <div>
          <p className="owner-logo">TradePi Owner</p>
          <p className="owner-subtitle">Operasyon Paneli</p>
        </div>
        <button className="btn btn-secondary owner-menu-button" type="button" onClick={() => setMenuOpen((v) => !v)} aria-expanded={menuOpen} aria-controls="owner-nav">
          ☰ Menü
        </button>
      </header>
      <aside className={`owner-sidebar ${menuOpen ? "open" : ""}`}>
        <div>
          <p className="owner-logo">TradePi Owner</p>
          <p className="owner-subtitle">Operasyon Paneli</p>
        </div>
        <nav id="owner-nav" className="owner-nav" aria-label="Owner navigasyon">
          {ownerLinks.map((item) => (
            <Link key={`${item.href}-${item.label}`} href={item.href} onClick={() => setMenuOpen(false)}>
              {item.label}
            </Link>
          ))}
        </nav>
      </aside>
      {menuOpen ? <button className="owner-overlay" type="button" aria-label="Menüyü kapat" onClick={() => setMenuOpen(false)} /> : null}
      <section className="owner-content">{children}</section>
    </div>
  );
}
