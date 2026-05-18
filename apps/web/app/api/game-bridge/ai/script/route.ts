import { NextResponse } from "next/server";
import { generateAdapterConfig, generateGodotSnippet } from "@/lib/game-bridge";

export async function POST(req: Request) {
  const body = await req.json();
  const item = body?.item ?? { item_key: "sword_of_koschei", name: "Sword of Koschei" };
  return NextResponse.json({
    prompts: {
      script_generation: "Generate Godot GDScript inventory integration code using item schema and readonly web3 adapter.",
      adapter_generation: "Generate Web3 adapter config for Alchemy read-only RPC/API with no custody and no tx sending."
    },
    godot_script: generateGodotSnippet(item),
    adapter_config: generateAdapterConfig(body?.chain)
  });
}
