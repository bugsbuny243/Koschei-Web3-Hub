"use client";

import { useMemo, useState } from "react";
import { getAllMachineryProducts } from "@/lib/machinery-catalog";

type MediaRow = { id: string; product_slug: string; file_path: string; secure_url?: string | null; is_primary: boolean; alt_text?: string | null };

export default function OwnerMediaLibraryPage() {
  const products = useMemo(() => getAllMachineryProducts(), []);
    const [productSlug, setProductSlug] = useState(products[0]?.slug ?? "");
  const [altText, setAltText] = useState("");
  const [isPrimary, setIsPrimary] = useState(true);
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<string | null>(null);
  const [status, setStatus] = useState("");
  const [list, setList] = useState<MediaRow[]>([]);

  async function load() {
    const res = await fetch(`/api/owner/media/upload-product-image/list?product_slug=${encodeURIComponent(productSlug)}`);
    const data = await res.json();
    setList(data.media ?? []);
  }

  async function upload() {
    if (!file) return;
    setStatus("Uploading...");
    const form = new FormData();
        form.set("product_slug", productSlug);
    form.set("alt_text", altText);
    form.set("is_primary", String(isPrimary));
    form.set("file", file);
    const res = await fetch("/api/owner/media/upload-product-image", { method: "POST", body: form });
    const data = await res.json();
    setStatus(res.ok ? "Upload complete." : data.error ?? "Upload failed.");
    if (res.ok) await load();
  }

  return <main className="page-stack"><h1>Owner Media Library</h1><section className="card"><label>Product<select value={productSlug} onChange={(e)=>setProductSlug(e.target.value)}>{products.map((p)=><option key={p.slug} value={p.slug}>{p.name}</option>)}</select></label><label>Alt text<input value={altText} onChange={(e)=>setAltText(e.target.value)} /></label><label><input type="checkbox" checked={isPrimary} onChange={(e)=>setIsPrimary(e.target.checked)} /> Set as primary</label><label>File<input type="file" accept="image/*" onChange={(e)=>{const f=e.target.files?.[0]??null; setFile(f); setPreview(f?URL.createObjectURL(f):null);}} /></label>{preview ? <img src={preview} alt="preview" style={{maxWidth:320}}/>:null}<div className="hero-actions"><button className="btn btn-primary" type="button" onClick={upload}>Upload</button><button className="btn" type="button" onClick={load}>Refresh List</button></div><p>{status}</p></section><section className="card"><h2>Uploaded Images</h2>{list.length===0?<p>Makine görseli hazırlanıyor</p>:<ul>{list.map((m)=><li key={m.id}><a href={m.secure_url||m.file_path} target="_blank">{m.secure_url||m.file_path}</a> {m.is_primary?"(Primary)":""} <button type="button" onClick={async()=>{await fetch('/api/owner/media/set-primary',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({media_id:m.id})}); await load();}}>Set Primary</button> <button type="button" onClick={async()=>{await navigator.clipboard.writeText(m.secure_url||m.file_path); setStatus('URL copied');}}>Copy URL</button> <button type="button" onClick={async()=>{await fetch('/api/owner/media/delete-product-image',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({media_id:m.id})}); await load();}}>Delete</button></li>)}</ul>}</section></main>;
}

