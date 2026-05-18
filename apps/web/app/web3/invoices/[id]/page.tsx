import { web3Db } from "@/lib/web3-db";
export default async function InvoiceDetail({params}:{params:{id:string}}){
  const invoice = await web3Db.query(`select * from web3_invoices where id = $1`, [params.id]);
  const events = await web3Db.query(`select * from web3_payment_events where invoice_id = $1 order by created_at desc`, [params.id]);
  const entries = await web3Db.query(`select * from web3_accounting_entries where invoice_id = $1 order by created_at desc`, [params.id]);
  if (!invoice.rows[0]) return <main className='p-6'>Not found</main>;
  return <main className="mx-auto max-w-5xl p-6 space-y-4"><h1 className="text-2xl font-bold">Invoice {params.id}</h1><pre className="bg-gray-100 p-3 rounded overflow-auto">{JSON.stringify(invoice.rows[0],null,2)}</pre><h2 className="font-semibold">Payment Events</h2><pre className="bg-gray-100 p-3 rounded overflow-auto">{JSON.stringify(events.rows,null,2)}</pre><h2 className="font-semibold">Accounting Entries</h2><pre className="bg-gray-100 p-3 rounded overflow-auto">{JSON.stringify(entries.rows,null,2)}</pre></main>;
}
