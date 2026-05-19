import Link from "next/link";

type SupportCtaProps = {
  compact?: boolean;
};

const supportLink = "https://www.shopier.com/TradeVisual/47208457";

export function SupportCta({ compact = false }: SupportCtaProps) {
  return (
    <section className={`rounded-xl border bg-emerald-50/60 p-5 ${compact ? "" : "space-y-3"}`}>
      <h2 className="text-xl font-semibold">Support Koschei Development</h2>
      <p className="text-gray-700">
        Koschei Web3 Game Bridge is being built as an AI-assisted, no-custody developer tool for Godot and Web3 game
        builders. While grant applications are being prepared, small community support helps keep development moving.
      </p>
      <p className="text-sm font-medium text-gray-800">10 TL micro-support</p>
      <Link
        href={supportLink}
        target="_blank"
        rel="noopener noreferrer"
        className="inline-flex rounded bg-emerald-700 px-4 py-2 text-sm font-semibold text-white hover:bg-emerald-800"
      >
        Support with 10 TL
      </Link>
    </section>
  );
}
