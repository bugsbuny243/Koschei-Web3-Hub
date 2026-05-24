import { useEffect, useMemo, useState } from 'react';
import { ScrollView, Text, View } from 'react-native';
import { Button, Card, ErrorState, Input } from '@/components/ui';
import { api } from '@/lib/api';
import { auth } from '@/lib/auth';
import { router } from 'expo-router';

const statusTextClass = (status: string) => {
  const normalized = String(status || '').toLowerCase();
  if (normalized === 'completed') return 'text-green-400';
  if (normalized === 'queued') return 'text-yellow-300';
  if (normalized === 'running') return 'text-blue-400';
  if (normalized === 'failed') return 'text-red-400';
  return 'text-zinc-300';
};

export default function Dashboard() {
  const [prompt, setPrompt] = useState('');
  const [error, setError] = useState('');
  const [credits, setCredits] = useState(0);
  const [plan, setPlan] = useState('free');
  const [projects, setProjects] = useState<any[]>([]);
  const [tasks, setTasks] = useState<any[]>([]);
  const [logs, setLogs] = useState<any[]>([]);
  const [email, setEmail] = useState('');

  const refreshRuntimeData = async () => {
    const projectRows: any[] = await api.getRuntimeProjects();
    const taskRows: any[] = await api.getRuntimeTasks();
    setProjects(projectRows);
    setTasks(taskRows);
    if (projectRows.length > 0) {
      const latestProject = projectRows[0];
      const logRows: any[] = await api.getRuntimeLogs(latestProject.id);
      setLogs(logRows);
      return;
    }
    setLogs([]);
  };

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
        await refreshRuntimeData();
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
      await api.createRuntimeProject({ title: `Project ${new Date().toISOString()}`, prompt });
      await refreshRuntimeData();
      setPrompt('');
    } catch (e: any) {
      setError(e.message);
    }
  };

  const projectsNewestFirst = useMemo(() => [...projects].reverse(), [projects]);
  const tasksNewestFirst = useMemo(() => [...tasks].reverse(), [tasks]);
  const logsNewestFirst = useMemo(() => [...logs].reverse(), [logs]);

  return (
    <View className="flex-1 bg-[#0a0a0a] p-4" style={{ backgroundColor: '#0a0a0a' }}>
      <View className="mx-auto w-full max-w-6xl gap-4">
        <View className="gap-4 md:flex-row">
          <View className="md:w-72">
            <Card>
              <Text className="text-sm text-zinc-400">Signed in as</Text>
              <Text className="mt-1 text-sm text-white">{email || 'Unknown user'}</Text>
              <View className="mt-4 border-t border-zinc-800 pt-4">
                <Text className="text-xs uppercase tracking-wider text-zinc-400">Plan</Text>
                <Text className="mt-1 text-lg font-semibold text-white">{plan}</Text>
              </View>
              <View className="mt-4">
                <Text className="text-xs uppercase tracking-wider text-zinc-400">Credits</Text>
                <Text className="mt-1 text-lg font-semibold text-white">{credits}</Text>
              </View>
              <View className="mt-5">
                <Button label="Logout" onPress={async () => { await auth.clearToken(); router.replace('/login'); }} />
              </View>
            </Card>
          </View>

          <View className="flex-1">
            <Card>
              <Text className="mb-2 text-base font-semibold text-white">Runtime Project Prompt</Text>
              <Input
                placeholder="Describe your runtime project..."
                value={prompt}
                onChangeText={setPrompt}
              />
              <View className="mt-3">
                <Button label="Create Runtime Project" onPress={send} />
              </View>
              {!!error && <View className="mt-3"><ErrorState text={error} /></View>}
            </Card>
          </View>
        </View>

        <ScrollView className="max-h-[65vh]" contentContainerStyle={{ gap: 12, paddingBottom: 24 }}>
          <Card>
            <Text className="text-lg font-semibold text-white">Projects ({projectsNewestFirst.length})</Text>
            <View className="mt-3 gap-2">
              {projectsNewestFirst.length === 0 && <Text className="text-zinc-500">No projects yet.</Text>}
              {projectsNewestFirst.map((p) => (
                <View key={p.id} className="flex-row items-center justify-between rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2">
                  <Text className="mr-3 flex-1 text-zinc-100">{p.title}</Text>
                  <Text className={statusTextClass(p.status)}>{String(p.status || 'unknown')}</Text>
                </View>
              ))}
            </View>
          </Card>

          <Card>
            <Text className="text-lg font-semibold text-white">Tasks ({tasksNewestFirst.length})</Text>
            <View className="mt-3 gap-2">
              {tasksNewestFirst.length === 0 && <Text className="text-zinc-500">No tasks yet.</Text>}
              {tasksNewestFirst.map((t) => (
                <View key={t.id} className="flex-row items-center justify-between rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2">
                  <Text className="mr-3 flex-1 text-zinc-100">{t.task_type}</Text>
                  <Text className={statusTextClass(t.status)}>{String(t.status || 'unknown')}</Text>
                </View>
              ))}
            </View>
          </Card>

          <Card>
            <Text className="text-lg font-semibold text-white">Logs ({logsNewestFirst.length})</Text>
            <View className="mt-3 gap-2">
              {logsNewestFirst.length === 0 && <Text className="text-zinc-500">No logs yet.</Text>}
              {logsNewestFirst.map((l) => (
                <View key={l.id} className="rounded-lg border border-zinc-800 bg-zinc-900 px-3 py-2">
                  <Text className="text-xs uppercase text-zinc-400">{String(l.level || 'info')}</Text>
                  <Text className="mt-1 text-zinc-100">{l.message}</Text>
                </View>
              ))}
            </View>
          </Card>
        </ScrollView>
      </View>
    </View>
  );
}
