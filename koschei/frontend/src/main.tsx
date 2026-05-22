import React, { useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { colors } from './theme/tokens';

type Role = 'public' | 'user' | 'owner';
type Plan = { name: string; price: string; detail: string; highlight?: boolean };
type Tool = { name: string; description: string };

type AppRoute = '/' | '/platform' | '/pricing' | '/dashboard' | '/billing' | '/owner';

const session = {
  role: 'public' as Role
};

const tools: Tool[] = [
  { name: 'Code Generator', description: 'Generate production-ready code, refactor legacy modules and scaffold APIs in minutes.' },
  { name: 'App Builder', description: 'Ship full product flows with a guided AI planner, QA assistant and integration prompts.' },
  { name: 'Web Builder', description: 'Create modern web experiences with responsive layouts, component logic and launch-ready copy.' },
  { name: 'Game Builder', description: 'Prototype gameplay loops, dialogue trees, balancing logic and asset prompts instantly.' },
  { name: 'Image Studio', description: 'Generate campaign visuals, thumbnails and design variations with premium fidelity models.' },
  { name: 'Video Studio', description: 'Create cinematic social clips, ads and explainers with timeline-aware AI generation.' },
  { name: 'Voice Lab', description: 'Produce synthetic narration, multilingual dubbing and studio voice pipelines from text.' },
  { name: 'Automation Workflows', description: 'Orchestrate cross-tool chains that transform prompts into end-to-end deliverables.' }
];

const models = [
  'Qwen3-Coder-480B',
  'Llama 3.3-70B',
  'DeepSeek V4 Pro',
  'FLUX.2 Pro',
  'FLUX Kontext Pro',
  'Google Veo 3.0',
  'Kling 2.1 Pro',
  'Kokoro 82M',
  'Whisper large-v3'
];

const plans: Plan[] = [
  { name: 'Free', price: '0 TL', detail: 'Ideal for testing core tools and previewing the immortal workflow stack.' },
  { name: 'Builder', price: '899 TL / month', detail: 'Best for solo founders building apps and launching client-ready assets.' },
  { name: 'Pro', price: '2,299 TL / month', detail: 'High-throughput generation, premium routing and team-level velocity.', highlight: true },
  { name: 'Studio', price: '4,999 TL / month', detail: 'For agencies and studios requiring predictable output and priority handling.' }
];

const shopierLinks = {
  builder: 'https://shopier.com/placeholder-builder',
  pro: 'https://shopier.com/placeholder-pro',
  studio: 'https://shopier.com/placeholder-studio'
};

const palette = {
  bg: '#06050e',
  panel: 'rgba(20, 18, 36, 0.68)',
  panelSoft: 'rgba(255,255,255,0.05)',
  text: '#edf0ff',
  muted: '#aeb2cb',
  green: '#59ff9d',
  purple: '#a86dff',
  border: 'rgba(168, 109, 255, 0.32)',
  heroGlow: 'radial-gradient(circle at 25% 10%, rgba(89,255,157,0.20), transparent 38%), radial-gradient(circle at 85% 5%, rgba(168,109,255,0.22), transparent 34%)'
};

const navigate = (route: AppRoute) => {
  window.history.pushState({}, '', route);
  window.dispatchEvent(new PopStateEvent('popstate'));
};

const styles: Record<string, React.CSSProperties> = {
  app: { minHeight: '100vh', color: palette.text, fontFamily: 'Inter, ui-sans-serif, system-ui', background: `${palette.heroGlow}, ${colors.bg}` },
  shell: { width: 'min(1160px, 100%)', margin: '0 auto', padding: '0 16px 48px' },
  nav: { position: 'sticky', top: 0, zIndex: 20, display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12, padding: '14px 16px', borderBottom: `1px solid ${palette.border}`, background: 'rgba(8, 7, 18, 0.8)', backdropFilter: 'blur(16px)' },
  navLinks: { display: 'flex', flexWrap: 'wrap', gap: 8, justifyContent: 'flex-end' },
  hero: { marginTop: 26, border: `1px solid ${palette.border}`, borderRadius: 24, background: palette.panel, backdropFilter: 'blur(16px)', padding: 'clamp(18px, 5vw, 40px)', boxShadow: '0 20px 60px rgba(0,0,0,0.4)' },
  heroTitle: { margin: '8px 0 10px', fontSize: 'clamp(1.9rem, 6vw, 3.9rem)', lineHeight: 1.04, letterSpacing: '-0.02em' },
  subtitle: { color: palette.muted, maxWidth: 860, lineHeight: 1.6, fontSize: 'clamp(0.95rem, 2vw, 1.15rem)' },
  section: { marginTop: 28 },
  sectionTitle: { marginBottom: 12, fontSize: 'clamp(1.15rem, 2.4vw, 1.5rem)' },
  grid: { display: 'grid', gap: 12, gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))' },
  card: { border: `1px solid ${palette.border}`, borderRadius: 18, background: palette.panelSoft, padding: 16, backdropFilter: 'blur(14px)' },
  cta: { marginTop: 26, border: `1px solid ${palette.border}`, borderRadius: 20, background: 'linear-gradient(120deg, rgba(89,255,157,0.18), rgba(168,109,255,0.2))', padding: '22px 18px' }
};

