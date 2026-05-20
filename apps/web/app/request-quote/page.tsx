export default function RequestQuotePage() {
  return (
    <div className="container page-stack">
      <section className="card">
        <p className="eyebrow">Teklif Al</p>
        <h1>RFQ Submission Form</h1>
        <p>
          Submit your operational and delivery requirements for Fine Cleaner 5X-5 and related
          configuration review.
        </p>
      </section>

      <form className="card rfq-form" action="/api/quote-requests" method="post">
        <h2>Customer / Company</h2>
        <div className="form-grid">
          <label>Full Name<input name="full_name" required /></label>
          <label>Company Name<input name="company_name" required /></label>
          <label>Email<input type="email" name="email" required /></label>
          <label>Phone<input name="phone" required /></label>
          <label>WhatsApp<input name="whatsapp" /></label>
          <label>Country<input name="country" required /></label>
          <label>City<input name="city" required /></label>
          <label>District<input name="district" /></label>
          <label className="full-width">Full Delivery Address<textarea name="full_delivery_address" rows={3} /></label>
          <label>Company Registration Status<input name="company_registration_status" /></label>
        </div>

        <h2>Business / Agriculture</h2>
        <div className="form-grid">
          <label>Business Type<input name="business_type" /></label>
          <label>Main Agricultural Activity<input name="main_agricultural_activity" /></label>
          <label>Crop Types<input name="crop_types" required /></label>
          <label>Current Processing Capacity<input name="current_processing_capacity" /></label>
          <label>Required Capacity (TPH)<input name="required_capacity_tph" required /></label>
          <label>Working Hours Per Day<input name="working_hours_per_day" /></label>
          <label>Expected Daily Volume<input name="expected_daily_volume" /></label>
          <label className="full-width">Impurity Problem Description<textarea name="impurity_problem_description" rows={3} /></label>
          <label className="full-width">Target Cleaning Result<textarea name="target_cleaning_result" rows={3} /></label>
        </div>

        <h2>Machine</h2>
        <div className="form-grid">
          <label>Product Interest<input name="product_interest" defaultValue="Fine Cleaner 5X-5" required /></label>
          <label className="checkbox"><input type="checkbox" name="need_control_cabinet" /> Need Control Cabinet</label>
          <label className="checkbox"><input type="checkbox" name="need_fan_cyclone" /> Need Fan + Cyclone</label>
          <label className="checkbox"><input type="checkbox" name="need_bucket_elevator" /> Need Bucket Elevator</label>
          <label className="checkbox"><input type="checkbox" name="need_spare_screen_sets" /> Need Spare Screen Sets</label>
          <label>Requested Screen Sets<input name="requested_screen_sets" /></label>
          <label>Voltage Available<input name="voltage_available" placeholder="e.g. 380V 50Hz 3 phase" /></label>
          <label>Installation Location Type<input name="installation_location_type" /></label>
          <label>Available Space Dimensions<input name="available_space_dimensions" /></label>
          <label className="full-width">Special Requirements<textarea name="special_requirements" rows={3} /></label>
        </div>

        <h2>Logistics</h2>
        <div className="form-grid">
          <label>Preferred Trade Term<input name="preferred_trade_term" /></label>
          <label>Delivery Country<input name="delivery_country" required /></label>
          <label>Destination City<input name="destination_city" required /></label>
          <label>Destination District<input name="destination_district" /></label>
          <label className="full-width">Delivery Address Details<textarea name="delivery_address_details" rows={3} /></label>
          <label className="checkbox"><input type="checkbox" name="forklift_or_unloading_available" /> Forklift / Unloading Available</label>
          <label className="checkbox"><input type="checkbox" name="customs_support_needed" /> Customs Support Needed</label>
        </div>

        <h2>Commercial</h2>
        <div className="form-grid">
          <label>Expected Purchase Time<input name="expected_purchase_time" /></label>
          <label>Quantity<input name="quantity" /></label>
          <label className="full-width">Message<textarea name="message" rows={4} /></label>
        </div>

        <p className="muted-note">
          AI supports RFQ completeness checks and supplier-message drafting only. AI does not set
          price, guarantee delivery, or decide DDP responsibility. Final terms come only from the
          supplier’s official written quotation/proforma.
        </p>

        <button className="btn btn-primary" type="submit">
          RFQ Gönder
        </button>
      </form>
    </div>
  );
}
