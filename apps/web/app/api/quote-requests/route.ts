import { NextResponse } from "next/server";
import { getDbPool } from "@/lib/db";

const bool = (v: FormDataEntryValue | null) => v === "on";
const text = (f: FormData, k: string) => (f.get(k) as string) || null;

export async function POST(req: Request) {
  const form = await req.formData();
  const pool = getDbPool();
  if (pool) {
    const q = `INSERT INTO quote_requests (full_name,company_name,email,phone,whatsapp,country,city,district,full_delivery_address,company_registration_status,tax_number,website,business_type,main_agricultural_activity,crop_types,current_processing_capacity,required_capacity_tph,working_hours_per_day,expected_daily_volume,material_moisture_note,impurity_problem_description,target_cleaning_result,product_interest,need_control_cabinet,need_fan_cyclone,need_bucket_elevator,need_spare_screen_sets,requested_screen_sets,voltage_available,installation_location_type,available_space_dimensions,special_requirements,preferred_trade_term,destination_port,destination_city,destination_district,delivery_country,delivery_address_details,forklift_or_unloading_available,customs_support_needed,target_budget,expected_purchase_time,quantity,message,status)
VALUES (${Array.from({ length: 45 }, (_, i) => `$${i + 1}`).join(",")})`;
    const vals = [text(form,"full_name"),text(form,"company_name"),text(form,"email"),text(form,"phone"),text(form,"whatsapp"),text(form,"country"),text(form,"city"),text(form,"district"),text(form,"full_delivery_address"),text(form,"company_registration_status"),text(form,"tax_number"),text(form,"website"),text(form,"business_type"),text(form,"main_agricultural_activity"),text(form,"crop_types"),text(form,"current_processing_capacity"),text(form,"required_capacity_tph"),text(form,"working_hours_per_day"),text(form,"expected_daily_volume"),text(form,"material_moisture_note"),text(form,"impurity_problem_description"),text(form,"target_cleaning_result"),text(form,"product_interest"),bool(form.get("need_control_cabinet")),bool(form.get("need_fan_cyclone")),bool(form.get("need_bucket_elevator")),bool(form.get("need_spare_screen_sets")),text(form,"requested_screen_sets"),text(form,"voltage_available"),text(form,"installation_location_type"),text(form,"available_space_dimensions"),text(form,"special_requirements"),text(form,"preferred_trade_term"),text(form,"destination_port"),text(form,"destination_city"),text(form,"destination_district"),text(form,"delivery_country"),text(form,"delivery_address_details"),bool(form.get("forklift_or_unloading_available")),bool(form.get("customs_support_needed")),text(form,"target_budget"),text(form,"expected_purchase_time"),text(form,"quantity"),text(form,"message"),"new"];
    await pool.query(q, vals);
  }
  return NextResponse.redirect(new URL("/request-quote/received", req.url));
}
