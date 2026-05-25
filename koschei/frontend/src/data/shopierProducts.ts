export type ShopierProduct = {
  id: string;
  name: string;
  priceTry: number;
  credits: number;
  description: string;
  shopierUrl: string;
  badge?: string;
  isActive: boolean;
};

export const shopierProducts: ShopierProduct[] = [
  {
    id: 'starter',
    name: 'Koschei Starter',
    priceTry: 899,
    credits: 20000,
    description: 'Başlangıç seviye AI üretim paketi. Runtime, chat/code/reason ve temel artifact üretimi için.',
    shopierUrl: 'https://www.shopier.com/TradeVisual/47465449',
    badge: 'Starter',
    isActive: true,
  },
  {
    id: 'pro',
    name: 'Koschei Pro',
    priceTry: 2299,
    credits: 70000,
    description: 'Daha yoğun üretim yapan kullanıcılar için gelişmiş AI üretim paketi.',
    shopierUrl: 'https://www.shopier.com/TradeVisual/47465484',
    badge: 'Popular',
    isActive: true,
  },
  {
    id: 'studio',
    name: 'Koschei Studio',
    priceTry: 4999,
    credits: 180000,
    description: 'Ajans, geliştirici ve owner üretim akışları için yüksek kredi paketi.',
    shopierUrl: 'https://www.shopier.com/TradeVisual/47465499',
    badge: 'Studio',
    isActive: true,
  },
];

export function getShopierProduct(id: string): ShopierProduct | undefined {
  return shopierProducts.find((product) => product.id === id);
}
