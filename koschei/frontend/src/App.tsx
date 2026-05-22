import { useEffect, useMemo, useState } from 'react';

type Role = 'public' | 'user' | 'owner';
const currentRole: Role = 'public';

type Tool = { title: string; icon: string; description: string };
const tools: Tool[] = [
  { title: 'Code Generator', icon: '🧠', description: 'Generate production-ready code, refactor modules, and ship faster with AI pair programming.' },
  { title: 'App Builder', icon: '📱', description: 'Design and launch full-stack app experiences from prompts, flows, and templates.' },
  { title: 'Web Builder', icon: '🌐', description: 'Build premium marketing sites, landing pages, and internal tools with live previews.' },
  { title: 'Game Builder', icon: '🎮', description: 'Create game loops, levels, assets, and scripts for rapid prototype-to-play cycles.' },
  { title: 'Image Studio', icon: '🖼️', description: 'Generate visual assets, concept art, and brand content with model-guided control.' },
  { title: 'Video Studio', icon: '🎬', description: 'Turn ideas into short-form videos, cinematic clips, and story-driven visual outputs.' },
  { title: 'Voice Lab', icon: '🎙️', description: 'Produce TTS voiceovers, transcribe audio, and run multilingual speech pipelines.' },
  { title: 'Automation Workflows', icon: '⚙️', description: 'Chain tools into repeatable workflows for campaigns, content ops, and delivery tasks.' },
];

const modelCategories = ['Code', 'Chat', 'Reasoning', 'Image', 'Image Edit', 'Video', 'Cinematic Video', 'TTS', 'STT'];

const pricing = [
  { name: 'Free', price: '0 TL', credits: '500 monthly credits', toolsIncluded: 'Core tools', limits: 'Image/Video: trial limits', cta: 'Start Free' },
  { name: 'Builder', price: '899 TL / month', credits: '20,000 monthly credits', toolsIncluded: 'All builder tools', limits: 'Image/Video: standard limits', cta: 'Choose Builder' },
  { name: 'Pro', price: '2,299 TL / month', credits: '70,000 monthly credits', toolsIncluded: 'All tools + priority queue', limits: 'Image/Video: high limits', cta: 'Upgrade to Pro' },
  { name: 'Studio', price: '4,999 TL / month', credits: '180,000 monthly credits', toolsIncluded: 'Full studio suite + premium routing', limits: 'Image/Video: studio limits', cta: 'Go Studio' },
];

const navigate = (to: string) => {
  window.history.pushState({}, '', to);
  window.dispatchEvent(new PopStateEvent('popstate'));
};

function Home() {
  return (
    <main className="container">
      <section className="hero premium-card">
        <div>
          <p className="eyebrow">Koschei Premium AI SaaS</p>
          <h1>Build with an Immortal AI Command Center for Code, Media, and Automation.</h1>
          <p className="subtext">Koschei unifies tools and model routing so teams can generate products, content, and workflows from one premium platform.</p>
          <div className="actions">
            <button className="btn primary" onClick={() => navigate('/platform')}>Start Building</button>
            <button className="btn" onClick={() => navigate('/pricing')}>View Pricing</button>
          </div>
        </div>
        <aside className="mockup-card">
          <h3>AI Command Center</h3>
          <p className="status">Router Status: <span>Online</span></p>
          <ul>
            <li>Credits: 18,420</li>
            <li>Active tools: 8</li>
            <li>Queue: 3 running jobs</li>
          </ul>
          <p className="model">Model selected: Qwen3-Coder / DeepSeek / FLUX / Veo</p>
        </aside>
      </section>

      <section className="premium-card">
        <h2>Live Dashboard Preview</h2>
        <div className="dashboard-grid">
          <article className="card"><h3>AI Router</h3><p>Stable • low latency • smart fallback</p></article>
          <article className="card"><h3>Credits</h3><p>18,420 available this cycle</p></article>
          <article className="card"><h3>Active Tools</h3><p>Code, Web, Image, Video, Voice</p></article>
          <article className="card"><h3>Recent Jobs</h3><p>Landing page generation, video teaser, and app scaffold</p></article>
        </div>
      </section>

      <section className="premium-card">
        <h2>Tools</h2>
        <div className="grid">{tools.map((item) => <article key={item.title} className="card"><h3>{item.icon} {item.title}</h3><p>{item.description}</p></article>)}</div>
      </section>

      <section className="premium-card">
        <h2>Model Router</h2>
        <div className="grid">{modelCategories.map((item) => <article key={item} className="card"><p>{item}</p></article>)}</div>
      </section>

      <section className="premium-card">
        <h2>Pricing</h2>
        <div className="grid">
          {pricing.map((plan) => (
            <article key={plan.name} className="card price-card">
              <h3>{plan.name}</h3>
              <p className="price">{plan.price}</p>
              <ul>
                <li>{plan.credits}</li>
                <li>{plan.toolsIncluded}</li>
                <li>{plan.limits}</li>
              </ul>
              <button className="btn primary">{plan.cta}</button>
            </article>
          ))}
        </div>
      </section>
    </main>
  );
}

