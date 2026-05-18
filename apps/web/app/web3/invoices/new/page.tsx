"use client";
import { useState } from "react";
import { useRouter } from "next/navigation";

export default function NewInvoicePage(){
  const router = useRouter();
  const [msg,setMsg]=useState("");
  const [error, setError] = useState("");

  async function onSubmit(formData: FormData){
    setError("");
    setMsg("");

    const body = Object.fromEntries(formData.entries()) as Record<string, string>;
    const expectedAmount = Number(body.expected_amount);

    const requiredFields = ["chain_slug", "stablecoin_symbol", "stablecoin_contract", "receiver_address", "expected_amount"] as const;
    for (const field of requiredFields) {
      if (!body[field]?.toString().trim()) {
        setError(`${field} is required`);
        return;
      }
    }
    if (!Number.isFinite(expectedAmount) || expectedAmount <= 0) {
      setError("expected_amount must be a number greater than 0");
      return;
    }

    const payload: Record<string, unknown> = {
      ...body,
      expected_amount: expectedAmount,
      due_at: body.due_at?.trim() ? body.due_at : null
    };
    if (!body.due_at?.trim()) {
      delete payload.due_at;
    }

    try {
      const res = await fetch('/api/web3/invoices',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(payload)});
      const data = await res.json();

      if (!res.ok || !data?.ok) {
        setError(data?.error ?? "Failed to create invoice");
        return;
      }

      setMsg(JSON.stringify(data));
      const invoiceId = data?.invoice?.id;
      router.push(invoiceId ? `/web3/invoices/${invoiceId}` : "/web3/invoices");
    } catch {
      setError("Failed to create invoice. Please try again.");
    }
  }
  return <main className="mx-auto max-w-xl p-6 space-y-3"><h1 className="text-2xl font-bold">Create Invoice</h1><form action={onSubmit} className="space-y-2">{[
    ['chain_slug','arbitrum-sepolia'],['stablecoin_symbol','USDC'],['stablecoin_contract',''],['receiver_address',''],['expected_amount',''],['due_at','']
  ].map(([n,d])=><input key={n} name={n} defaultValue={d} placeholder={n} className="w-full border rounded p-2"/>)}<button className="bg-black text-white rounded px-4 py-2">Create</button></form>{error ? <p className="text-sm text-red-600" role="alert">{error}</p> : null}<pre>{msg}</pre></main>
}
