import "server-only";

export type SupplierSearchResult = {
  title: string;
  url: string;
  snippet: string;
  platform: string;
  search_query: string;
};

function detectPlatform(url: string): string {
  const value = url.toLowerCase();
  if (value.includes("alibaba.com")) return "Alibaba";
  if (value.includes("made-in-china.com")) return "Made-in-China";
  if (value.includes("globalsources.com")) return "Global Sources";
  if (value.includes("aliexpress.com")) return "AliExpress";
  return "Website";
}

export async function searchSuppliers(query: string): Promise<SupplierSearchResult[]> {
  const apiKey = process.env.BRAVE_SEARCH_API_KEY;
  if (!apiKey) throw new Error("Brave Search API is not configured.");

  const url = new URL("https://api.search.brave.com/res/v1/web/search");
  url.searchParams.set("q", query);
  url.searchParams.set("count", "15");

  const res = await fetch(url.toString(), {
    headers: { Accept: "application/json", "X-Subscription-Token": apiKey },
    cache: "no-store",
  });
  if (!res.ok) throw new Error(`Brave Search failed (${res.status})`);

  const data = (await res.json()) as { web?: { results?: Array<{ title?: string; url?: string; description?: string }> } };
  return (data.web?.results ?? []).filter((r) => r.url).map((r) => ({
    title: r.title ?? "",
    url: r.url ?? "",
    snippet: r.description ?? "",
    platform: detectPlatform(r.url ?? ""),
    search_query: query,
  }));
}
