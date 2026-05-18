import { gameBridgeSafetyCopy } from "@/lib/game-bridge";

export default function GameBridgeGrantPage() {
  return <main className="mx-auto max-w-4xl p-6 space-y-4"><h1 className="text-3xl font-bold">Koschei Web3 Game Bridge - Grant Roadmap</h1>
  <p className="rounded bg-amber-100 p-3 text-sm">{gameBridgeSafetyCopy}</p>
  <ul className="list-disc ml-6 space-y-1"><li>Phase 1: Project and item generation dashboard.</li><li>Phase 2: Metadata + Godot script assistant and adapter exports.</li><li>Phase 3: Validation, templates, and grant outcome reporting.</li></ul></main>;
}
