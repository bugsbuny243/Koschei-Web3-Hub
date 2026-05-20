export default function HomePage() {
  return (
    <main className="mx-auto max-w-5xl px-4 py-10 md:px-8">
      <section className="rounded-2xl bg-slate-900 p-8 text-white md:p-12">
        <p className="text-sm uppercase tracking-[0.2em] text-sky-300">TradePi Globall Machinery</p>
        <h1 className="mt-3 text-3xl font-bold md:text-5xl">Quote-based B2B Machinery Supply</h1>
        <p className="mt-5 max-w-3xl text-slate-200">
          Current verified public product: Fine Cleaner 5X-5. Public listing is quote-based only.
        </p>
        <div className="mt-6 flex gap-3">
          <a href="/products" className="rounded-xl bg-white px-4 py-2 text-sm font-semibold text-slate-900">Request Quote</a>
          <a href="/request-quote" className="rounded-xl border border-slate-200 px-4 py-2 text-sm font-semibold text-white">Quote-based</a>
        </div>
      </section>

      <section className="mt-10 rounded-2xl border border-slate-200 bg-white p-6">
        <h2 className="text-2xl font-semibold text-slate-900">Public pricing rule</h2>
        <p className="mt-3 text-slate-700">Final price confirmed per official proforma invoice.</p>
      </section>
    </main>
  );
}
