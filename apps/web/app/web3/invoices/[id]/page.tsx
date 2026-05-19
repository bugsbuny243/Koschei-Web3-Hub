import { web3Db } from "@/lib/web3-db";

const warning = "Koschei PayWatch is a no-custody monitoring MVP. It does not hold funds, send transactions, manage private keys, provide escrow, or move user assets. It only monitors blockchain payment activity and records accounting events.";

type PageProps = {
  params: Promise<{ id: string }>
};

export const dynamic = "force-dynamic";

export default async function InvoiceDetail({ params }: PageProps){
  const { id } = await params;
  try {
    const invoice = await web3Db.query(`select * from web3_invoices where id = $1`, [id]);
    const events = await web3Db.query(`select * from web3_payment_events where invoice_id = $1 order by created_at desc`, [id]);
    const entries = await web3Db.query(`select * from web3_accounting_entries where invoice_id = $1 order by created_at desc`, [id]);
    if (!invoice.rows[0]) return <main className='p-6'>Not found</main>;
    return <main className="mx-auto max-w-5xl p-6 space-y-4"><h1 className="text-2xl font-bold">Invoice {id}</h1><p className="rounded-lg bg-amber-100 p-3 text-sm">{warning}</p><pre className="bg-gray-100 p-3 rounded overflow-auto">{JSON.stringify(invoice.rows[0],null,2)}</pre><h2 className="font-semibold">Payment Events</h2><pre className="bg-gray-100 p-3 rounded overflow-auto">{JSON.stringify(events.rows,null,2)}</pre><h2 className="font-semibold">Accounting Entries</h2><pre className="bg-gray-100 p-3 rounded overflow-auto">{JSON.stringify(entries.rows,null,2)}</pre></main>;
  } catch (error) {
    return <main className="mx-auto max-w-5xl p-6 space-y-4"><h1 className="text-2xl font-bold">Invoice {id}</h1><p className="rounded-lg bg-amber-100 p-3 text-sm">{warning}</p><p className="rounded-lg bg-red-100 p-3 text-sm text-red-700">Failed to load invoice details: {error instanceof Error ? error.message : "Unknown database error"}</p></main>;
  }
}
