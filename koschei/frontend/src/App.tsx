import { useEffect, useState } from 'react';
import { api, tokenStore } from './lib/api';

const navigate=(p:string)=>{history.pushState({},'',p); dispatchEvent(new PopStateEvent('popstate'));};
function Auth({mode}:{mode:'login'|'register'}){const [email,setEmail]=useState('');const [password,setPassword]=useState('');const [msg,setMsg]=useState('');
const submit=async()=>{try{const r= mode==='login'? await api.login({email,password}) : await api.register({email,password}); tokenStore.set(r.token); navigate('/dashboard');}catch(e){setMsg((e as Error).message)}};
return <main className='container page premium-card'><h1>{mode==='login'?'Login':'Register'}</h1><input placeholder='email' value={email} onChange={e=>setEmail(e.target.value)}/><input placeholder='password' type='password' value={password} onChange={e=>setPassword(e.target.value)}/><button className='btn primary' onClick={submit}>{mode}</button><p>{msg}</p></main>}
function Dashboard(){const [me,setMe]=useState<any>(null);const [title,setTitle]=useState('');const [prompt,setPrompt]=useState('');const [projects,setProjects]=useState<any[]>([]);const [tasks,setTasks]=useState<any[]>([]);const [logs,setLogs]=useState<any[]>([]);
const load=async()=>{const m=await api.me(); setMe(m); const p=await api.getRuntimeProjects(m.email); setProjects(p); const t=await api.getRuntimeTasks(m.email); setTasks(t); if(p[0]) setLogs(await api.getRuntimeLogs(p[0].id));};
useEffect(()=>{load().catch(()=>navigate('/login'));},[]);
const create=async()=>{if(!me) return; await api.createRuntimeProject({email:me.email,title,prompt}); await load();};
return <main className='container page premium-card'><h1>Dashboard</h1><p><b>Plan:</b> {me?.plan||'free'} | <b>Credits:</b> {me?.credits??0}</p><button className='btn' onClick={()=>{tokenStore.clear();navigate('/login')}}>Logout</button>
<h2>Chat / Generation</h2><textarea rows={4} value={prompt} onChange={e=>setPrompt(e.target.value)} /><input placeholder='Project title' value={title} onChange={e=>setTitle(e.target.value)} /><button className='btn primary' onClick={create}>Create Project</button>
<h3>Projects</h3>{projects.map(p=><article key={p.id} className='card'>{p.title} - {p.status}</article>)}
<h3>Tasks</h3>{tasks.map(t=><article key={t.id} className='card'>{t.task_type} - {t.status}</article>)}
<h3>Logs</h3>{logs.map(l=><article key={l.id} className='card'>[{l.level}] {l.message}</article>)}
</main>}

export default function App(){const [path,setPath]=useState(location.pathname); useEffect(()=>{const f=()=>setPath(location.pathname); addEventListener('popstate',f); return ()=>removeEventListener('popstate',f);},[]);
return <div><header className='topbar'><button className='logo' onClick={()=>navigate('/')}>KOSCHEI</button><nav><button className='btn' onClick={()=>navigate('/login')}>Login</button><button className='btn' onClick={()=>navigate('/register')}>Register</button><button className='btn' onClick={()=>navigate('/dashboard')}>Dashboard</button><button className='btn' onClick={()=>navigate('/pricing')}>Pricing</button><button className='btn' onClick={()=>navigate('/billing')}>Billing</button></nav></header>
{path==='/login'?<Auth mode='login'/>:path==='/register'?<Auth mode='register'/>:path==='/dashboard'?<Dashboard/>:<main className='container premium-card'><h1>Koschei</h1></main>}</div>}
