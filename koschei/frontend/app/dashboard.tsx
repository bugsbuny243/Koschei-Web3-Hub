import { useState } from 'react';
import { ScrollView, Text, View } from 'react-native';
import { Sidebar } from '@/components/Sidebar';
import { Button, ErrorState, Input } from '@/components/ui';
import { api } from '@/lib/api';

export default function Dashboard() {
  const [prompt,setPrompt]=useState(''); const [error,setError]=useState('');
  const [credits,setCredits]=useState(0); const [projects, setProjects] = useState<any[]>([]); const [tasks, setTasks] = useState<any[]>([]); const [logs, setLogs] = useState<any[]>([]);

  const send = async ()=>{
    try {
      setError('');
      const me:any = await api.me();
      const created:any = await api.createRuntimeProject({ email: me.email, title: `Project ${new Date().toISOString()}`, prompt });
      const projectRows:any[] = await api.getRuntimeProjects();
      const taskRows:any[] = await api.getRuntimeTasks();
      const logRows:any[] = await api.getRuntimeLogs(created.project_id);
      setProjects(projectRows);
      setTasks(taskRows);
      setLogs(logRows);
      setPrompt('');
      if (typeof me.credits === 'number') setCredits(me.credits);
    } catch(e:any){ setError(e.message); }
  };

  return <View className="flex-1 bg-[#0a0a0a] p-4 md:flex-row gap-4"><Sidebar credits={credits} plan="free"/><View className="flex-1"><Text className="text-white mb-2">Runtime Project Prompt</Text><Input placeholder="Describe your project..." value={prompt} onChangeText={setPrompt}/><View className="mt-2"><Button label="Create Runtime Project" onPress={send}/></View>{!!error && <ErrorState text={error}/>}<ScrollView className="mt-4"><Text className="text-white text-lg">Projects ({projects.length})</Text>{projects.map((p)=><Text key={p.id} className="text-zinc-300">{p.title} · {p.status}</Text>)}<Text className="text-white text-lg mt-4">Tasks ({tasks.length})</Text>{tasks.map((t)=><Text key={t.id} className="text-zinc-300">{t.task_type} · {t.status}</Text>)}<Text className="text-white text-lg mt-4">Logs ({logs.length})</Text>{logs.map((l)=><Text key={l.id} className="text-zinc-300">[{l.level}] {l.message}</Text>)}</ScrollView></View></View>;
}
