const products = [
  {
    name: "Koschei Starter Pack",
    subtitle: "Web3 Project Output",
    price: "899 TL",
    description: "For builders who want to turn one Web3 idea into structured metadata, risk checklist and launch copy.",
    features: [
      "1 Web3 project or asset metadata output",
      "JSON metadata format",
      "Risk & Trust checklist",
      "Launch page text draft",
      "No-custody workflow guidance",
    ],
    button: "Buy Starter Pack",
    href: "https://www.shopier.com/TradeVisual/47465449",
  },
  {
    name: "Koschei Builder Pack",
    subtitle: "Metadata + Risk + Launch",
    price: "2.299 TL",
    description: "For Web3 builders who need stronger project presentation and structured outputs.",
    features: [
      "3 Web3 project or asset outputs",
      "AI-assisted metadata descriptions",
      "Game asset / NFT metadata JSON",
      "Risk & Trust Scanner report",
      "Ecosystem-focused project summary",
    ],
    button: "Buy Builder Pack",
    href: "https://www.shopier.com/TradeVisual/47465484",
    featured: true,
  },
  {
    name: "Koschei Studio Pack",
    subtitle: "Web3 Builder Studio",
    price: "4.999 TL",
    description: "For teams, agencies and high-volume Web3 builders.",
    features: [
      "10 Web3 project or asset outputs",
      "AI Metadata Studio outputs",
      "Game Asset Builder JSON files",
      "Risk & Trust checklist reports",
      "ChainOps / Alchemy testnet guidance",
      "Builder-ready documentation draft",
    ],
    button: "Buy Studio Pack",
    href: "https://www.shopier.com/TradeVisual/47465499",
  },
];

export function Web3PricingSection({ className = "" }: { className?: string }) {
  return (
    <section id="products" className={`border-y border-white/10 bg-slate-950/45 ${className}`}>
      <div className="mx-auto max-w-7xl px-5 py-20 lg:px-8">
        <div className="max-w-3xl">
          <p className="eyebrow">Builder output packages</p>
          <h2 className="mt-4 text-3xl font-black tracking-tight text-white sm:text-4xl">Choose your Web3 builder pack.</h2>
          <p className="mt-4 leading-7 text-slate-400">Premium, structured project outputs for builders moving from an idea to ecosystem-ready presentation materials.</p>
        </div>

        <div className="mt-10 grid gap-5 lg:grid-cols-3">
          {products.map((product) => (
            <article
              key={product.name}
              className={`relative flex h-full flex-col overflow-hidden rounded-2xl border p-6 shadow-2xl shadow-black/20 sm:p-7 ${
                product.featured
                  ? "border-violet-400/70 bg-gradient-to-b from-violet-500/20 via-slate-900/95 to-slate-950 ring-1 ring-violet-400/40"
                  : "border-white/10 bg-gradient-to-b from-slate-900/90 to-slate-950/95"
              }`}
            >
              {product.featured && (
                <span className="absolute right-5 top-5 rounded-full border border-violet-300/30 bg-violet-400/15 px-3 py-1 text-[.65rem] font-black uppercase tracking-[.14em] text-violet-200">
                  Most popular
                </span>
              )}
              <div className={product.featured ? "pr-28" : ""}>
                <p className="text-xs font-black uppercase tracking-[.14em] text-blue-300">{product.subtitle}</p>
                <h3 className="mt-3 text-xl font-black text-white">{product.name}</h3>
              </div>
              <p className="mt-6 text-4xl font-black tracking-tight text-white">{product.price}</p>
              <p className="mt-5 min-h-20 text-sm leading-6 text-slate-400">{product.description}</p>
              <ul className="mt-6 flex-1 space-y-3 border-t border-white/10 pt-6 text-sm leading-5 text-slate-300">
                {product.features.map((feature) => (
                  <li key={feature} className="flex gap-3">
                    <span className="mt-0.5 text-blue-400" aria-hidden="true">✓</span>
                    <span>{feature}</span>
                  </li>
                ))}
              </ul>
              <a
                href={product.href}
                target="_blank"
                rel="noopener noreferrer"
                className={`mt-8 w-full ${product.featured ? "web3-button" : "web3-button-secondary"}`}
              >
                {product.button} →
              </a>
            </article>
          ))}
        </div>

        <p className="mt-8 rounded-xl border border-blue-400/15 bg-blue-500/5 px-5 py-4 text-xs leading-6 text-slate-400">
          Koschei Web3 Hub does not provide token trading, wallet custody, private key handling, investment advice or smart contract deployment. These are digital Web3 project output and builder support packages.
        </p>
      </div>
    </section>
  );
}
