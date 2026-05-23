import { useEffect, useState } from 'react';
import { ScrollView, Text, View } from 'react-native';
import { Sidebar } from '@/components/Sidebar';
import { Button, ErrorState, Input } from '@/components/ui';
import { api } from '@/lib/api';
import { auth } from '@/lib/auth';
import { router } from 'expo-router';

export default function Dashboard() {
  const [prompt, setPrompt] = useState('');
  const [error, setError] = useState('');
  const [credits, setCredits] = useState(0);
  const [plan, setPlan] = useState('free');
  const [projects, setProjects] = useState<any[]>([]);
  const [tasks, setTasks] = useState<any[]>([]);
  const [logs, setLogs] = useState<any[]>([]);
  const [email, setEmail] = useState('');

  useEffect(() => {
    const loadMe = async () => {
      const token = await auth.getToken();
      if (!token) {
        router.replace('/login');
        return;
      }
      try {
        const me: any = await api.me();
        if (me?.user) {
          setCredits(typeof me.user.credits === 'number' ? me.user.credits : 0);
          setPlan(typeof me.user.plan === 'string' && me.user.plan.trim() ? me.user.plan : 'free');
          setEmail(me.user.email || '');
        }
      } catch (e: any) {
        if (String(e?.message || '').includes('401')) {
          await auth.clearToken();
        }
        router.replace('/login');
      }
    };
    loadMe();
  }, []);

  const send = async () => {
    try {
      setError('');
      const created: any = await api.createRuntimeProject({ title: `Project ${new Date().toISOString()}`, prompt });
      const projectRows: any[] = await api.getRuntimeProjects();
      const taskRows: any[] = await api.getRuntimeTasks();
      const logRows: any[] = await api.getRuntimeLogs(created.project_id);
      setProjects(projectRows);
      setTasks(taskRows);
      setLogs(logRows);
      setPrompt('');
      
    } catch (e: any) {
      setError(e.message);
    }
  };

  return (
    <View className="flex-1 bg-[#0a0a0a] p-4 md:flex-row gap-4" style={{ backgroundColor: '#0a0a0a' }}>
      <Sidebar credits={credits} plan={plan} />
      <View><Text className="text-zinc-300" style={{ color: '#ffffff' }}>{email}</Text><Button label="Logout" onPress={async()=>{await auth.clearToken(); router.replace('/login');}} /></View>
      <View className="flex-1">
        <Text className="text-white mb-2" style={{ color: '#ffffff' }}>
          Runtime Project Prompt
        </Text>
        <Input placeholder="Describe your project..." value={prompt} onChangeText={setPrompt} />
        <View className="mt-2">
          <Button label="Create Runtime Project" onPress={send} />
        </View>
        {!!error && <ErrorState text={error} />}
        <ScrollView className="mt-4">
          <Text className="text-white text-lg" style={{ color: '#ffffff' }}>
            Projects ({projects.length})
          </Text>
          {projects.map((p) => (
            <Text key={p.id} className="text-zinc-300" style={{ color: '#ffffff' }}>
              {p.title} · {p.status}
            </Text>
          ))}
          <Text className="text-white text-lg mt-4" style={{ color: '#ffffff' }}>
            Tasks ({tasks.length})
          </Text>
          {tasks.map((t) => (
            <Text key={t.id} className="text-zinc-300" style={{ color: '#ffffff' }}>
              {t.task_type} · {t.status}
            </Text>
          ))}
          <Text className="text-white text-lg mt-4" style={{ color: '#ffffff' }}>
            Logs ({logs.length})
          </Text>
          {logs.map((l) => (
            <Text key={l.id} className="text-zinc-300" style={{ color: '#ffffff' }}>
              [{l.level}] {l.message}
            </Text>
          ))}
        </ScrollView>
      </View>
    </View>
  );
}
