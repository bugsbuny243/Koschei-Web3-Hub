import Link from "next/link";

const items = ["code","web-builder","app-builder","game-builder","image","video","audio","projects"];

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  return <div className="container page-stack"><h1>The Immortal AI Platform</h1><nav className="main-nav">{items.map(i=><Link key={i} href={`/dashboard/${i}`}>{i}</Link>)}<Link href="/billing">billing</Link></nav>{children}</div>;
}
