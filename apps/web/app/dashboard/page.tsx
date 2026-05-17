import { auth } from "@/lib/auth";
import { redirect } from "next/navigation";

export default async function DashboardPage() {
  const session = await auth();
  if (!session?.user) redirect("/");

  return (
    <main className="space-y-3">
      <h1 className="text-2xl font-bold">Protected Dashboard</h1>
      <p>User: {session.user.email}</p>
      <a className="text-blue-700 underline" href="/wallet">Wallet page</a>
    </main>
  );
}
