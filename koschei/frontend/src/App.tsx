import { useEffect, useState } from 'react';
import { api, apiConnected } from './lib/api';

const SHOPIER_LINKS = {
  starter: 'https://www.shopier.com/TradeVisual/47465449',
  pro: 'https://www.shopier.com/TradeVisual/47465484',
  studio: 'https://www.shopier.com/TradeVisual/47465499',
} as const;

type PackageId = 'starter' | 'pro' | 'studio';
type Project = { id: string; title: string; prompt: string; status: string; created_at: string };
type Task = { id: string; project_id: string; task_type: string; status: string; output_json?: unknown; error?: string };
type Log = { id: string; level: string; message: string; created_at: string };

const navigate = (to: string) => {
  window.history.pushState({}, '', to);
  window.dispatchEvent(new PopStateEvent('popstate'));
};

function Home() {
  return (
    <main className="container">
      <section className="hero premium-card">
        <div>
          <p className="eyebrow">AI Production Platform</p>
          <h1>Koschei AI Production Engine</h1>
          <h2 className="hero-headline">Ship structured AI production workflows from one command center.</h2>
          <p className="subtext">
            Koschei turns one request into a structured production workflow with planning, generation, review and
            delivery stages.
          </p>
          <div className="actions hero-actions">
            <button className="btn primary" onClick={() => navigate('/dashboard')}>
              Start Building
            </button>
            <button className="btn" onClick={() => navigate('/pricing')}>
              View Pricing
            </button>
          </div>
        </div>
        <aside className="mockup-card">
          <h3>Command Center Preview</h3>
          <p>Runtime: Queue active • Generation running • Delivery ready</p>
          <ul className="list-tight">
            <li>Planning lanes with queue visibility</li>
            <li>Review checkpoints before delivery</li>
            <li>Automation rules for output routing</li>
          </ul>
        </aside>
      </section>

      <section className="premium-card">
        <h2>AI Production Engine</h2>
        <div className="grid">
          <article className="card"><h3>Planning & Queue</h3><p>Turn requests into scoped tasks and assign priorities.</p></article>
          <article className="card"><h3>Generation & Review</h3><p>Produce assets, inspect outputs, and revise with confidence.</p></article>
          <article className="card"><h3>Automation & Delivery</h3><p>Route approvals to delivery paths and keep operations consistent.</p></article>
        </div>
      </section>
    </main>
  );
}

function Pricing() {
  return <main className="container page premium-card">
    <h1>Pricing</h1>
    <p>One-time credit packs designed for serious AI production velocity.</p>
    <section className="grid pricing-grid">
      <article className="card pricing-card">
        <h3>Free</h3><p className="credits">0 Credits</p><p className="chip">One-time credit pack</p><p className="price">0 TL</p>
        <ul className="list-tight"><li>Explore dashboard workflow</li><li>Test project setup</li><li>No payment needed</li></ul>
        <button className="btn primary" onClick={() => navigate('/dashboard')}>Start Free</button>
      </article>
      <article className="card pricing-card">
        <h3>Starter</h3><p className="credits">20.000 Credits</p><p className="chip">One-time credit pack</p><p className="price">899 TL</p>
        <ul className="list-tight"><li>Launch production-ready flows</li><li>Balanced for small teams</li><li>Manual activation support</li></ul>
        <a className="btn primary" href={SHOPIER_LINKS.starter} target="_blank" rel="noopener noreferrer">Buy 20.000 Credits</a>
      </article>
      <article className="card pricing-card featured">
        <h3>Pro</h3><p className="credits">70.000 Credits</p><p className="chip">One-time credit pack</p><p className="price">2.299 TL</p>
        <ul className="list-tight"><li>Scale parallel content pipelines</li><li>Faster throughput for teams</li><li>Best value for growing demand</li></ul>
        <a className="btn primary" href={SHOPIER_LINKS.pro} target="_blank" rel="noopener noreferrer">Buy 70.000 Credits</a>
      </article>
      <article className="card pricing-card">
        <h3>Studio</h3><p className="credits">180.000 Credits</p><p className="chip">One-time credit pack</p><p className="price">4.999 TL</p>
        <ul className="list-tight"><li>High-volume output operations</li><li>Built for studio-scale delivery</li><li>Priority operational continuity</li></ul>
        <a className="btn primary" href={SHOPIER_LINKS.studio} target="_blank" rel="noopener noreferrer">Buy 180.000 Credits</a>
      </article>
    </section>
  </main>;
}

