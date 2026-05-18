import { gameBridgeSafetyCopy, generateAdapterConfig } from "@/lib/game-bridge";

export default function PluginPage() {
  const pluginExport = {
    plugin_name: "GodotWeb3Bridge",
    plugin_goal: "AI-assisted no-custody Godot Web3 developer tooling",
    supports: ["game item ownership", "NFT metadata", "adapter configs", "integration scripts"],
    inventory_demo_script: "inventory_demo.gd",
    metadata_template: "koschei-item-metadata-v1",
    web3_adapter: generateAdapterConfig(),
    export_format: "plugin-compatible-json"
  };

  return <main className="mx-auto max-w-5xl space-y-4 p-6">
    <h1 className="text-2xl font-bold">Game Bridge Plugin Demo</h1>
    <p className="text-gray-700">Generated output for the public grant demo plugin workflow.</p>
    <p className="rounded bg-amber-100 p-3 text-sm">{gameBridgeSafetyCopy}</p>
    <pre className="rounded bg-gray-100 p-4 text-xs">{JSON.stringify(pluginExport, null, 2)}</pre>
  </main>;
}
