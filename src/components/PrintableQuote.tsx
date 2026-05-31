"use client";

/* eslint-disable @next/next/no-img-element */

import type { QuoteData } from "@/lib/types";
import { calculateTotal, HS_GTIP_DISCLAIMER, PLATFORM_DISCLAIMER } from "@/lib/quote";

const money = (amount: number, currency: string) => new Intl.NumberFormat("en-US", { style: "currency", currency }).format(amount);

export function PrintableQuote({ quote }: { quote: QuoteData }) {
  const { company, buyer, product } = quote;
  const total = calculateTotal(product);
  return <article className="printable-quote mx-auto min-h-[1120px] max-w-[800px] border border-slate-200 bg-white p-5 shadow-xl sm:p-12">
    <header className="avoid-print-break flex flex-col justify-between gap-5 border-b-2 border-slate-950 pb-7 sm:flex-row sm:items-start">
      <div className="flex items-start gap-4">{company.logoUrl ? <img src={company.logoUrl} alt={`${company.name} logo`} className="h-14 w-14 rounded-lg object-contain"/> : <span className="flex h-14 w-14 items-center justify-center rounded-lg bg-slate-950 text-lg font-black text-cyan-400">{company.name.slice(0,2).toUpperCase()}</span>}<div><h1 className="text-2xl font-black tracking-tight text-slate-950">{company.name}</h1><p className="mt-1 max-w-sm text-xs leading-5 text-slate-500">{company.address}</p><p className="mt-1 text-xs text-slate-500">{company.website}</p></div></div>
      <div className="sm:text-right"><p className="text-xl font-black tracking-widest text-slate-950">QUOTATION</p><p className="mt-2 text-xs font-bold text-cyan-700">{quote.quotationNumber}</p><p className="mt-1 text-xs text-slate-500">{new Date(quote.createdAt).toLocaleDateString("en-GB", { day: "2-digit", month: "long", year: "numeric" })}</p></div>
    </header>
    <section className="avoid-print-break grid gap-6 border-b border-slate-200 py-7 sm:grid-cols-2"><div><Label>Prepared for</Label><p className="mt-2 text-base font-black">{buyer.company}</p><p className="mt-1 text-sm text-slate-600">Attn: {buyer.contactName}</p><p className="text-sm text-slate-600">{buyer.country}</p><p className="text-sm text-slate-600">{buyer.email}</p></div><div><Label>Prepared by</Label><p className="mt-2 text-base font-black">{company.contactPerson}</p><p className="mt-1 text-sm text-slate-600">{company.email}</p><p className="text-sm text-slate-600">{company.phone}</p></div></section>
    <section className="avoid-print-break py-7"><h2 className="mb-4 text-sm font-black tracking-widest text-slate-950">COMMERCIAL OFFER</h2><div className="overflow-x-auto rounded-lg border border-slate-200"><table className="quote-table min-w-[620px] w-full text-left text-sm"><thead className="bg-slate-950 text-xs tracking-wide text-white"><tr><th className="px-4 py-3">PRODUCT</th><th className="px-4 py-3">QTY</th><th className="px-4 py-3 text-right">UNIT PRICE</th><th className="px-4 py-3 text-right">TOTAL</th></tr></thead><tbody><tr className="align-top"><td className="px-4 py-4"><strong>{product.name}</strong><p className="mt-1 text-xs leading-5 text-slate-500">{product.category}</p><p className="mt-2 text-xs leading-5 text-slate-500">{quote.productDescriptionEn}</p></td><td className="px-4 py-4">{product.quantity} {product.unit}</td><td className="px-4 py-4 text-right">{money(product.unitPrice, product.currency)}</td><td className="px-4 py-4 text-right font-bold">{money(total, product.currency)}</td></tr></tbody></table></div><div className="flex justify-end"><div className="mt-4 flex w-full max-w-xs justify-between rounded-lg bg-slate-100 px-4 py-3 text-base font-black"><span><span className="block">Total Amount</span><span className="mt-1 block text-[10px] font-bold text-slate-500">{product.quantity} {product.unit} × {money(product.unitPrice, product.currency)}</span></span><span>{money(total, product.currency)}</span></div></div></section>
    <section className="avoid-print-break grid gap-x-6 gap-y-4 rounded-lg bg-slate-50 p-5 text-sm sm:grid-cols-2"><Detail label="Incoterm" value={product.incoterm}/><Detail label="Delivery time" value={product.deliveryTime}/><Detail label="Payment terms" value={product.paymentTerms}/><Detail label="Offer validity" value={`${product.validityDays} days`}/><Detail label="Estimated HS/GTIP code" value={product.hsCode || "Not specified"}/><Detail label="Packaging" value={product.packagingDetails}/>{product.notes && <div className="sm:col-span-2"><Detail label="Additional notes" value={product.notes}/></div>}</section>
    <section className="py-7"><Label>Offer message</Label><p className="mt-3 whitespace-pre-line text-sm leading-6 text-slate-700">{quote.englishOfferText}</p></section>
    <section className="avoid-print-break pb-7"><Label>Export notes</Label><p className="mt-3 whitespace-pre-line text-xs leading-5 text-slate-600">{quote.exportNotes}</p></section>
    <footer className="space-y-2 border-t border-slate-200 pt-5 text-[10px] leading-4 text-slate-500"><p><strong>HS/GTIP disclaimer:</strong> {HS_GTIP_DISCLAIMER}</p><p><strong>Platform disclaimer:</strong> {PLATFORM_DISCLAIMER}</p></footer>
  </article>;
}
function Label({ children }: { children: React.ReactNode }) { return <p className="text-[10px] font-black uppercase tracking-widest text-cyan-700">{children}</p>; }
function Detail({ label, value }: { label: string; value: string }) { return <div><Label>{label}</Label><p className="mt-1 leading-5 text-slate-700">{value}</p></div>; }
