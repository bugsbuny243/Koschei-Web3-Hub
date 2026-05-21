import { MachineryProductDetail } from "../product-detail";

export default async function ProductDetailPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  return <MachineryProductDetail slug={slug} videoSectionTitle="Product Videos" />;
}
