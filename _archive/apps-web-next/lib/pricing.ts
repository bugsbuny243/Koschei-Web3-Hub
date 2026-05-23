export type PricingPackage = {
  key: "starter" | "pro";
  badge: string;
  title: string;
  priceLabel: string;
  ctaLabel: string;
  shopierUrl: string;
};

export const PRICING_PACKAGES: Record<PricingPackage["key"], PricingPackage> = {
  starter: {
    key: "starter",
    badge: "Starter Pack",
    title: "Koschei Starter Pack – 20.000 Credits",
    priceLabel: "899 TL",
    ctaLabel: "Buy 20.000 Credits",
    shopierUrl: "https://www.shopier.com/TradeVisual/47465449",
  },
  pro: {
    key: "pro",
    badge: "Pro",
    title: "Pro",
    priceLabel: "2,299 TL / month",
    ctaLabel: "Upgrade to Pro",
    shopierUrl: "https://www.shopier.com/TradeVisual/47465484",
  },
};
