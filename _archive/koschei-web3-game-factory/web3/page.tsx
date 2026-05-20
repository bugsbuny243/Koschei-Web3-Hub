export const runtime = "nodejs";
export const dynamic = "force-dynamic";

import Link from "next/link";
import { web3Db } from "@/lib/web3-db";

const warning = "Koschei PayWatch is a no-custody monitoring MVP. It does not hold funds, send transactions, manage private keys, provide escrow, or move user assets. It only monitors blockchain payment activity and records accounting events.";

export default async function Web3Dashboard() {
  try {
    const stats = await web3Db.query<{ total: string }>(`select count(*)::text as total from web3_invoices`);
    const pending = await web3Db.query<{ total: string }>(`select count(*)::text as total from web3_invoices where status = 'pending'`);
    const paid = await web3Db.query<{ total: string }>(`select count(*)::text as total from web3_invoices where status = 'paid'`);
    const detected = await web3Db.query<{ total: string }>(`select count(*)::text as total from web3_payment_events where status = 'detected'`);
    const matched = await web3Db.query<{ total: string }>(`select count(*)::text as total from web3_payment_events where status = 'matched'`);
    const paidAmount = await web3Db.query<{ total: string }>(`select coalesce(sum(expected_amount),0)::text as total from web3_invoices where status = 'paid'`);

    return <main className="mx-auto max-w-6xl space-y-4 p-6">
      <h1 className="text-3xl font-bold">Koschei Web3 Products Dashboard</h1>
      <p className="rounded-lg bg-violet-100 p-3 text-sm">Primary grant direction: <Link href="/web3/game-bridge/grant" className="underline">Koschei Web3 Game Bridge public demo</Link>.</p>
      <p className="rounded-lg bg-amber-100 p-3 text-sm">{warning}</p>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">{[
        ["total invoices", stats.rows[0]?.total ?? "0"], ["pending invoices", pending.rows[0]?.total ?? "0"], ["paid invoices", paid.rows[0]?.total ?? "0"], ["detected payments", detected.rows[0]?.total ?? "0"], ["matched payments", matched.rows[0]?.total ?? "0"], ["total paid amount", paidAmount.rows[0]?.total ?? "0"]
      ].map(([k, v]) => <div key={String(k)} className="rounded border p-4"><p className="text-sm text-gray-500">{k}</p><p className="text-2xl font-semibold">{v}</p></div>)}</div>
      <div className="flex flex-wrap gap-3">
        <Link href="/web3/game-bridge" className="underline">Game Bridge</Link>
        <Link href="/web3/game-bridge/grant" className="underline">Game Bridge Grant</Link>
        <Link href="/web3/game-bridge/plugin" className="underline">Game Bridge Plugin</Link>
        <Link href="/web3/game-bridge/items/new" className="underline">Game Bridge New Item</Link>
        <Link href="/web3/invoices" className="underline">PayWatch Invoices</Link>
        <Link href="/web3/testing" className="underline">PayWatch Testing</Link>
        <Link href="/web3/grant" className="underline">PayWatch Grant (Legacy)</Link>
      </div>
    </main>;
  } catch (error) {
    return <main className="mx-auto max-w-6xl space-y-4 p-6">
      <h1 className="text-3xl font-bold">Koschei Web3 Products Dashboard</h1>
      <p className="rounded-lg bg-violet-100 p-3 text-sm">Primary grant direction: <Link href="/web3/game-bridge/grant" className="underline">Koschei Web3 Game Bridge public demo</Link>.</p>
      <p className="rounded-lg bg-amber-100 p-3 text-sm">{warning}</p>
      <p className="rounded-lg bg-red-100 p-3 text-sm text-red-700">Failed to load dashboard data: {error instanceof Error ? error.message : "Unknown database error"}</p>
    </main>;
  }
}
