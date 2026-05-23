import { useEffect, useState } from 'react';

const SHOPIER_LINKS = {
  starter: 'https://www.shopier.com/TradeVisual/47465449',
  pro: 'https://www.shopier.com/TradeVisual/47465484',
  studio: 'https://www.shopier.com/TradeVisual/47465499',
} as const;

type PackageId = 'starter' | 'pro' | 'studio';
type Project = { id:string; title:string; prompt:string; status:string; created_at:string };
type Task = { id:string; project_id:string; task_type:string; status:string; result?:string; error?:string };
type Log = { id:string; level:string; message:string; created_at:string };

const navigate = (to: string) => { window.history.pushState({}, '', to); window.dispatchEvent(new PopStateEvent('popstate')); };

function Home() {
  return <main className="container">
    <section className="hero premium-card">
      <div>
        <h1>Koschei AI Production Engine</h1>
        <p>Koschei routes each request through planning, generation, review, automation and delivery stages inside one AI command center.</p>
      </div>
    </section>
    <section className="premium-card">
      <h2>Command Center</h2>
      <div className="grid">
        <article className="card"><h3>Planning & Queue</h3></article>
        <article className="card"><h3>Generation & Review</h3></article>
        <article className="card"><h3>Automation & Delivery</h3></article>
      </div>
    </section>
  </main>;
}

function Pricing() {
  return <main className='container page premium-card'>
    <h1>Pricing</h1>
    <p>Choose a one-time credit pack and pay securely on Shopier.</p>
    <section className='grid'>
      <article className='card'>
        <h3>Free</h3>
        <p>0 Credits</p>
        <p className='price'>0 TL</p>
        <button className='btn primary' onClick={() => navigate('/dashboard')}>Start Free</button>
      </article>
      <article className='card'>
        <h3>Starter</h3>
        <p>20.000 Credits</p>
        <p className='price'>899 TL</p>
        <a className='btn primary' href={SHOPIER_LINKS.starter} target='_blank' rel='noopener noreferrer'>Buy 20.000 Credits</a>
      </article>
      <article className='card'>
        <h3>Pro</h3>
        <p>70.000 Credits</p>
        <p className='price'>2.299 TL</p>
        <a className='btn primary' href={SHOPIER_LINKS.pro} target='_blank' rel='noopener noreferrer'>Buy 70.000 Credits</a>
      </article>
      <article className='card'>
        <h3>Studio</h3>
        <p>180.000 Credits</p>
        <p className='price'>4.999 TL</p>
        <a className='btn primary' href={SHOPIER_LINKS.studio} target='_blank' rel='noopener noreferrer'>Buy 180.000 Credits</a>
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
    const res = await fetch('/api/billing/manual-payment-request', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, plan, payment_provider: 'Shopier', payment_reference: paymentReference, note }),
    });
    setStatus(res.ok ? 'Payment request sent. Credits will be activated manually after review.' : 'Failed to send request.');
  };

  return <main className='container page premium-card'>
    <h1>Billing</h1>
    <p>All paid options are one-time credit packs.</p>
    <section className='grid'>
      <article className='card'><h3>Koschei Starter Pack</h3><p>20.000 Credits</p><p className='price'>899 TL</p><a className='btn' href={SHOPIER_LINKS.starter} target='_blank' rel='noopener noreferrer'>Buy on Shopier</a></article>
      <article className='card'><h3>Koschei Pro Pack</h3><p>70.000 Credits</p><p className='price'>2.299 TL</p><a className='btn' href={SHOPIER_LINKS.pro} target='_blank' rel='noopener noreferrer'>Buy on Shopier</a></article>
      <article className='card'><h3>Koschei Studio Pack</h3><p>180.000 Credits</p><p className='price'>4.999 TL</p><a className='btn' href={SHOPIER_LINKS.studio} target='_blank' rel='noopener noreferrer'>Buy on Shopier</a></article>
    </section>
    <p>After payment, return to this page and submit your Koschei account email and Shopier payment/order reference. Your credits will be activated manually by the owner.</p>
    <section className='card'>
      <h2>Manual Activation Form</h2>
      <div className='grid'>
        <input placeholder='email' value={email} onChange={e => setEmail(e.target.value)} />
        <select value={plan} onChange={e => setPlan(e.target.value as PackageId)}>
          <option value='starter'>Starter — 20.000 credits</option>
          <option value='pro'>Pro — 70.000 credits</option>
          <option value='studio'>Studio — 180.000 credits</option>
        </select>
        <input value='Shopier' disabled readOnly />
        <input placeholder='payment reference / order number' value={paymentReference} onChange={e => setPaymentReference(e.target.value)} />
        <input placeholder='note' value={note} onChange={e => setNote(e.target.value)} />
        <button className='btn primary' onClick={submit}>Submit Manual Activation</button>
      </div>
      {status && <p>{status}</p>}
    </section>
  </main>;
}

