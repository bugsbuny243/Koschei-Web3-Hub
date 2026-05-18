async function getStats() {
  const baseUrl = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:4000';
  const [metricsRes, leaderboardRes] = await Promise.all([
    fetch(`${baseUrl}/api/metrics`, { cache: 'no-store' }),
    fetch(`${baseUrl}/api/leaderboard`, { cache: 'no-store' })
  ]);
  const metrics = await metricsRes.json();
  const leaderboard = await leaderboardRes.json();
  return { metrics: metrics.data, leaderboard: leaderboard.data.topPlayers || [] };
}

export default async function StatsPage() {
  const { metrics, leaderboard } = await getStats();
  return <main style={{ padding: 24 }}>
    <h1>Koscei Base Metrics</h1>
    <ul>
      <li>Total players registered: {metrics.totalPlayers}</li>
      <li>Total NFTs minted: {metrics.totalAssets}</li>
      <li>Total transactions on Base (weekly): {metrics.weeklyTransactions}</li>
      <li>Network: {metrics.network}</li>
    </ul>
    <h2>Top 10 players</h2>
    <pre>{JSON.stringify(leaderboard, null, 2)}</pre>
    <h2>Daily activity (last 7 days)</h2>
    <p>Coming from /api/metrics daily stats aggregation.</p>
  </main>;
}
