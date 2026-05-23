import { useEffect, useState } from 'react';

type Project = { id:string; title:string; prompt:string; status:string; created_at:string };
type Task = { id:string; project_id:string; task_type:string; status:string; result?:string; error?:string };
type Log = { id:string; level:string; message:string; created_at:string };

const navigate = (to: string) => { window.history.pushState({}, '', to); window.dispatchEvent(new PopStateEvent('popstate')); };

function Home() {
  return <main className="container"><section className="hero premium-card"><h1>Koschei AI Production Engine</h1><p>Koschei routes each request through planning, generation, review and delivery stages.</p></section><section className="premium-card"><h2>Command Center</h2><div className="grid"><article className="card"><h3>Workflow Router</h3></article><article className="card"><h3>Parallel AI Workers</h3></article><article className="card"><h3>Credit Ledger</h3></article></div></section></main>
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
return <div><header className='topbar'><button className='logo' onClick={()=>navigate('/')}>KOSCHEI</button><nav><button className='btn' onClick={()=>navigate('/dashboard')}>Dashboard</button><button className='btn' onClick={()=>navigate('/pricing')}>Pricing</button></nav></header>{path==='/dashboard'?<Dashboard/>:path==='/owner'?<Owner/>:<Home/>}</div>}
