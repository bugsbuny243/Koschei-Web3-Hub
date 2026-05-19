import { gameBridgeSafetyCopy } from "@/lib/game-bridge-copy";

const plannedFiles = [
  "addons/koschei_bridge/plugin.cfg",
  "addons/koschei_bridge/plugin.gd",
  "addons/koschei_bridge/core/inventory_item.gd",
  "addons/koschei_bridge/core/nft_metadata_builder.gd",
  "addons/koschei_bridge/web3/web3_adapter.gd",
  "addons/koschei_bridge/ui/koschei_bridge_panel.gd"
];

export default function PluginPage() {
  return <main className="mx-auto max-w-5xl space-y-5 p-6">
    <h1 className="text-3xl font-bold">Godot Plugin Plan</h1>

    <section>
      <h2 className="text-xl font-semibold">Godot plugin structure</h2>
      <p className="text-gray-700">The MVP plugin separates editor registration, core item models, metadata building, web3 adapter configuration, and editor UI panel utilities.</p>
    </section>

    <section>
      <h2 className="text-xl font-semibold">Planned files</h2>
      <ul className="ml-6 list-disc font-mono text-sm">
        {plannedFiles.map((file) => <li key={file}>{file}</li>)}
      </ul>
    </section>

    <section className="rounded border bg-gray-50 p-4 text-sm text-gray-800">
      <p><strong>MVP output boundary:</strong> this MVP exports JSON metadata and adapter configuration artifacts only. No onchain actions are performed.</p>
    </section>

    <section className="rounded bg-amber-100 p-3 text-sm">
      <p>{gameBridgeSafetyCopy}</p>
    </section>
  </main>;
}
