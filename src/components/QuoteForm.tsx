"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent, type InputHTMLAttributes, type TextareaHTMLAttributes } from "react";
import { Button } from "@/components/Button";
import { Card } from "@/components/Card";
import { generateEnglishOfferText, generateFollowUpMessage, generateQuotationNumber, getLatestQuoteFromLocalStorage, saveQuoteToLocalStorage } from "@/lib/quote";
import type { BuyerInfo, CompanyInfo, ProductInfo } from "@/lib/types";

const defaultCompany: CompanyInfo = { name: "", contactPerson: "", email: "", phone: "", address: "", website: "", logoUrl: "" };
const defaultBuyer: BuyerInfo = { company: "", contactName: "", country: "", email: "" };
const defaultProduct: ProductInfo = { name: "", category: "", descriptionTr: "", quantity: 1, unit: "pcs", unitPrice: 0, currency: "USD", incoterm: "EXW", deliveryTime: "2-3 weeks after order confirmation", paymentTerms: "50% advance payment, 50% before shipment", validityDays: 15, hsCode: "", packagingDetails: "", notes: "" };

function Field({ label, ...props }: InputHTMLAttributes<HTMLInputElement> & { label: string }) { return <label className="block"><span className="mb-2 block text-sm font-bold text-slate-700">{label}</span><input {...props} className="w-full rounded-xl border border-slate-200 bg-white px-4 py-3 text-sm outline-none focus:border-cyan-500 focus:ring-2 focus:ring-cyan-100" /></label>; }
function Area({ label, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement> & { label: string }) { return <label className="block"><span className="mb-2 block text-sm font-bold text-slate-700">{label}</span><textarea {...props} className="min-h-24 w-full rounded-xl border border-slate-200 bg-white px-4 py-3 text-sm outline-none focus:border-cyan-500 focus:ring-2 focus:ring-cyan-100" /></label>; }
function Select({ label, value, options, onChange }: { label: string; value: string; options: string[]; onChange: (value: string) => void }) { return <label className="block"><span className="mb-2 block text-sm font-bold text-slate-700">{label}</span><select value={value} onChange={(event) => onChange(event.target.value)} className="w-full rounded-xl border border-slate-200 bg-white px-4 py-3 text-sm outline-none focus:border-cyan-500">{options.map((option) => <option key={option}>{option}</option>)}</select></label>; }

export function QuoteForm() {
  const router = useRouter();
  const [company, setCompany] = useState(defaultCompany);
  const [buyer, setBuyer] = useState(defaultBuyer);
  const [product, setProduct] = useState(defaultProduct);
  useEffect(() => { if (new URLSearchParams(window.location.search).get("edit") !== "latest") return; const latest = getLatestQuoteFromLocalStorage(); if (latest) { setCompany(latest.company); setBuyer(latest.buyer); setProduct(latest.product); } }, []);
  const companyField = (key: keyof CompanyInfo) => ({ value: company[key] ?? "", onChange: (event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => setCompany({ ...company, [key]: event.target.value }) });
  const buyerField = (key: keyof BuyerInfo) => ({ value: buyer[key], onChange: (event: React.ChangeEvent<HTMLInputElement>) => setBuyer({ ...buyer, [key]: event.target.value }) });
  const productField = (key: keyof ProductInfo) => ({ value: product[key] ?? "", onChange: (event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => setProduct({ ...product, [key]: event.target.value }) });
  const numberField = (key: "quantity" | "unitPrice" | "validityDays") => ({ value: product[key], min: key === "unitPrice" ? 0 : 1, step: key === "unitPrice" ? "0.01" : "1", onChange: (event: React.ChangeEvent<HTMLInputElement>) => setProduct({ ...product, [key]: Number(event.target.value) }) });

  function submit(event: FormEvent) {
    event.preventDefault();
    const quote = { quotationNumber: generateQuotationNumber(), createdAt: new Date().toISOString(), company, buyer, product, englishOfferText: generateEnglishOfferText(company, buyer, product), followUpMessage: generateFollowUpMessage(buyer, product) };
    saveQuoteToLocalStorage(quote);
    router.push("/quote/preview");
  }

  return <form onSubmit={submit} className="space-y-6">
    <Card className="p-6"><SectionTitle number="01" title="Firma bilgileri" copy="Teklifi gönderen şirketin kurumsal bilgileri."/><div className="mt-6 grid gap-4 md:grid-cols-2"><Field label="Firma adı" required {...companyField("name")}/><Field label="Yetkili kişi" required {...companyField("contactPerson")}/><Field label="E-posta" type="email" required {...companyField("email")}/><Field label="Telefon / WhatsApp" required {...companyField("phone")}/><Field label="Web sitesi" placeholder="https://" required {...companyField("website")}/><Field label="Logo URL (opsiyonel)" placeholder="https://" {...companyField("logoUrl")}/><div className="md:col-span-2"><Area label="Adres" required {...companyField("address")}/></div></div></Card>
    <Card className="p-6"><SectionTitle number="02" title="Alıcı bilgileri" copy="Teklifin ulaşacağı müşteri ve ülke bilgileri."/><div className="mt-6 grid gap-4 md:grid-cols-2"><Field label="Alıcı firma" required {...buyerField("company")}/><Field label="Alıcı yetkili adı" required {...buyerField("contactName")}/><Field label="Alıcı ülke" required {...buyerField("country")}/><Field label="Alıcı e-posta" type="email" required {...buyerField("email")}/></div></Card>
    <Card className="p-6"><SectionTitle number="03" title="Ürün ve ticari koşullar" copy="Fiyatlandırma ve teslimat için gereken temel detaylar."/><div className="mt-6 grid gap-4 md:grid-cols-2"><Field label="Ürün adı" required {...productField("name")}/><Field label="Ürün kategorisi" required {...productField("category")}/><div className="md:col-span-2"><Area label="Türkçe ürün açıklaması" required {...productField("descriptionTr")}/></div><Field label="Miktar" type="number" required {...numberField("quantity")}/><Field label="Birim" placeholder="pcs, kg, box..." required {...productField("unit")}/><Field label="Birim fiyat" type="number" required {...numberField("unitPrice")}/><Select label="Para birimi" value={product.currency} options={["USD", "EUR", "TRY"]} onChange={(currency) => setProduct({ ...product, currency: currency as ProductInfo["currency"] })}/><Select label="Incoterm" value={product.incoterm} options={["EXW", "FOB", "CIF", "DAP"]} onChange={(incoterm) => setProduct({ ...product, incoterm: incoterm as ProductInfo["incoterm"] })}/><Field label="Teslimat süresi" required {...productField("deliveryTime")}/><Field label="Ödeme koşulları" required {...productField("paymentTerms")}/><Field label="Geçerlilik süresi (gün)" type="number" required {...numberField("validityDays")}/></div></Card>
    <Card className="p-6"><SectionTitle number="04" title="İhracat yardımcı bilgileri" copy="Gümrük ve paketleme görüşmelerinde kullanabileceğiniz ek alanlar."/><div className="mt-6 grid gap-4 md:grid-cols-2"><Field label="Tahmini HS / GTIP kodu (opsiyonel)" {...productField("hsCode")}/><Field label="Paketleme detayları" required {...productField("packagingDetails")}/><div className="md:col-span-2"><Area label="Notlar" placeholder="Teklife eklemek istediğiniz ilave notlar..." {...productField("notes")}/></div></div></Card>
    <div className="flex flex-col items-end gap-3"><p className="text-right text-xs leading-5 text-slate-500">Devam ederek bilgilerin bu tarayıcıda yerel olarak saklanmasını kabul edersiniz.</p><Button type="submit" className="w-full sm:w-auto">İngilizce Teklifi Hazırla <span className="ml-1">→</span></Button></div>
  </form>;
}
function SectionTitle({ number, title, copy }: { number: string; title: string; copy: string }) { return <div className="flex gap-4"><span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-slate-950 text-xs font-black text-cyan-400">{number}</span><div><h2 className="text-lg font-black">{title}</h2><p className="mt-1 text-sm text-slate-500">{copy}</p></div></div>; }
