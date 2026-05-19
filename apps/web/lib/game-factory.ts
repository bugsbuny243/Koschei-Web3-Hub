import "server-only";
import { web3Db } from "@/lib/web3-db";

export const gameFactoryPositioning = "AI-assisted prompt-to-HTML5 game generation with one-click Web3-ready ownership packaging.";
export const gameFactorySafetyCopy = "MVP safety: no private keys, no wallet custody, no contract deployment, no minting, no transaction sending, no escrow, and no funds movement.";

export type GameFactoryProject = {
  id: string;
  name: string;
  slug: string;
  prompt: string;
  status: string;
  game_brief: Record<string, unknown> | null;
  phaser_template: string | null;
  extracted_items: Record<string, unknown>[];
  created_at: string;
};

export function buildGameBrief(prompt: string) {
  return {
    title: "Prompt-generated web game",
    core_loop: "Move, collect, and survive",
    genre: "arcade",
    art_style: "minimal",
    mechanics: ["movement", "collectibles", "scoring"],
    rewards: ["coin", "gem", "star"],
    achievements: ["first_run", "high_score_100", "collector_10"],
    prompt
  };
}

export function buildPhaserTemplate(projectId: string, brief: Record<string, unknown>) {
  const title = String(brief.title ?? "Koschei Game");
  return `// Generated Phaser template\nconst config={type:Phaser.AUTO,width:800,height:600,scene:{preload,create,update}};\nconst game=new Phaser.Game(config);\nlet player;let cursors;let score=0;let scoreText;\nfunction preload(){}\nfunction create(){this.cameras.main.setBackgroundColor('#0f172a');player=this.add.rectangle(400,300,28,28,0x22d3ee);this.physics.add.existing(player);cursors=this.input.keyboard.createCursorKeys();scoreText=this.add.text(16,16,'${title} | Score: 0',{color:'#fff'});}\nfunction update(){const body=player.body;body.setVelocity(0);if(cursors.left.isDown) body.setVelocityX(-160);if(cursors.right.isDown) body.setVelocityX(160);if(cursors.up.isDown) body.setVelocityY(-160);if(cursors.down.isDown) body.setVelocityY(160);score+=0.02;scoreText.setText('${title} | Score: '+Math.floor(score));}\n// project:${projectId}`;
}

export function extractItemsFromBrief(brief: Record<string, unknown>) {
  const rewards = Array.isArray(brief.rewards) ? brief.rewards : ["coin"];
  const achievements = Array.isArray(brief.achievements) ? brief.achievements : ["starter"];
  return [
    ...rewards.map((r, i) => ({ kind: "reward", key: String(r), name: String(r), rarity: i === 0 ? "common" : "rare" })),
    ...achievements.map((a) => ({ kind: "achievement", key: String(a), name: String(a), rarity: "badge" }))
  ];
}

export function buildNftMetadata(items: Record<string, unknown>[]) {
  return items.map((item) => ({
    name: String(item.name),
    description: `In-game ${String(item.kind)} from Koschei Web Game Factory`,
    image: "",
    attributes: [
      { trait_type: "kind", value: item.kind },
      { trait_type: "key", value: item.key },
      { trait_type: "rarity", value: item.rarity }
    ]
  }));
}

export function buildWeb3BridgeConfig(projectId: string) {
  return {
    project_id: projectId,
    chain: "arbitrum-sepolia",
    adapter: "alchemy-readonly",
    rpc_env_key: "ALCHEMY_ARBITRUM_SEPOLIA_URL",
    can_send_transactions: false,
    custody: "none"
  };
}

export const gameFactoryDb = {
  async createProject(input: { name: string; slug: string; prompt: string }) {
    const { rows } = await web3Db.query<GameFactoryProject>(`insert into game_factory_projects (name, slug, prompt) values ($1,$2,$3) returning id::text, name, slug, prompt, status, game_brief, phaser_template, extracted_items, created_at::text`, [input.name, input.slug, input.prompt]);
    return rows[0];
  },
  async listProjects() {
    return (await web3Db.query<GameFactoryProject>(`select id::text, name, slug, prompt, status, game_brief, phaser_template, extracted_items, created_at::text from game_factory_projects order by created_at desc`)).rows;
  },
  async getProject(id: string) {
    const { rows } = await web3Db.query<GameFactoryProject>(`select id::text, name, slug, prompt, status, game_brief, phaser_template, extracted_items, created_at::text from game_factory_projects where id=$1 limit 1`, [id]);
    return rows[0] ?? null;
  },
  async saveGenerated(id: string, brief: Record<string, unknown>, template: string, items: Record<string, unknown>[]) {
    const { rows } = await web3Db.query<GameFactoryProject>(`update game_factory_projects set game_brief=$2::jsonb, phaser_template=$3, extracted_items=$4::jsonb, status='generated' where id=$1 returning id::text, name, slug, prompt, status, game_brief, phaser_template, extracted_items, created_at::text`, [id, JSON.stringify(brief), template, JSON.stringify(items)]);
    return rows[0] ?? null;
  }
};