function LinkBtn({ label, to, primary = false }: { label: string; to: AppRoute; primary?: boolean }) {
  return (
    <button
      onClick={() => navigate(to)}
      style={{
        borderRadius: 999,
        border: `1px solid ${primary ? palette.green : palette.border}`,
        padding: '10px 16px',
        background: primary ? 'linear-gradient(110deg, #59ff9d, #a86dff)' : 'rgba(255,255,255,0.02)',
        color: primary ? '#080910' : palette.text,
        fontWeight: 700,
        cursor: 'pointer'
      }}
    >
      {label}
    </button>
  );
}

const Section = ({ title, children }: { title: string; children: React.ReactNode }) => <section style={styles.section}><h2 style={styles.sectionTitle}>{title}</h2>{children}</section>;

function LandingPage() {
  return (
    <div style={styles.shell}>
      <section style={styles.hero}>
        <p style={{ color: palette.green, fontWeight: 700, margin: 0 }}>Koschei — The Immortal AI Platform</p>
        <h1 style={styles.heroTitle}>Build apps, games, websites, scripts, images, videos and voices with one immortal AI engine.</h1>
        <p style={styles.subtitle}>A premium AI SaaS platform for builders who need one command center for generation, routing and delivery.</p>
        <div style={{ display: 'flex', gap: 10, marginTop: 20, flexWrap: 'wrap' }}>
          <LinkBtn to="/platform" label="Start Building" primary />
          <LinkBtn to="/pricing" label="View Pricing" />
        </div>
      </section>

      <Section title="AI Tools">
        <div style={styles.grid}>{tools.map((tool) => <div key={tool.name} style={styles.card}><h3 style={{ marginTop: 0 }}>{tool.name}</h3><p style={{ color: palette.muted, marginBottom: 0 }}>{tool.description}</p></div>)}</div>
      </Section>

      <Section title="Together Model Router">
        <div style={styles.grid}>{models.map((model) => <div key={model} style={styles.card}><strong>{model}</strong></div>)}</div>
      </Section>

      <section style={styles.cta}>
        <h2 style={{ marginTop: 0 }}>One immortal engine. Multiple creative outputs.</h2>
        <p style={{ color: palette.muted }}>Route tasks by objective and unlock code, media and automation from a single interface.</p>
        <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
          <LinkBtn to="/platform" label="Start Building" primary />
          <LinkBtn to="/pricing" label="View Pricing" />
        </div>
      </section>
    </div>
  );
}

function PlatformPage() {
  return <div style={styles.shell}><section style={styles.hero}><h1 style={styles.heroTitle}>Platform / Tools</h1><p style={styles.subtitle}>Koschei workspace includes tool-specific lanes, model routing rules and project orchestration layers. Backend APIs stay in Go placeholder mode.</p><div style={styles.grid}>{tools.map((tool) => <div key={tool.name} style={styles.card}><h3 style={{ marginTop: 0 }}>{tool.name}</h3><p style={{ color: palette.muted }}>{tool.description}</p></div>)}</div></section></div>;
}

