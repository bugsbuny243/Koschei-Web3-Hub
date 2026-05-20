export default function RequestQuotePage() {
  return (
    <div className="page-stack">
      <section>
        <p className="eyebrow">Request Quote</p>
        <h1>RFQ Intake - Fine Cleaner 5X-5</h1>
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
