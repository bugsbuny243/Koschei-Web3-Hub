"use client";
import { useState } from "react";

export default function TestingPage(){
  const [out,setOut]=useState('');
  async function submit(path:string,formData:FormData,header:string){
    const res = await fetch(path,{method:'POST',headers:{'content-type':'application/json',[header]:String(formData.get('secret')||'')},body:JSON.stringify(Object.fromEntries(formData.entries()))});
    setOut(JSON.stringify(await res.json(),null,2));
  }
  return <main className="mx-auto max-w-2xl p-6 space-y-4"><h1 className="text-2xl font-bold">MVP Testing</h1><p className="bg-red-100 rounded p-3 text-sm">This page is only for MVP/testing.</p>
  <form action={(fd)=>submit('/api/web3/payment-events/manual',fd,'x-webhook-secret')} className="space-y-2 border p-3 rounded"><h2 className='font-semibold'>Manual Payment Event</h2>{['secret','chain_slug','tx_hash','from_address','to_address','token_contract','token_symbol','token_decimals','amount'].map(n=><input key={n} name={n} placeholder={n} defaultValue={n==='chain_slug'?'arbitrum-sepolia':n==='token_symbol'?'USDC':n==='token_decimals'?'6':''} className='w-full border rounded p-2'/>) }<button className='px-3 py-1 bg-black text-white rounded'>Send</button></form>
  <form action={(fd)=>submit('/api/web3/scan/arbitrum-sepolia',fd,'x-cron-secret')} className="space-y-2 border p-3 rounded"><h2 className='font-semibold'>Scan Arbitrum Sepolia</h2>{['secret','token_contract','token_symbol','token_decimals','from_block','to_block'].map(n=><input key={n} name={n} placeholder={n} defaultValue={n==='token_symbol'?'USDC':n==='token_decimals'?'6':''} className='w-full border rounded p-2'/>) }<button className='px-3 py-1 bg-black text-white rounded'>Scan</button></form><pre>{out}</pre></main>
}
