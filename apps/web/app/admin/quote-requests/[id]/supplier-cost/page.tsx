export default async function Page({ searchParams, params }: { searchParams: Promise<{ password?: string }>; params: Promise<{ id: string }> }) {
  const { password } = await searchParams; const { id } = await params;
  if (password !== process.env.ADMIN_PASSWORD) return <div>Unauthorized</div>;
  return <form method="post" action={`/api/admin/quote-requests/${id}/supplier-cost?password=${password}`} className="page-stack card"><h1>Supplier Cost Input</h1>{["machine_cost","spare_parts_cost","packing_cost","inland_china_transport_cost","sea_freight_cost","destination_customs_cost","destination_tax_cost","destination_delivery_cost","other_cost","total_supplier_landed_cost","trade_term","destination","validity_date","supplier_notes"].map((k)=><input key={k} name={k} placeholder={k} className="input" />)}<button className="btn btn-primary">Save supplier cost</button></form>;
}
