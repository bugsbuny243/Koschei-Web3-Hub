export default function RequestQuotePage() {
  return (
    <div className="page-stack">
      <section>
        <p className="eyebrow">Request Quote</p>
        <h1>Quote-based RFQ</h1>
        <p>Final price confirmed per official proforma invoice.</p>
      </section>

      <section className="card">
        <form className="grid gap-4 md:grid-cols-2">
          <label className="flex flex-col gap-2">
            target_crop_types
            <input name="target_crop_types" type="text" className="rounded-xl border border-slate-300 px-3 py-2" />
          </label>
          <label className="flex flex-col gap-2">
            required_screen_sets
            <input name="required_screen_sets" type="text" className="rounded-xl border border-slate-300 px-3 py-2" />
          </label>

          <label className="flex items-center gap-2">
            <input name="need_control_cabinet" type="checkbox" /> need_control_cabinet
          </label>
          <label className="flex items-center gap-2">
            <input name="need_fan_cyclone" type="checkbox" /> need_fan_cyclone
          </label>
          <label className="flex items-center gap-2">
            <input name="need_bucket_elevator" type="checkbox" /> need_bucket_elevator
          </label>

          <label className="flex flex-col gap-2">
            delivery_city
            <input name="delivery_city" type="text" className="rounded-xl border border-slate-300 px-3 py-2" />
          </label>
          <label className="flex flex-col gap-2">
            delivery_country
            <input name="delivery_country" type="text" className="rounded-xl border border-slate-300 px-3 py-2" />
          </label>

          <label className="flex flex-col gap-2">
            preferred_delivery_term
            <select name="preferred_delivery_term" className="rounded-xl border border-slate-300 px-3 py-2">
              <option>EXW</option><option>FOB</option><option>CIF</option><option>DDP</option><option>Not sure</option>
            </select>
          </label>

          <label className="flex flex-col gap-2">
            company_registration_status
            <select name="company_registration_status" className="rounded-xl border border-slate-300 px-3 py-2">
              <option>Registered</option><option>In progress</option><option>Individual buyer</option><option>Not sure</option>
            </select>
          </label>
        </form>

        <p className="mt-4 text-sm text-slate-600">
          Freight and DDP door-to-door cost can change depending on destination, shipment date and customs/tax conditions.
        </p>
        <p className="mt-4 text-sm text-slate-700">
          TradePi Globall coordinates B2B sourcing and quotation workflow. TradePi Globall does not display fixed public prices for heavy machinery. Final price, freight, customs, taxes, payment terms and delivery terms are confirmed per official quotation / proforma invoice.
        </p>
      </section>
    </div>
  );
}