function Platform() {
  return <main className="container page premium-card"><h1>Platform</h1><p>Koschei combines Code, App, Web, Game, Image, Video, Voice, and Automation tools in one workflow engine with intelligent model routing across code, chat, reasoning, image, video, and audio categories.</p></main>;
}
function PricingPage() {
  return <main className="container page premium-card"><h1>Pricing</h1><div className="grid">{pricing.map((plan) => <article key={plan.name} className="card price-card"><h3>{plan.name}</h3><p className="price">{plan.price}</p><p>{plan.credits}</p><p>{plan.toolsIncluded}</p><p>{plan.limits}</p><button className="btn primary">{plan.cta}</button></article>)}</div></main>;
}
function BillingPage() {
  return <main className="container page premium-card"><h1>Billing</h1><p>Automatic payment API is not active yet. Payments are manually verified and activated.</p><ol><li>Choose a plan and pay via Shopier payment link placeholder.</li><li>Share payment confirmation with owner.</li><li>Owner manually activates your plan and credits.</li></ol><div className="actions"><button className="btn">Shopier Free/Builder Placeholder</button><button className="btn">Shopier Pro/Studio Placeholder</button></div></main>;
}
function DashboardPage() {
  return <main className="container page premium-card"><h1>Dashboard</h1><div className="dashboard-grid"><article className="card"><h3>Credits</h3><p>0</p></article><article className="card"><h3>Projects</h3><p>0</p></article><article className="card"><h3>AI Jobs</h3><p>No jobs yet</p></article><article className="card"><h3>Model Router</h3><p>Ready to route</p></article></div><p className="subtext">Premium dashboard modules coming soon.</p></main>;
}
function OwnerPage() {
  return <main className="container page premium-card"><h1>Private Owner Cockpit</h1><p>This area is restricted by owner role guard placeholder. Public users cannot access owner tools.</p></main>;
}

export default function App() {
  const [path, setPath] = useState(window.location.pathname);

  useEffect(() => {
    const onPop = () => setPath(window.location.pathname);
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);

  const route = useMemo(() => (['/', '/platform', '/dashboard', '/pricing', '/billing', '/owner'].includes(path) ? path : '/'), [path]);
  const canOwner = currentRole === 'owner';

  return (
    <div>
      <header className="topbar">
        <button className="logo" onClick={() => navigate('/')}>KOSCHEI</button>
        <nav>
          <button className="btn" onClick={() => navigate('/platform')}>Platform</button>
          <button className="btn" onClick={() => navigate('/pricing')}>Pricing</button>
          <button className="btn" onClick={() => navigate('/dashboard')}>Dashboard</button>
          <button className="btn" onClick={() => navigate('/billing')}>Billing</button>
          {canOwner && <button className="btn" onClick={() => navigate('/owner')}>Owner</button>}
        </nav>
      </header>

      {route === '/' && <Home />}
      {route === '/platform' && <Platform />}
      {route === '/dashboard' && <DashboardPage />}
      {route === '/pricing' && <PricingPage />}
      {route === '/billing' && <BillingPage />}
      {route === '/owner' && <OwnerPage />}

      <footer className="footer">
        <div className="container footer-inner">
          <p className="logo">KOSCHEI</p>
          <div className="footer-links">
            <button className="btn" onClick={() => navigate('/platform')}>Platform</button>
            <button className="btn" onClick={() => navigate('/pricing')}>Pricing</button>
            <button className="btn" onClick={() => navigate('/billing')}>Billing</button>
          </div>
          <p className="subtext">Security: Owner tools are private and not visible publicly.</p>
        </div>
      </footer>
    </div>
  );
}