function PricingPage() {
  return <div style={styles.shell}><section style={styles.hero}><h1 style={styles.heroTitle}>Pricing</h1><p style={styles.subtitle}>Transparent TL pricing for creators, teams and studios.</p><div style={styles.grid}>{plans.map((plan) => <div key={plan.name} style={{ ...styles.card, outline: plan.highlight ? `1px solid ${palette.green}` : 'none' }}><h3 style={{ marginTop: 0 }}>{plan.name}</h3><p style={{ color: palette.green, fontWeight: 800 }}>{plan.price}</p><p style={{ color: palette.muted, marginBottom: 0 }}>{plan.detail}</p></div>)}</div></section></div>;
}

function DashboardPage() {
  return <div style={styles.shell}><section style={styles.hero}><h1 style={styles.heroTitle}>Dashboard</h1><p style={styles.subtitle}>Authenticated user dashboard placeholder. Connect to Go auth session and API gateways.</p></section></div>;
}

function BillingPage() {
  return <div style={styles.shell}><section style={styles.hero}><h1 style={styles.heroTitle}>Billing</h1><p style={styles.subtitle}>Payment automation is disabled for now. Manual activation after payment is required.</p><div style={styles.grid}><div style={styles.card}><h3>Manual activation after payment</h3><p style={{ color: palette.muted }}>After a payment is verified, owner panel assigns the selected plan and credits manually.</p></div><div style={styles.card}><h3>Shopier links (placeholder)</h3><p style={{ color: palette.muted, marginBottom: 8 }}>Builder: {shopierLinks.builder}</p><p style={{ color: palette.muted, marginBottom: 8 }}>Pro: {shopierLinks.pro}</p><p style={{ color: palette.muted, marginBottom: 0 }}>Studio: {shopierLinks.studio}</p></div></div></section></div>;
}

function OwnerPage() {
  return <div style={styles.shell}><section style={styles.hero}><h1 style={styles.heroTitle}>Private Owner God Mode</h1><p style={styles.subtitle}>Owner-only namespace for admin operations, manual plan assignment and credit management.</p><div style={styles.card}><strong>Role guard placeholder:</strong><p style={{ color: palette.muted }}>Only owner/admin role should access /owner routes. Public navigation never shows this route.</p></div></section></div>;
}

function App() {
  const [path, setPath] = useState(window.location.pathname);
  useEffect(() => {
    const onPop = () => setPath(window.location.pathname);
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);

  const route = useMemo<AppRoute>(() => {
    if (path === '/' || path === '/platform' || path === '/pricing' || path === '/dashboard' || path === '/billing' || path.startsWith('/owner')) {
      return (path.startsWith('/owner') ? '/owner' : path) as AppRoute;
    }
    return '/';
  }, [path]);

  const canUseProtected = session.role === 'user' || session.role === 'owner';
  const canUseOwner = session.role === 'owner';

  return (
    <div style={styles.app}>
      <header style={styles.nav}>
        <button onClick={() => navigate('/')} style={{ background: 'transparent', border: 'none', color: palette.text, fontWeight: 800, fontSize: '1rem', cursor: 'pointer' }}>Koschei</button>
        <nav style={styles.navLinks}>
          <LinkBtn to="/" label="Landing" />
          <LinkBtn to="/platform" label="Platform" />
          <LinkBtn to="/pricing" label="Pricing" />
          {canUseProtected && <LinkBtn to="/dashboard" label="Dashboard" />}
          {canUseProtected && <LinkBtn to="/billing" label="Billing" />}
        </nav>
      </header>

      {route === '/' && <LandingPage />}
      {route === '/platform' && <PlatformPage />}
      {route === '/pricing' && <PricingPage />}
      {route === '/dashboard' && (canUseProtected ? <DashboardPage /> : <DashboardPage />)}
      {route === '/billing' && (canUseProtected ? <BillingPage /> : <BillingPage />)}
      {route === '/owner' && (canUseOwner ? <OwnerPage /> : <div style={styles.shell}><section style={styles.hero}><h1 style={styles.heroTitle}>Unauthorized</h1><p style={styles.subtitle}>Owner God Mode is protected by owner/admin role guard placeholder.</p></section></div>)}
    </div>
  );
}

createRoot(document.getElementById('root')!).render(<App />);
