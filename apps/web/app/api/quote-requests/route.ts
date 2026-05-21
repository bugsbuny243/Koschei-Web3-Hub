import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";
import {
  calculateCompanyInfoComplete,
  calculateQuoteRequestStatus,
  normalizeFormCheckboxValue,
  normalizeFormTextValue,
  type QuoteRequestFormInput,
} from "@/lib/quote-requests";

function buildQuoteRequestInput(form: FormData): QuoteRequestFormInput {
  return {
    full_name: normalizeFormTextValue(form.get("full_name")),
    company_name: normalizeFormTextValue(form.get("company_name")),
    company_registration_status: normalizeFormTextValue(form.get("company_registration_status")),
    tax_number: normalizeFormTextValue(form.get("tax_number")),
    tax_office: normalizeFormTextValue(form.get("tax_office")),
    company_address: normalizeFormTextValue(form.get("company_address")),
    email: normalizeFormTextValue(form.get("email")),
    phone: normalizeFormTextValue(form.get("phone")),
    whatsapp: normalizeFormTextValue(form.get("whatsapp")),
    country: normalizeFormTextValue(form.get("country")),
    city: normalizeFormTextValue(form.get("city")),
    district: normalizeFormTextValue(form.get("district")),
    full_delivery_address: normalizeFormTextValue(form.get("full_delivery_address")),
    delivery_contact_name: normalizeFormTextValue(form.get("delivery_contact_name")),
    delivery_contact_phone: normalizeFormTextValue(form.get("delivery_contact_phone")),
    business_type: normalizeFormTextValue(form.get("business_type")),
    main_agricultural_activity: normalizeFormTextValue(form.get("main_agricultural_activity")),
    crop_types: normalizeFormTextValue(form.get("crop_types")),
    current_processing_capacity: normalizeFormTextValue(form.get("current_processing_capacity")),
    required_capacity_tph: normalizeFormTextValue(form.get("required_capacity_tph")),
    expected_daily_volume: normalizeFormTextValue(form.get("expected_daily_volume")),
    product_interest: normalizeFormTextValue(form.get("product_interest")),
    required_configuration_notes: normalizeFormTextValue(form.get("required_configuration_notes")),
    need_control_cabinet: normalizeFormCheckboxValue(form.get("need_control_cabinet")),
    need_fan_cyclone: normalizeFormCheckboxValue(form.get("need_fan_cyclone")),
    need_bucket_elevator: normalizeFormCheckboxValue(form.get("need_bucket_elevator")),
    need_spare_screen_sets: normalizeFormCheckboxValue(form.get("need_spare_screen_sets")),
    requested_screen_sets: normalizeFormTextValue(form.get("requested_screen_sets")),
    voltage_available: normalizeFormTextValue(form.get("voltage_available")),
    installation_location_type: normalizeFormTextValue(form.get("installation_location_type")),
    forklift_or_unloading_available: normalizeFormCheckboxValue(
      form.get("forklift_or_unloading_available"),
    ),
    expected_purchase_time: normalizeFormTextValue(form.get("expected_purchase_time")),
    quantity: normalizeFormTextValue(form.get("quantity")),
    message: normalizeFormTextValue(form.get("message")),
  };
}

export async function POST(req: Request) {
  const pool = getDbPool();
  if (!pool) {
    return NextResponse.json({ error: "Database is not configured." }, { status: 500 });
  }

  const form = await req.formData();
  const input = buildQuoteRequestInput(form);
  const companyInfoComplete = calculateCompanyInfoComplete(input);
  const status = calculateQuoteRequestStatus(companyInfoComplete);

  await pool.query(
    `INSERT INTO quote_requests (
      full_name, company_name, company_registration_status, tax_number, tax_office,
      company_address, email, phone, whatsapp, country,
      city, district, full_delivery_address, delivery_contact_name, delivery_contact_phone,
      business_type, main_agricultural_activity, crop_types, current_processing_capacity,
      required_capacity_tph, expected_daily_volume, product_interest, required_configuration_notes,
      need_control_cabinet, need_fan_cyclone, need_bucket_elevator, need_spare_screen_sets,
      requested_screen_sets, voltage_available, installation_location_type,
      forklift_or_unloading_available, expected_purchase_time, quantity, message,
      company_info_complete, status
    ) VALUES (
      $1, $2, $3, $4, $5,
      $6, $7, $8, $9, $10,
      $11, $12, $13, $14, $15,
      $16, $17, $18, $19,
      $20, $21, $22, $23,
      $24, $25, $26, $27,
      $28, $29, $30,
      $31, $32, $33, $34,
      $35, $36
    )`,
    [
      input.full_name,
      input.company_name,
      input.company_registration_status,
      input.tax_number,
      input.tax_office,
      input.company_address,
      input.email,
      input.phone,
      input.whatsapp,
      input.country,
      input.city,
      input.district,
      input.full_delivery_address,
      input.delivery_contact_name,
      input.delivery_contact_phone,
      input.business_type,
      input.main_agricultural_activity,
      input.crop_types,
      input.current_processing_capacity,
      input.required_capacity_tph,
      input.expected_daily_volume,
      input.product_interest,
      input.required_configuration_notes,
      input.need_control_cabinet,
      input.need_fan_cyclone,
      input.need_bucket_elevator,
      input.need_spare_screen_sets,
      input.requested_screen_sets,
      input.voltage_available,
      input.installation_location_type,
      input.forklift_or_unloading_available,
      input.expected_purchase_time,
      input.quantity,
      input.message,
      companyInfoComplete,
      status,
    ],
  );

  return NextResponse.redirect(new URL("/request-quote/thank-you", req.url), { status: 303 });
}
