import { gameBridgeDb } from "@/lib/game-bridge";

export const dynamic = "force-dynamic";

export default async function ItemsPage() {
  const items = await gameBridgeDb.listItems();
  return <main className="mx-auto max-w-5xl p-6 space-y-4"><h1 className="text-2xl font-bold">Game Bridge Items</h1>
    <pre className="rounded bg-gray-100 p-4 text-xs overflow-auto">{JSON.stringify(items, null, 2)}</pre>
  </main>;
}
