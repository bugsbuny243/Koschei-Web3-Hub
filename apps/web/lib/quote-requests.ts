export type QuoteRequestFormInput = {
  full_name: string | null;
  company_name: string | null;
  company_registration_status: string | null;
  tax_number: string | null;
  tax_office: string | null;
  company_address: string | null;
  email: string | null;
  phone: string | null;
  whatsapp: string | null;
  country: string | null;
  city: string | null;
  district: string | null;
  full_delivery_address: string | null;
  delivery_contact_name: string | null;
  delivery_contact_phone: string | null;
  business_type: string | null;
  main_agricultural_activity: string | null;
  crop_types: string | null;
  current_processing_capacity: string | null;
  required_capacity_tph: string | null;
  expected_daily_volume: string | null;
  product_interest: string | null;
  required_configuration_notes: string | null;
  need_control_cabinet: boolean;
  need_fan_cyclone: boolean;
  need_bucket_elevator: boolean;
  need_spare_screen_sets: boolean;
  requested_screen_sets: string | null;
  voltage_available: string | null;
  installation_location_type: string | null;
  forklift_or_unloading_available: boolean;
  expected_purchase_time: string | null;
  quantity: string | null;
  message: string | null;
};

const REQUIRED_COMPANY_INFO_FIELDS: Array<keyof QuoteRequestFormInput> = [
  "company_name",
  "company_registration_status",
  "tax_number",
  "tax_office",
  "company_address",
  "country",
  "city",
  "full_delivery_address",
];

export function normalizeFormTextValue(value: FormDataEntryValue | null): string | null {
  if (typeof value !== "string") {
    return null;
  }
  const trimmed = value.trim();
  return trimmed.length ? trimmed : null;
}

export function normalizeFormCheckboxValue(value: FormDataEntryValue | null): boolean {
  return value === "on";
}

export function calculateCompanyInfoComplete(input: QuoteRequestFormInput): boolean {
  return REQUIRED_COMPANY_INFO_FIELDS.every((field) => Boolean(input[field]));
}

export function calculateQuoteRequestStatus(companyInfoComplete: boolean): string {
  return companyInfoComplete ? "new" : "incomplete_company_info";
}
