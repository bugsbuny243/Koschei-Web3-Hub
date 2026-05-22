import React, { useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';

type Role = 'public' | 'user' | 'owner';
const currentRole: Role = 'public';

const tools = [
  'Code Generator', 'App Builder', 'Web Builder', 'Game Builder',
  'Image Studio', 'Video Studio', 'Voice Lab', 'Automation Workflows'
];

const modelRouter = [
  'Qwen3-Coder-480B — code, debug, refactor',
  'Llama 3.3-70B — chat and Turkish/English analysis',
  'DeepSeek V4 Pro — reasoning, strategy and math',
  'FLUX.2 Pro — high-quality image generation',
  'FLUX Kontext Pro — image editing',
  'Google Veo 3.0 — video with audio',
  'Kling 2.1 Pro — cinematic video',
  'Kokoro 82M — text-to-speech',
  'Whisper large-v3 — speech-to-text'
];

const pricing = [
  { name: 'Free', price: '0 TL' },
  { name: 'Builder', price: '899 TL / month' },
  { name: 'Pro', price: '2,299 TL / month' }
];

const navigate = (to: string) => {
  window.history.pushState({}, '', to);
  window.dispatchEvent(new PopStateEvent('popstate'));
};

const styles: Record<string, React.CSSProperties> = {
  app: { minHeight: '100vh', background: 'radial-gradient(circle at 10% 0%, #251641 0%, #090710 40%, #04050a 100%)', color: '#eef0ff', fontFamily: 'Inter,system-ui,sans-serif' },
  shell: { maxWidth: 1150, margin: '0 auto', padding: '0 16px 56px' },
  nav: { position: 'sticky', top: 0, zIndex: 20, display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '14px 16px', background: 'rgba(8,8,16,.75)', borderBottom: '1px solid rgba(126,99,255,.3)', backdropFilter: 'blur(10px)' },
  row: { display: 'flex', gap: 10, flexWrap: 'wrap' },
  hero: { padding: '76px 0 32px' },
  h1: { fontSize: 'clamp(2.1rem, 5vw, 3.8rem)', lineHeight: 1.05, margin: '8px 0 12px' },
  sub: { color: '#afb5d9', lineHeight: 1.6, maxWidth: 880, fontSize: 'clamp(1rem, 2vw, 1.2rem)' },
  section: { marginTop: 30 },
  sectionTitle: { fontSize: '1.4rem', marginBottom: 12 },
  grid: { display: 'grid', gridTemplateColumns: 'repeat(auto-fit,minmax(220px,1fr))', gap: 14 },
  card: { borderRadius: 18, padding: 16, background: 'linear-gradient(160deg, rgba(255,255,255,.1), rgba(255,255,255,.03))', border: '1px solid rgba(93,255,149,.24)', boxShadow: '0 10px 30px rgba(0,0,0,.28)' },
};

const Button = ({ label, to, primary = false }: { label: string; to: string; primary?: boolean }) => (
  <button onClick={() => navigate(to)} style={{ padding: '11px 16px', borderRadius: 999, border: primary ? '1px solid #62ff95' : '1px solid rgba(166,137,255,.5)', background: primary ? 'linear-gradient(90deg,#62ff95,#9f6bff)' : 'transparent', color: primary ? '#04050a' : '#eef0ff', cursor: 'pointer', fontWeight: 700 }}>
    {label}
  </button>
);

const Placeholder = ({ title, text }: { title: string; text: string }) => (
  <main style={styles.shell}><section style={{ ...styles.section, paddingTop: 30 }}><h1>{title}</h1><div style={styles.card}>{text}</div></section></main>
);

function Home() {
  return <main style={styles.shell}>
    <section style={styles.hero}>
      <p style={{ color: '#62ff95', margin: 0 }}>Koschei — The Immortal AI Platform</p>
      <h1 style={styles.h1}>Build apps, games, websites, scripts, images, videos and voices with one immortal AI engine.</h1>
      <p style={styles.sub}>Koschei combines code generation, AI chat, deep reasoning, image generation, video generation, voice tools and automation into one production platform.</p>
      <div style={{ ...styles.row, marginTop: 24 }}><Button label='Start Building' to='/platform' primary /><Button label='View Pricing' to='/pricing' /></div>
    </section>

    <section style={styles.section}><h2 style={styles.sectionTitle}>AI Tools Grid</h2><div style={styles.grid}>{tools.map((t) => <article key={t} style={styles.card}><h3 style={{ marginTop: 0 }}>{t}</h3></article>)}</div></section>
    <section style={styles.section}><h2 style={styles.sectionTitle}>Smart model router</h2><div style={styles.grid}>{modelRouter.map((m) => <article key={m} style={styles.card}>{m}</article>)}</div></section>
    <section style={styles.section}><h2 style={styles.sectionTitle}>Public SaaS</h2><article style={styles.card}>Normal users access Koschei through monthly plans and usage credits, while advanced AI execution is routed through backend services.</article></section>
    <section style={styles.section}><h2 style={styles.sectionTitle}>Pricing preview</h2><div style={styles.grid}>{pricing.map((p) => <article key={p.name} style={styles.card}><h3 style={{ marginTop: 0 }}>{p.name}</h3><p style={{ marginBottom: 0, color: '#62ff95', fontWeight: 700 }}>{p.price}</p></article>)}</div></section>
    <section style={{ ...styles.section, ...styles.card, marginTop: 38 }}><h2 style={{ marginTop: 0 }}>Start building with Koschei.</h2><div style={styles.row}><Button label='Start Building' to='/platform' primary /><Button label='View Pricing' to='/pricing' /></div></section>
  </main>;
}

function App() {
  const [path, setPath] = useState(window.location.pathname);
  React.useEffect(() => {
    const onPop = () => setPath(window.location.pathname);
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);

  const route = useMemo(() => ['/', '/platform', '/dashboard', '/pricing', '/billing'].includes(path) || path.startsWith('/owner') ? path : '/', [path]);
  const canUser = currentRole === 'user' || currentRole === 'owner';
  const canOwner = currentRole === 'owner';

  return <div style={styles.app}>
    <header style={styles.nav}>
      <button onClick={() => navigate('/')} style={{ border: 'none', background: 'none', color: '#fff', fontWeight: 800, cursor: 'pointer' }}>Koschei</button>
      <div style={styles.row}>
        <Button label='Home' to='/' />
        <Button label='Platform' to='/platform' />
        <Button label='Pricing' to='/pricing' />
        {canUser && <><Button label='Dashboard' to='/dashboard' /><Button label='Billing' to='/billing' /></>}
        {canOwner && <Button label='Owner' to='/owner' />}
      </div>
    </header>

    {route === '/' && <Home />}
    {route === '/platform' && <Placeholder title='Platform' text='Public route placeholder. Future AI calls must go through Go backend APIs only.' />}
    {route === '/pricing' && <Placeholder title='Pricing' text='Public pricing route placeholder.' />}
    {route === '/dashboard' && (canUser ? <Placeholder title='Dashboard' text='Protected dashboard placeholder route.' /> : <Placeholder title='Access Required' text='Dashboard requires authenticated user role.' />)}
    {route === '/billing' && (canUser ? <Placeholder title='Billing' text='Protected billing placeholder route.' /> : <Placeholder title='Access Required' text='Billing requires authenticated user role.' />)}
    {route.startsWith('/owner') && (canOwner ? <Placeholder title='Owner Panel' text='Private /owner route placeholder for admin-only tools.' /> : <Placeholder title='Unauthorized' text='Owner routes are private and never shown to public users.' />)}
  </div>;
}

createRoot(document.getElementById('root')!).render(<App />);
