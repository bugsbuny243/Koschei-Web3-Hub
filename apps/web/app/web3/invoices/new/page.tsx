"use client";
import { useState } from "react";

export default function NewInvoicePage(){
  const [msg,setMsg]=useState("");
  async function onSubmit(formData: FormData){
    const body = Object.fromEntries(formData.entries());
    const res = await fetch('/api/web3/invoices',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(body)});
    const data = await res.json();
    setMsg(JSON.stringify(data));
  }
  return <main className="mx-auto max-w-xl p-6 space-y-3"><h1 className="text-2xl font-bold">Create Invoice</h1><form action={onSubmit} className="space-y-2">{[
    ['chain_slug','arbitrum-sepolia'],['stablecoin_symbol','USDC'],['stablecoin_contract',''],['receiver_address',''],['expected_amount',''],['due_at','']
  ].map(([n,d])=><input key={n} name={n} defaultValue={d} placeholder={n} className="w-full border rounded p-2"/>)}<button className="bg-black text-white rounded px-4 py-2">Create</button></form><pre>{msg}</pre></main>
}
