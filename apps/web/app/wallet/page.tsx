export default function WalletPage() {
  return (
    <main className="space-y-3">
      <h1 className="text-2xl font-bold">Wallet</h1>
      <p>Use API routes:</p>
      <ul className="list-disc pl-6">
        <li>POST /api/wallet/create</li>
        <li>GET /api/wallet/me</li>
      </ul>
    </main>
  );
}
