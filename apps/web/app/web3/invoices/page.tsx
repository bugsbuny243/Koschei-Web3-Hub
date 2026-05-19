export const dynamic = "force-dynamic";

import Link from "next/link";
import { web3Db } from "@/lib/web3-db";

const warning = "Koschei PayWatch is a no-custody monitoring MVP. It does not hold funds, send transactions, manage private keys, provide escrow, or move user assets. It only monitors blockchain payment activity and records accounting events.";

export default async function InvoicesPage(){
  try {
    const invoices = await web3Db.invoices.list();
    return <main className="mx-auto max-w-6xl p-6 space-y-4"><h1 className="text-2xl font-bold">Invoices</h1><p className="rounded-lg bg-amber-100 p-3 text-sm">{warning}</p><Link href="/web3/invoices/new" className="underline">Create Invoice</Link><div className="overflow-auto"><table className="w-full text-sm"><thead><tr className="text-left"><th>id</th><th>chain</th><th>token</th><th>expected amount</th><th>receiver</th><th>status</th><th>created</th><th>paid</th></tr></thead><tbody>{invoices.map((i) =><tr key={i.id} className="border-t"><td><Link className="underline" href={`/web3/invoices/${i.id}`}>{i.id.slice(0,8)}</Link></td><td>{i.chain_slug}</td><td>{i.stablecoin_symbol}</td><td>{i.expected_amount}</td><td>{i.receiver_address.slice(0,10)}...</td><td>{i.status}</td><td>{new Date(i.created_at).toLocaleString()}</td><td>{i.paid_at?new Date(i.paid_at).toLocaleString():"-"}</td></tr>)}</tbody></table></div></main>;
  } catch (error) {
    return <main className="mx-auto max-w-6xl p-6 space-y-4"><h1 className="text-2xl font-bold">Invoices</h1><p className="rounded-lg bg-amber-100 p-3 text-sm">{warning}</p><p className="rounded-lg bg-red-100 p-3 text-sm text-red-700">Failed to load invoices: {error instanceof Error ? error.message : "Unknown database error"}</p></main>;
  }
}
