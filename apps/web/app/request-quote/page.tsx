export default function RequestQuotePage() {
  return (
    <div className="container page-stack">
      <section className="card">
        <p className="eyebrow">Request a Quote</p>
        <h1>Public RFQ Form</h1>
        <p>Submit your requirements and our team will review your request.</p>
      </section>

      <form className="card rfq-form" action="/api/quote-requests" method="post">
        <h2>Company & Contact</h2>
        <div className="form-grid">
          <label>Full Name<input name="full_name" required /></label>
          <label>Company Name<input name="company_name" required /></label>
          <label>Company Registration Status<input name="company_registration_status" required /></label>
          <label>Tax Number<input name="tax_number" required /></label>
          <label>Tax Office<input name="tax_office" required /></label>
          <label className="full-width">Company Address<textarea name="company_address" rows={3} required /></label>
          <label>Email<input type="email" name="email" required /></label>
          <label>Phone<input name="phone" required /></label>
          <label>WhatsApp<input name="whatsapp" /></label>
          <label>Country<input name="country" required /></label>
          <label>City<input name="city" required /></label>
          <label>District<input name="district" /></label>
          <label className="full-width">Full Delivery Address<textarea name="full_delivery_address" rows={3} required /></label>
          <label>Delivery Contact Name<input name="delivery_contact_name" /></label>
          <label>Delivery Contact Phone<input name="delivery_contact_phone" /></label>
        </div>

        <h2>Operational Requirements</h2>
        <div className="form-grid">
          <label>Business Type<input name="business_type" /></label>
          <label>Main Agricultural Activity<input name="main_agricultural_activity" /></label>
          <label>Crop Types<input name="crop_types" /></label>
          <label>Current Processing Capacity<input name="current_processing_capacity" /></label>
          <label>Required Capacity (TPH)<input name="required_capacity_tph" /></label>
          <label>Expected Daily Volume<input name="expected_daily_volume" /></label>
          <label>Product Interest<input name="product_interest" /></label>
          <label className="full-width">Required Configuration Notes<textarea name="required_configuration_notes" rows={3} /></label>
          <label className="checkbox"><input type="checkbox" name="need_control_cabinet" /> Need Control Cabinet</label>
          <label className="checkbox"><input type="checkbox" name="need_fan_cyclone" /> Need Fan Cyclone</label>
          <label className="checkbox"><input type="checkbox" name="need_bucket_elevator" /> Need Bucket Elevator</label>
          <label className="checkbox"><input type="checkbox" name="need_spare_screen_sets" /> Need Spare Screen Sets</label>
          <label>Requested Screen Sets<input name="requested_screen_sets" /></label>
          <label>Voltage Available<input name="voltage_available" /></label>
          <label>Installation Location Type<input name="installation_location_type" /></label>
          <label className="checkbox"><input type="checkbox" name="forklift_or_unloading_available" /> Forklift / Unloading Available</label>
          <label>Expected Purchase Time<input name="expected_purchase_time" /></label>
          <label>Quantity<input name="quantity" /></label>
          <label className="full-width">Message<textarea name="message" rows={4} /></label>
        </div>

        <button className="btn btn-primary" type="submit">Submit RFQ</button>
      </form>
    </div>
  );
}
