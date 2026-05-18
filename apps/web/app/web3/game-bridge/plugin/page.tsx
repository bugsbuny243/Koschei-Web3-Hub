import { generateAdapterConfig } from "@/lib/game-bridge";

export default function PluginPage() {
  const pluginExport = {
    plugin_name: "GodotWeb3Bridge",
    inventory_demo_script: "inventory_demo.gd",
    web3_adapter: generateAdapterConfig(),
    export_format: "plugin-compatible-json"
  };
  return <main className="mx-auto max-w-5xl p-6 space-y-3"><h1 className="text-2xl font-bold">Plugin Export</h1>
  <pre className="rounded bg-gray-100 p-4 text-xs">{JSON.stringify(pluginExport, null, 2)}</pre></main>;
}
