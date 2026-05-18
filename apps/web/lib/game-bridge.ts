import "server-only";
import { web3Db } from "@/lib/web3-db";

export const gameBridgeSafetyCopy = "Koschei Web3 Game Bridge MVP does not hold funds, manage private keys, deploy contracts, mint NFTs, or custody user assets. It only prepares game item metadata, adapter configs, and integration scripts.";

export type GameBridgeProject = {
  id: string;
  name: string;
  slug: string;
  genre: string | null;
  target_chain: string | null;
  status: string;
  description: string | null;
  created_at: string;
};

export type GameBridgeItem = {
  id: string;
  project_id: string | null;
  item_key: string;
  name: string;
  item_type: string;
  rarity: string;
  image_uri: string | null;
  attributes: Record<string, unknown>;
  created_at: string;
};

export function buildMetadata(item: Pick<GameBridgeItem, "item_key" | "name" | "item_type" | "rarity" | "image_uri" | "attributes">) {
  return {
    name: item.name,
    description: `${item.name} (${item.rarity} ${item.item_type}) from Koschei game inventory.`,
    image: item.image_uri ?? "",
    external_url: "https://koschei-bridge.app/web3/game-bridge",
    animation_url: "",
    background_color: "000000",
    attributes: [
      { trait_type: "item_key", value: item.item_key },
      { trait_type: "item_type", value: item.item_type },
      { trait_type: "rarity", value: item.rarity },
      ...Object.entries(item.attributes ?? {}).map(([trait_type, value]) => ({ trait_type, value }))
    ]
  };
}

export function generateGodotSnippet(item: Pick<GameBridgeItem, "item_key" | "name">) {
  return `# inventory_demo.gd snippet\nvar ${item.item_key} = {\n  "id": "${item.item_key}",\n  "name": "${item.name}",\n  "metadata_endpoint": "https://your-app.com/api/game-bridge/metadata/generate"\n}`;
}

export function generateAdapterConfig(chain = "arbitrum-sepolia") {
  return {
    adapter: "alchemy-readonly",
    chain,
    capabilities: ["balance_read", "nft_metadata_read", "contract_read"],
    send_transactions: false,
    custody: "none"
  };
}

export const gameBridgeDb = {
  async listProjects() {
    return (await web3Db.query<GameBridgeProject>(`select id::text, name, slug, genre, target_chain, status, description, created_at::text from game_bridge_projects order by created_at desc`)).rows;
  },
  async createProject(input: {name: string; slug: string; genre?: string; target_chain?: string; description?: string;}) {
    const { rows } = await web3Db.query<GameBridgeProject>(`insert into game_bridge_projects (name, slug, genre, target_chain, description) values ($1,$2,$3,$4,$5) returning id::text, name, slug, genre, target_chain, status, description, created_at::text`, [input.name, input.slug, input.genre ?? null, input.target_chain ?? null, input.description ?? null]);
    return rows[0];
  },
  async listItems() {
    return (await web3Db.query<GameBridgeItem>(`select id::text, project_id::text, item_key, name, item_type, rarity, image_uri, attributes, created_at::text from game_bridge_items order by created_at desc`)).rows;
  },
  async createItem(input: {project_id?: string; item_key: string; name: string; item_type: string; rarity: string; image_uri?: string; attributes?: Record<string, unknown>;}) {
    const { rows } = await web3Db.query<GameBridgeItem>(`insert into game_bridge_items (project_id, item_key, name, item_type, rarity, image_uri, attributes) values ($1,$2,$3,$4,$5,$6,$7) returning id::text, project_id::text, item_key, name, item_type, rarity, image_uri, attributes, created_at::text`, [input.project_id ?? null, input.item_key, input.name, input.item_type, input.rarity, input.image_uri ?? null, JSON.stringify(input.attributes ?? {})]);
    return rows[0];
  }
};
