import { signIn, signOut, auth } from "@/lib/auth";

export default async function Home() {
  const session = await auth();
  return (
    <main className="space-y-4">
      <h1 className="text-2xl font-bold">Koschei Bridge</h1>
      {!session?.user ? (
        <form
          action={async () => {
            "use server";
            await signIn("credentials", { email: "demo@koschei.local", name: "Demo User", redirectTo: "/dashboard" });
          }}
        >
          <button className="rounded bg-black px-4 py-2 text-white">Sign in (Demo)</button>
        </form>
      ) : (
        <div className="space-y-3">
          <p>Signed in as {session.user.email}</p>
          <a className="text-blue-700 underline" href="/dashboard">Go to dashboard</a>
          <form action={async () => { "use server"; await signOut({ redirectTo: "/" }); }}>
            <button className="rounded border px-4 py-2">Sign out</button>
          </form>
        </div>
      )}
    </main>
  );
}
