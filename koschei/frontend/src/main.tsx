import React, { useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { colors } from './theme/tokens';

type Role = 'public' | 'user' | 'owner';

type ToolCard = { title: string; description: string };
type Plan = { name: string; price: string; details: string };
type RouteLink = { href: string; label: string; protected?: boolean };

const currentRole: Role = 'public'; // Placeholder auth role from Go backend session.

const tools: ToolCard[] = [
  { title: 'Code Generator', description: 'Generate, refactor and ship production code faster.' },
  { title: 'App Builder', description: 'Compose full applications with multi-step AI workflows.' },
  { title: 'Web Builder', description: 'Launch premium web experiences from prompts and assets.' },
  { title: 'Game Builder', description: 'Prototype game mechanics, worlds and logic instantly.' },
  { title: 'Image Generator', description: 'Create marketing visuals, assets and concept art.' },
  { title: 'Video Generator', description: 'Produce cinematic clips and branded social reels.' },
  { title: 'Voice Tools', description: 'Generate voiceovers, clones and studio-ready narration.' },
  { title: 'Automation Workflows', description: 'Automate multi-tool chains with resilient orchestration.' }
];

const models = [
  'Qwen3-Coder-480B for code',
  'Llama 3.3-70B for chat and analysis',
  'DeepSeek V4 Pro for reasoning',
  'FLUX.2 Pro for images',
  'FLUX Kontext Pro for image editing',
  'Veo 3.0 for video',
  'Kling 2.1 Pro for cinema',
  'Kokoro 82M for TTS',
  'Whisper large-v3 for STT'
];

const plans: Plan[] = [
  { name: 'Free', price: '$0', details: 'Start building with core tools and limited usage.' },
  { name: 'Starter', price: '$29/mo', details: 'For founders shipping faster with more generation credits.' },
  { name: 'Pro', price: '$99/mo', details: 'For teams that need full velocity and premium model access.' }
];

const publicNav: RouteLink[] = [
  { href: '/', label: 'Home' },
  { href: '/platform', label: 'Platform' },
  { href: '/pricing', label: 'Pricing' }
];

const appNav: RouteLink[] = [
  { href: '/dashboard', label: 'Dashboard', protected: true },
  { href: '/billing', label: 'Billing', protected: true }
];

const ownerNav: RouteLink[] = [{ href: '/owner', label: 'Owner Console', protected: true }];

const navigate = (to: string) => {
  window.history.pushState({}, '', to);
  window.dispatchEvent(new PopStateEvent('popstate'));
};

const api = {
  getPublicPlatform: async () => ({ endpoint: '/api/platform', backend: 'go-placeholder' }),
  getPricing: async () => ({ endpoint: '/api/pricing', backend: 'go-placeholder' })
};

const palette = {
  bg: '#05040c',
  bg2: '#0b0a16',
  text: '#ecedff',
  muted: '#a5a8c7',
  neonGreen: '#57ff8a',
  neonPurple: '#9f6bff',
  border: 'rgba(157, 114, 255, 0.25)',
  glass: 'rgba(255, 255, 255, 0.06)'
};

const styles: Record<string, React.CSSProperties> = {
  app: { background: `radial-gradient(circle at 15% -20%, #1f1a3d 0%, ${palette.bg} 40%, #040308 100%)`, color: palette.text, minHeight: '100vh', fontFamily: 'Inter, system-ui, sans-serif' },
  nav: { position: 'sticky', top: 0, zIndex: 10, display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 16, padding: '14px 18px', background: 'rgba(7, 6, 16, 0.72)', borderBottom: `1px solid ${palette.border}`, backdropFilter: 'blur(12px)' },
  navLinks: { display: 'flex', gap: 10, flexWrap: 'wrap' },
  shell: { maxWidth: 1160, margin: '0 auto', padding: '0 16px 48px' },
  hero: { padding: '68px 0 34px' },
  heroTitle: { fontSize: 'clamp(2rem, 5vw, 3.5rem)', lineHeight: 1.05, margin: '0 0 12px', letterSpacing: '-0.02em' },
  subtitle: { color: palette.muted, fontSize: 'clamp(1rem, 2vw, 1.2rem)', maxWidth: 820, lineHeight: 1.6 },
  buttonRow: { display: 'flex', gap: 12, flexWrap: 'wrap', marginTop: 26 },
  grid: { display: 'grid', gap: 14, gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))' },
  card: { background: palette.glass, border: `1px solid ${palette.border}`, borderRadius: 18, padding: 16, boxShadow: '0 8px 26px rgba(9, 9, 18, 0.35)' },
  section: { marginTop: 30 },
  sectionTitle: { fontSize: '1.35rem', marginBottom: 14 },
  cta: { marginTop: 36, padding: 24, borderRadius: 20, background: 'linear-gradient(90deg, rgba(87,255,138,0.18), rgba(159,107,255,0.18))', border: `1px solid ${palette.border}` }
};

const LinkButton = ({ href, label, filled = false }: { href: string; label: string; filled?: boolean }) => (
  <button
    onClick={() => navigate(href)}
    style={{
      padding: '11px 16px',
      borderRadius: 999,
      border: `1px solid ${filled ? palette.neonGreen : palette.border}`,
      background: filled ? `linear-gradient(90deg, ${palette.neonGreen}, ${palette.neonPurple})` : 'transparent',
      color: filled ? '#06060f' : palette.text,
      cursor: 'pointer',
      fontWeight: 700
    }}
  >
    {label}
  </button>
);

