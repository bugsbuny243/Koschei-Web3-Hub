import Link from "next/link";

export default function HomePage() {
  return (
    <main className="flex min-h-screen items-center justify-center bg-zinc-950 px-6 text-zinc-100">
      <section className="mx-auto flex w-full max-w-4xl flex-col items-center text-center">
        <h1 className="text-3xl font-semibold tracking-tight md:text-5xl">
          Koschei — The Immortal AI Platform
        </h1>

        <p className="mt-8 max-w-3xl text-base text-zinc-300 md:text-2xl">
          “Build apps, games, websites, scripts, images, videos and voices with one immortal AI engine.”
        </p>

        <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
          <Link
            href="/dashboard"
            className="rounded-xl bg-sky-600 px-5 py-3 text-sm font-semibold text-white transition hover:bg-sky-500"
          >
            Start Building
          </Link>
          <Link
            href="/pricing"
            className="rounded-xl border border-zinc-700 px-5 py-3 text-sm font-semibold text-zinc-100 transition hover:bg-zinc-800"
          >
            View Pricing
          </Link>
          <Link
            href="/koschei"
            className="rounded-xl bg-fuchsia-600 px-5 py-3 text-sm font-semibold text-white transition hover:bg-fuchsia-500"
          >
            Enter God Mode
          </Link>
        </div>
      </section>
    </main>
  );
}
