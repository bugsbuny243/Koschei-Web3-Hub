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
        <p className="mt-4 text-sm text-slate-700">
          Secure payment will be arranged through Escrow.com after TradePi Globall confirms your quote and transaction details.
        </p>

      </section>
      <form className="card" action="/api/quote-requests" method="post">
        <h3>Customer / Company</h3>
        {['full_name','company_name','email','phone','whatsapp','country','city','district','full_delivery_address','company_registration_status','tax_number','website'].map((k)=><input key={k} name={k} placeholder={k.replaceAll('_',' ')} required={!['tax_number','website'].includes(k)} className="input" />)}
        <h3>Agriculture / Business</h3>
        <input name="business_type" placeholder="business_type" className="input" required />
        {['main_agricultural_activity','crop_types','current_processing_capacity','required_capacity_tph','working_hours_per_day','expected_daily_volume','material_moisture_note','impurity_problem_description','target_cleaning_result'].map((k)=><input key={k} name={k} placeholder={k.replaceAll('_',' ')} className="input" required />)}
        <h3>Machine Requirement</h3>
        <input name="product_interest" defaultValue="Fine Cleaner 5X-5" className="input" required />
        {['need_control_cabinet','need_fan_cyclone','need_bucket_elevator','need_spare_screen_sets'].map((k)=><label key={k}><input type="checkbox" name={k} /> {k}</label>)}
        {['requested_screen_sets','voltage_available','installation_location_type','available_space_dimensions','special_requirements'].map((k)=><input key={k} name={k} placeholder={k.replaceAll('_',' ')} className="input" required />)}
        <h3>Logistics</h3>
        <input name="preferred_trade_term" placeholder="EXW / FOB / CIF / DDP / Door-to-door / Not sure" className="input" required />
        {['destination_port','destination_city','destination_district','delivery_country','delivery_address_details'].map((k)=><input key={k} name={k} placeholder={k.replaceAll('_',' ')} className="input" required />)}
        {['forklift_or_unloading_available','customs_support_needed'].map((k)=><label key={k}><input type="checkbox" name={k} /> {k}</label>)}
        <h3>Commercial</h3>
        {['target_budget','expected_purchase_time','quantity','message'].map((k)=><input key={k} name={k} placeholder={k.replaceAll('_',' ')} className="input" required={k!=='target_budget'} />)}
        <label><input type="checkbox" name="consent" required /> I consent</label>
        <button className="btn btn-primary" type="submit">Submit RFQ</button>
      </form>
    </div>
  );
}
