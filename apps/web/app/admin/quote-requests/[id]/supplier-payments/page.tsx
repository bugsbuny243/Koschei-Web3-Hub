import { customerQuotes } from "@/lib/payment-store";

export default async function SupplierPaymentsPage({ params, searchParams }: { params: Promise<{ id: string }>; searchParams: Promise<{ password?: string }> }) {
  const { id } = await params;
  const { password } = await searchParams;
  if (!process.env.ADMIN_PASSWORD || password !== process.env.ADMIN_PASSWORD) return <div className="page-stack"><h1>Admin Access Required</h1></div>;

  const quote = customerQuotes.find((q) => q.quoteRequestId === id);
  const total = quote?.supplierLandedCost ?? 0;
  return <div className="page-stack"><h1>Supplier Payments (Internal Manual Tracking)</h1><p>Supplier total landed cost: {total}</p><p>Expected 30% deposit: {(total * 0.3).toFixed(2)}</p><p>Expected 70% balance: {(total * 0.7).toFixed(2)}</p><p>Manual status updates and proof_file_url/private notes are managed internally only.</p></div>;
}
