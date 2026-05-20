import { gameBridgeSafetyCopy } from "@/lib/game-bridge-copy";
import { SupportCta } from "@/components/support-cta";

const plannedFiles = [
  "addons/koschei_bridge/plugin.cfg",
  "addons/koschei_bridge/plugin.gd",
  "addons/koschei_bridge/core/inventory_item.gd",
  "addons/koschei_bridge/core/nft_metadata_builder.gd",
  "addons/koschei_bridge/web3/web3_adapter.gd",
  "addons/koschei_bridge/ui/koschei_bridge_panel.gd"
];

export default function PluginPage() {
  return <main className="mx-auto max-w-5xl space-y-6 px-4 py-6 sm:px-6">
    <h1 className="text-3xl font-bold">Godot Web3 Bridge Plugin Plan</h1>

    <section className="rounded-xl border p-5">
      <h2 className="text-xl font-semibold">Godot plugin structure</h2>
      <p className="mt-2 text-gray-700">The MVP plugin separates editor registration, core item models, NFT metadata building, Web3 adapter configuration, and editor UI utilities.</p>
    </section>

    <section className="rounded-xl border p-5">
      <h2 className="text-xl font-semibold">Planned files</h2>
      <ul className="mt-2 ml-6 list-disc break-all font-mono text-sm leading-6">
        {plannedFiles.map((file) => <li key={file}>{file}</li>)}
      </ul>
    </section>

    <section className="rounded border bg-gray-50 p-4 text-sm text-gray-800">
      <p><strong>MVP output boundary:</strong> this MVP exports JSON metadata and adapter configuration artifacts only. No onchain actions are performed.</p>
    </section>

    <section className="rounded bg-amber-100 p-3 text-sm">
      <p>{gameBridgeSafetyCopy}</p>
    </section>

    <SupportCta />
  </main>;
}