function Billing() {
  const [email, setEmail] = useState('');
  const [plan, setPlan] = useState<PackageId>('starter');
  const [paymentReference, setPaymentReference] = useState('');
  const [note, setNote] = useState('');
  const [status, setStatus] = useState('');

  const submit = async () => {
    setStatus('Submitting...');
    if (!apiConnected) { setStatus('API not connected yet'); return; }
    try {
      await api.createPaymentRequest({ email, plan, payment_provider: 'Shopier', payment_reference: paymentReference, note });
      setStatus('Payment request sent. Credits will be activated manually after review.');
    } catch { setStatus('Failed to send request.'); }
  };

  return <main className='container page premium-card'>
    <h1>Billing</h1>
    <p>All paid options are one-time credit packs. After payment, submit your email and Shopier order reference for manual activation.</p>
    <section className='grid pricing-grid'>
      <article className='card pricing-card'><h3>Koschei Starter Pack</h3><p className='credits'>20.000 Credits</p><p className='price'>899 TL</p><a className='btn' href={SHOPIER_LINKS.starter} target='_blank' rel='noopener noreferrer'>Buy 20.000 Credits</a></article>
      <article className='card pricing-card'><h3>Koschei Pro Pack</h3><p className='credits'>70.000 Credits</p><p className='price'>2.299 TL</p><a className='btn' href={SHOPIER_LINKS.pro} target='_blank' rel='noopener noreferrer'>Buy 70.000 Credits</a></article>
      <article className='card pricing-card'><h3>Koschei Studio Pack</h3><p className='credits'>180.000 Credits</p><p className='price'>4.999 TL</p><a className='btn' href={SHOPIER_LINKS.studio} target='_blank' rel='noopener noreferrer'>Buy 180.000 Credits</a></article>
    </section>
    <section className='card form-card'>
      <h2>Manual Activation Form</h2>
      <div className='form-grid'>
        <input placeholder='email' value={email} onChange={e => setEmail(e.target.value)} />
        <select value={plan} onChange={e => setPlan(e.target.value as PackageId)}>
          <option value='starter'>Starter — 20.000 credits</option>
          <option value='pro'>Pro — 70.000 credits</option>
          <option value='studio'>Studio — 180.000 credits</option>
        </select>
        <input value='Shopier' disabled readOnly />
        <input placeholder='payment/order reference' value={paymentReference} onChange={e => setPaymentReference(e.target.value)} />
        <input placeholder='note' value={note} onChange={e => setNote(e.target.value)} />
        <button className='btn primary wide-btn' onClick={submit} disabled={!apiConnected}>Submit Manual Activation</button>
      </div>
      {!!status && <p>{status}</p>}
      <div className='trust-notes'>
        <span>Digital product</span><span>Manual activation</span><span>No physical delivery</span><span>One-time credit pack</span>
      </div>
    </section>
  </main>;
}

function Dashboard() {
  const [email, setEmail] = useState(''); const [title, setTitle] = useState(''); const [prompt, setPrompt] = useState('');
  const [projects, setProjects] = useState<Project[]>([]); const [tasks, setTasks] = useState<Task[]>([]); const [logs, setLogs] = useState<Log[]>([]); const [projectId, setProjectId] = useState('');
  const [statusMessage, setStatusMessage] = useState('');

  const refresh = async () => {
    if (!email) return;
    if (!apiConnected) return;
    const p = await api.getRuntimeProjects(email); setProjects(Array.isArray(p) ? p : []);
    const t = await api.getRuntimeTasks(email); setTasks(Array.isArray(t) ? t : []);
    return Array.isArray(p) ? p as Project[] : [];
  };
  const createProject = async () => {
    if (!apiConnected) return;
    setStatusMessage('Creating project...');
    try {
      const created = await api.createRuntimeProject({ email, title, prompt }) as { project_id?: string };
      setTitle('');
      setPrompt('');
      const refreshedProjects = await refresh();
      const latestProjectID = created.project_id || refreshedProjects?.[0]?.id;
      if (latestProjectID) await loadLogs(latestProjectID);
      setStatusMessage('Project created successfully.');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to create project.';
      setStatusMessage(`Create project failed: ${message}`);
    }
  };
  const loadLogs = async (pid: string) => { if (!apiConnected) return; setProjectId(pid); const l = await api.getRuntimeLogs(pid); setLogs(Array.isArray(l) ? l : []); };
  useEffect(() => { if (email) refresh(); }, []);

  return <main className="container page premium-card">
    <h1>Command Center Dashboard</h1>
    <div className='form-grid'>
      <input placeholder='email' value={email} onChange={e => setEmail(e.target.value)} />
      <input placeholder='project title' value={title} onChange={e => setTitle(e.target.value)} />
      <textarea placeholder='what do you want to build?' value={prompt} onChange={e => setPrompt(e.target.value)} rows={4} />
      <div className='actions'>
        <button className='btn' onClick={refresh} disabled={!apiConnected}>Refresh</button>
        <button className='btn primary wide-btn' onClick={createProject} disabled={!apiConnected}>Create Project</button>
      </div>
    </div>
    {!!statusMessage && <p>{statusMessage}</p>}
    <h2>Projects</h2>
    {projects.length === 0 ? <article className='card'>No projects yet</article> : projects.map(p => <article className='card row' key={p.id}><b>{p.title}</b><span>{p.status}</span><button className='btn' onClick={() => loadLogs(p.id)}>Logs</button></article>)}
    <h2>Tasks</h2>
    {tasks.length === 0 ? <article className='card'>No tasks yet</article> : tasks.map(t => <article className='card' key={t.id}><b>{t.task_type}</b> <span>{t.status}</span><p>{t.error || 'Pending...'}</p></article>)}
    <h2>Logs {projectId && `(Project ${projectId.slice(0, 8)})`}</h2>
    {logs.length === 0 ? <article className='card'>No logs yet</article> : logs.map(l => <article className='card' key={l.id}><b>[{l.level}]</b> {l.message}</article>)}
  </main>;
}

export default function App() {
  const [path, setPath] = useState(window.location.pathname);
  useEffect(() => { const f = () => setPath(window.location.pathname); window.addEventListener('popstate', f); return () => window.removeEventListener('popstate', f); }, []);
  return <div><header className='topbar'><button className='logo' onClick={() => navigate('/')}>KOSCHEI</button><nav><button className='btn' onClick={() => navigate('/dashboard')}>Dashboard</button><button className='btn' onClick={() => navigate('/pricing')}>Pricing</button><button className='btn' onClick={() => navigate('/billing')}>Billing</button></nav></header>{path === '/dashboard' ? <Dashboard /> : path === '/pricing' ? <Pricing /> : path === '/billing' ? <Billing /> : <Home />}</div>;
}
