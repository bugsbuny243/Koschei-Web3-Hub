import Link from "next/link";

const ownerLinks = [
  { href: "/owner/command-center", label: "Dashboard" },
  { href: "/owner/command-center", label: "RFQ Inbox" },
  { href: "/owner/supplier-outreach", label: "Supplier Outreach" },
  { href: "/owner/media-library", label: "Media Library" },
  { href: "/owner/command-center", label: "Quotes" },
  { href: "/owner/command-center", label: "Escrow" },
  { href: "/owner/login", label: "Logout" },
];

export default function OwnerLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="owner-shell">
      <aside className="owner-sidebar">
        <div>
          <p className="owner-logo">TradePi Owner</p>
          <p className="owner-subtitle">Operasyon Paneli</p>
        </div>
        <nav className="owner-nav" aria-label="Owner navigasyon">
          {ownerLinks.map((item) => (
            <Link key={`${item.href}-${item.label}`} href={item.href}>
              {item.label}
            </Link>
          ))}
        </nav>
      </aside>
      <section className="owner-content">{children}</section>
    </div>
  );
}