function Dashboard() {
  const [email,setEmail]=useState(''); const [title,setTitle]=useState(''); const [prompt,setPrompt]=useState('');
  const [projects,setProjects]=useState<Project[]>([]); const [tasks,setTasks]=useState<Task[]>([]); const [logs,setLogs]=useState<Log[]>([]); const [projectId,setProjectId]=useState('');
  const refresh = async () => {
    if (!email) return;
    const p=await fetch(`/api/runtime/projects?email=${encodeURIComponent(email)}`).then(r=>r.json()); setProjects(Array.isArray(p)?p:[]);
    const t=await fetch(`/api/runtime/tasks?email=${encodeURIComponent(email)}`).then(r=>r.json()); setTasks(Array.isArray(t)?t:[]);
  };
  const createProject = async () => {
    await fetch('/api/runtime/projects',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({email,title,prompt})});
    setTitle(''); setPrompt(''); refresh();
  };
  const loadLogs = async (pid:string) => { setProjectId(pid); const l=await fetch(`/api/runtime/logs/${pid}`).then(r=>r.json()); setLogs(Array.isArray(l)?l:[]); };
  useEffect(()=>{ if(email) refresh(); },[]);
  return <main className="container page premium-card"><h1>Command Center Dashboard</h1><div className="actions"><input placeholder='email' value={email} onChange={e=>setEmail(e.target.value)} /><button className='btn' onClick={refresh}>Refresh</button></div><div className='grid'><input placeholder='Project title' value={title} onChange={e=>setTitle(e.target.value)} /><input placeholder='Prompt' value={prompt} onChange={e=>setPrompt(e.target.value)} /><button className='btn primary' onClick={createProject}>Create Project</button></div><h2>Projects</h2>{projects.map(p=><article className='card' key={p.id}><b>{p.title}</b> <span>{p.status}</span> <button className='btn' onClick={()=>loadLogs(p.id)}>Logs</button></article>)}<h2>Tasks</h2>{tasks.map(t=><article className='card' key={t.id}><b>{t.task_type}</b> <span>{t.status}</span><p>{t.result || t.error || 'Pending...'}</p></article>)}<h2>Logs {projectId && `(Project ${projectId.slice(0,8)})`}</h2>{logs.map(l=><article className='card' key={l.id}><b>[{l.level}]</b> {l.message}</article>)}</main>
}

function Owner() { return <main className='container page premium-card'><h1>Owner Cockpit / God Mode</h1><p>Private controls for payment requests, users/credits, runtime projects, runtime tasks, retry/cancel.</p></main>; }

export default function App(){ const [path,setPath]=useState(window.location.pathname); useEffect(()=>{const f=()=>setPath(window.location.pathname); window.addEventListener('popstate',f); return ()=>window.removeEventListener('popstate',f)},[]);
return <div><header className='topbar'><button className='logo' onClick={()=>navigate('/')}>KOSCHEI</button><nav><button className='btn' onClick={()=>navigate('/dashboard')}>Dashboard</button><button className='btn' onClick={()=>navigate('/pricing')}>Pricing</button><button className='btn' onClick={()=>navigate('/billing')}>Billing</button></nav></header>{path==='/dashboard'?<Dashboard/>:path==='/pricing'?<Pricing/>:path==='/billing'?<Billing/>:path==='/owner'?<Owner/>:<Home/>}</div>}
