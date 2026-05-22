import { useEffect, useMemo, useState } from 'react';

type Role = 'public' | 'user' | 'owner';
const currentRole: Role = 'public';

const tools = [
  'Code Generator',
  'App Builder',
  'Web Builder',
  'Game Builder',
  'Image Studio',
  'Video Studio',
  'Voice Lab',
  'Automation Workflows',
];

const modelRouter = [
  'Qwen3-Coder-480B',
  'Llama 3.3-70B',
  'DeepSeek V4 Pro',
  'FLUX.2 Pro',
  'FLUX Kontext Pro',
  'Google Veo 3.0',
  'Kling 2.1 Pro',
  'Kokoro 82M',
  'Whisper large-v3',
];

const pricing = [
  { name: 'Free', price: '0 TL' },
  { name: 'Builder', price: '899 TL / month' },
  { name: 'Pro', price: '2,299 TL / month' },
  { name: 'Studio', price: '4,999 TL / month' },
];

const navigate = (to: string) => {
  window.history.pushState({}, '', to);
  window.dispatchEvent(new PopStateEvent('popstate'));
};

function Placeholder({ title, text }: { title: string; text: string }) {
  return (
    <main className="container placeholder">
      <h1>{title}</h1>
      <p>{text}</p>
    </main>
  );
}

function Home() {
  return (
    <main className="container">
      <section className="hero">
        <p className="eyebrow">Koschei Immortal AI Platform</p>
        <h1>Build apps, games, websites, scripts, images, videos and voices with one immortal AI engine.</h1>
        <p className="subtext">
          Design, generate, iterate and ship from a single futuristic workspace built for modern AI-native teams.
        </p>
        <div className="actions">
          <button className="btn primary" onClick={() => navigate('/platform')}>Start Building</button>
          <button className="btn" onClick={() => navigate('/pricing')}>View Pricing</button>
        </div>
      </section>

      <section>
        <h2>Tools</h2>
        <div className="grid">{tools.map((item) => <article key={item} className="card"><h3>{item}</h3></article>)}</div>
      </section>

      <section>
        <h2>Model Router</h2>
        <div className="grid">{modelRouter.map((item) => <article key={item} className="card"><p>{item}</p></article>)}</div>
      </section>

      <section>
        <h2>Pricing</h2>
        <div className="grid">
          {pricing.map((plan) => (
            <article key={plan.name} className="card price-card">
              <h3>{plan.name}</h3>
              <p>{plan.price}</p>
            </article>
          ))}
        </div>
      </section>
    </main>
  );
}

export default function App() {
  const [path, setPath] = useState(window.location.pathname);

  useEffect(() => {
    const onPop = () => setPath(window.location.pathname);
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);

  const route = useMemo(
    () => (['/', '/platform', '/dashboard', '/pricing', '/billing', '/owner'].includes(path) ? path : '/'),
    [path],
  );

  const canUser = currentRole === 'user' || currentRole === 'owner';
  const canOwner = currentRole === 'owner';

  return (
    <div>
      <header className="topbar">
        <button className="logo" onClick={() => navigate('/')}>KOSCHEI</button>
        <nav>
          <button className="btn" onClick={() => navigate('/platform')}>Platform</button>
          <button className="btn" onClick={() => navigate('/pricing')}>Pricing</button>
          {canUser && <button className="btn" onClick={() => navigate('/dashboard')}>Dashboard</button>}
          {canUser && <button className="btn" onClick={() => navigate('/billing')}>Billing</button>}
          {canOwner && <button className="btn" onClick={() => navigate('/owner')}>Owner</button>}
        </nav>
      </header>

      {route === '/' && <Home />}
      {route === '/platform' && <Placeholder title="Platform" text="Internal /platform route placeholder." />}
      {route === '/dashboard' && <Placeholder title="Dashboard" text="Internal /dashboard route placeholder." />}
      {route === '/pricing' && <Placeholder title="Pricing" text="Internal /pricing route placeholder." />}
      {route === '/billing' && <Placeholder title="Billing" text="Internal /billing route placeholder." />}
      {route === '/owner' && <Placeholder title="Owner" text="Private /owner route placeholder." />}
    </div>
  );
}