function HomePage() {
  useEffect(() => { void api.getPublicPlatform(); }, []);
  return (
    <div style={styles.shell}>
      <section style={styles.hero}>
        <p style={{ color: palette.neonGreen, marginBottom: 12 }}>Koschei — The Immortal AI Platform</p>
        <h1 style={styles.heroTitle}>Build apps, games, websites, scripts, images, videos and voices with one immortal AI engine.</h1>
        <p style={styles.subtitle}>Koschei combines code generation, AI chat, deep reasoning, image generation, video generation, voice tools and automation into one production platform.</p>
        <div style={styles.buttonRow}><LinkButton href="/platform" label="Start Building" filled /><LinkButton href="/pricing" label="View Pricing" /></div>
      </section>

      <section style={styles.section}>
        <h2 style={styles.sectionTitle}>AI Tools</h2>
        <div style={styles.grid}>{tools.map((tool) => <div key={tool.title} style={styles.card}><h3>{tool.title}</h3><p style={{ color: palette.muted }}>{tool.description}</p></div>)}</div>
      </section>

      <section style={styles.section}>
        <h2 style={styles.sectionTitle}>Model Router</h2>
        <div style={styles.grid}>{models.map((model) => <div key={model} style={styles.card}>{model}</div>)}</div>
      </section>

      <section style={styles.section}>
        <h2 style={styles.sectionTitle}>Public SaaS Platform</h2>
        <div style={styles.card}><p style={{ margin: 0, color: palette.muted }}>From prototype to production, Koschei unifies generation, orchestration and delivery in one public platform experience.</p></div>
      </section>

      <section style={styles.section}>
        <h2 style={styles.sectionTitle}>Pricing Preview</h2>
        <div style={styles.grid}>{plans.map((plan) => <div key={plan.name} style={styles.card}><h3>{plan.name}</h3><p style={{ color: palette.neonGreen, fontWeight: 700 }}>{plan.price}</p><p style={{ color: palette.muted }}>{plan.details}</p></div>)}</div>
      </section>

      <section style={styles.cta}><h2 style={{ marginTop: 0 }}>Start building with Koschei</h2><div style={styles.buttonRow}><LinkButton href="/platform" label="Start Building" filled /><LinkButton href="/pricing" label="View Pricing" /></div></section>
    </div>
  );
}

const Placeholder = ({ title, description }: { title: string; description: string }) => <div style={{ ...styles.shell, paddingTop: 42 }}><h1>{title}</h1><div style={styles.card}><p style={{ color: palette.muted }}>{description}</p></div></div>;

function App() {
  const [path, setPath] = useState(window.location.pathname);
  useEffect(() => {
    const onPop = () => setPath(window.location.pathname);
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);

  const route = useMemo(() => {
    if (path === '/' || path === '/platform' || path === '/pricing' || path === '/dashboard' || path === '/billing' || path.startsWith('/owner')) return path;
    return '/';
  }, [path]);

  const canUserAccessProtected = currentRole === 'user' || currentRole === 'owner';
  const canOwnerAccess = currentRole === 'owner';

  return (
    <div style={{ ...styles.app, background: colors.bg }}>
      <header style={styles.nav}>
        <button onClick={() => navigate('/')} style={{ background: 'none', border: 'none', color: palette.text, fontWeight: 800, cursor: 'pointer' }}>Koschei</button>
        <nav style={styles.navLinks}>
          {publicNav.map((item) => <LinkButton key={item.href} href={item.href} label={item.label} />)}
          {appNav.filter((item) => !item.protected || canUserAccessProtected).map((item) => <LinkButton key={item.href} href={item.href} label={item.label} />)}
          {ownerNav.filter(() => canOwnerAccess).map((item) => <LinkButton key={item.href} href={item.href} label={item.label} />)}
        </nav>
      </header>

      {route === '/' && <HomePage />}
      {route === '/platform' && <Placeholder title="Koschei Platform" description="Public platform route placeholder. API integrations should call Go backend endpoints only." />}
      {route === '/pricing' && <Placeholder title="Pricing" description="Pricing page route placeholder with Go backend data source contract." />}
      {route === '/dashboard' && (canUserAccessProtected ? <Placeholder title="Dashboard" description="Protected dashboard placeholder. Requires authenticated user session." /> : <Placeholder title="Access Required" description="Dashboard is protected. Sign in through the Go auth backend to continue." />)}
      {route === '/billing' && (canUserAccessProtected ? <Placeholder title="Billing" description="Protected billing placeholder. Connect subscription APIs via Go backend." /> : <Placeholder title="Access Required" description="Billing is protected. Sign in through the Go auth backend to continue." />)}
      {route.startsWith('/owner') && (canOwnerAccess ? <Placeholder title="Owner Control" description="Owner-only route placeholder under /owner namespace." /> : <Placeholder title="Unauthorized" description="Owner routes are hidden and restricted by owner role checks only." />)}
    </div>
  );
}

createRoot(document.getElementById('root')!).render(<App />);
