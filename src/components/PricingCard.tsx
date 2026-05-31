import { Button } from "@/components/Button";
import { Card } from "@/components/Card";

export function PricingCard({ name, price, description, features, featured = false }: { name: string; price: string; description: string; features: string[]; featured?: boolean }) {
  return (
    <Card className={`relative flex h-full flex-col p-7 ${featured ? "border-cyan-400 ring-2 ring-cyan-100" : ""}`}>
      {featured && <span className="absolute -top-3 right-5 rounded-full bg-cyan-500 px-3 py-1 text-xs font-black text-slate-950">EN POPÜLER</span>}
      <h3 className="text-lg font-black text-slate-950">{name}</h3>
      <p className="mt-2 min-h-10 text-sm leading-6 text-slate-500">{description}</p>
      <p className="mt-6 text-2xl font-black text-slate-950">{price}</p>
      <ul className="mt-6 flex-1 space-y-3 text-sm text-slate-600">
        {features.map((feature) => <li key={feature} className="flex gap-2"><span className="font-bold text-cyan-600">✓</span>{feature}</li>)}
      </ul>
      <Button href="/quote/new" variant={featured ? "primary" : "secondary"} className="mt-7 w-full">Hemen Başla</Button>
    </Card>
  );
}
