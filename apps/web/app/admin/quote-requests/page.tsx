export default async function QuoteRequestsPage({ searchParams }: { searchParams: Promise<{ password?: string }> }) {
  const { password } = await searchParams;
  if (!process.env.ADMIN_PASSWORD || password !== process.env.ADMIN_PASSWORD) {
    return <div className="page-stack"><h1>Admin Access Required</h1></div>;
  }

  return (
    <div className="page-stack">
      <h1>Quote Requests (Admin)</h1>
      <ul>
        <li>target crop types</li>
        <li>delivery country/city</li>
        <li>preferred delivery term</li>
        <li>screen sets requested</li>
        <li>need fan/cyclone</li>
        <li>need control cabinet</li>
        <li>need bucket elevator</li>
        <li>company registration status</li>
      </ul>
    </div>
  );
}
