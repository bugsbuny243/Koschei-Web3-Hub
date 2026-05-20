import Link from "next/link";

export default function Home() {
  return <main className="mx-auto max-w-6xl space-y-8 px-4 py-8 sm:px-6">
    <section className="rounded-xl border bg-white p-6">
      <h1 className="text-3xl font-bold">Grain Processing & Seed Cleaning Machinery from China to Global Buyers</h1>
      <p className="mt-3 text-slate-700">TradePi Globall Machinery is a B2B sourcing, dropshipping, and RFQ coordination platform for industrial machinery.</p>
      <div className="mt-5 flex gap-3">
        <Link href="/request-quote" className="rounded bg-slate-900 px-4 py-2 text-white">Request a Quote</Link>
        <Link href="/products" className="rounded border px-4 py-2">View Products</Link>
      </div>
    </section>
    <section className="grid gap-4 rounded-xl border bg-white p-6 md:grid-cols-2">
      <div>
        <h2 className="text-xl font-semibold">Why buyers choose this network</h2>
        <ul className="mt-3 list-disc space-y-2 pl-5 text-slate-700">
          <li>Manufacturer catalogue history since 1976.</li>
          <li>Grain cleaning and seed cleaning machinery.</li>
          <li>Flour mill and storage/silo equipment.</li>
          <li>Export-oriented machinery categories.</li>
        </ul>
      </div>
      <div className="rounded border bg-amber-50 p-4 text-sm text-slate-700">
        <h3 className="font-semibold">Safety & commercial note</h3>
        <p className="mt-2">TradePi Globall coordinates buyer inquiries, supplier quotation, documentation and dropshipping workflow. Final technical confirmation, freight, customs, taxes and delivery terms are quote-based and validated per destination/date.</p>
      </div>
    </section>
  </main>;
}
