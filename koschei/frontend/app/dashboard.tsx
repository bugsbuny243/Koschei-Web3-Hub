import { useEffect, useMemo, useState } from 'react';
import { Pressable, ScrollView, Text, View } from 'react-native';
import AsyncStorage from '@react-native-async-storage/async-storage';
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

const statusPriority = (status: string) => {
  const normalized = String(status || '').toLowerCase();
  if (normalized === 'completed') return 0;
  if (normalized === 'running') return 1;
  if (normalized === 'queued') return 2;
  if (normalized === 'failed') return 3;
  return 4;
};

const parseTimestamp = (item: any) => {
  const raw = item?.updated_at || item?.created_at || item?.timestamp || item?.createdAt || item?.updatedAt;
  if (!raw) return 0;
  const parsed = Date.parse(String(raw));
  return Number.isNaN(parsed) ? 0 : parsed;
};

export default function Dashboard() {
  const [prompt, setPrompt] = useState('');
  const [error, setError] = useState('');
  const [aiTool, setAiTool] = useState<'chat' | 'code' | 'reason'>('chat');
  const [aiPrompt, setAiPrompt] = useState('');
  const [aiResult, setAiResult] = useState('');
  const [aiError, setAiError] = useState('');
  const [aiLoading, setAiLoading] = useState(false);
  const [credits, setCredits] = useState(0);
  const [plan, setPlan] = useState('free');
  const [projects, setProjects] = useState<any[]>([]);
  const [tasks, setTasks] = useState<any[]>([]);
  const [logs, setLogs] = useState<any[]>([]);
  const [email, setEmail] = useState('');

  const sortedProjects = useMemo(() => {
    return [...projects].sort((a, b) => {
      const timeDiff = parseTimestamp(b) - parseTimestamp(a);
      if (timeDiff !== 0) return timeDiff;
      return statusPriority(a.status) - statusPriority(b.status);
    });
  }, [projects]);

  const newestProject = useMemo(() => (sortedProjects.length > 0 ? sortedProjects[0] : null), [sortedProjects]);

  const displayedTasks = useMemo(() => {
    const sortedAll = [...tasks].sort((a, b) => {
      const timeDiff = parseTimestamp(b) - parseTimestamp(a);
      if (timeDiff !== 0) return timeDiff;
      return statusPriority(a.status) - statusPriority(b.status);
    });

    if (newestProject?.id) {
      const filtered = sortedAll.filter((t) => t?.project_id === newestProject.id);
      if (filtered.length > 0) return filtered;
    }

    return sortedAll;
  }, [tasks, newestProject]);

  const taskGroups = useMemo(() => {
    if (newestProject?.id) {
      return [{ title: 'Latest project tasks', rows: displayedTasks }];
    }

    const latestCompleted = displayedTasks.filter((t) => String(t?.status || '').toLowerCase() === 'completed');
    const olderQueued = displayedTasks.filter((t) => String(t?.status || '').toLowerCase() === 'queued');
    const other = displayedTasks.filter((t) => !['completed', 'queued'].includes(String(t?.status || '').toLowerCase()));

    return [
      { title: 'Latest completed tasks', rows: latestCompleted },
      { title: 'Older queued tasks', rows: olderQueued },
      ...(other.length ? [{ title: 'Other tasks', rows: other }] : []),
    ];
  }, [displayedTasks, newestProject]);

  const displayedLogs = useMemo(() => {
    const sorted = [...logs].sort((a, b) => parseTimestamp(b) - parseTimestamp(a));
    return sorted.slice(0, 20);
  }, [logs]);

  const refreshRuntimeData = async () => {
    const projectRows: any[] = await api.getRuntimeProjects();
    const taskRows: any[] = await api.getRuntimeTasks();
    setProjects(projectRows);
    setTasks(taskRows);

    const sorted = [...projectRows].sort((a, b) => {
      const timeDiff = parseTimestamp(b) - parseTimestamp(a);
      if (timeDiff !== 0) return timeDiff;
      return statusPriority(a.status) - statusPriority(b.status);
    });

    if (sorted.length > 0) {
      const latestProject = sorted[0];
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

  const runAiTest = async () => {
    if (aiLoading) return;
    setAiError('');
    setAiResult('');
    setAiLoading(true);
    try {
      const token = await AsyncStorage.getItem('koschei_token');
      const response = await fetch(`${(process.env.EXPO_PUBLIC_API_URL || '').trim()}/api/ai/generate`, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${token || ''}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          tool: aiTool,
          prompt: aiPrompt,
        }),
      });

      const data = await response.json().catch(() => ({}));
      if (!response.ok) {
        const errorText = `${data?.error || data?.message || 'ai_request_failed'} (status ${response.status})`;
        setAiError(errorText);
        return;
      }

      setAiResult(typeof data?.result === 'string' ? data.result : JSON.stringify(data));
    } catch (e: any) {
      setAiError(e?.message || 'ai_request_failed');
    } finally {
      setAiLoading(false);
    }
  };

  return (
    <View className="flex-1 bg-[#020207] p-4" style={{ backgroundColor: '#020207' }}>
      <View className="absolute -left-16 top-28 h-72 w-72 rounded-full bg-cyan-500/10" />
      <View className="absolute -right-16 bottom-24 h-72 w-72 rounded-full bg-violet-500/10" />
      <View className="mx-auto w-full max-w-6xl gap-4">
        <View className="gap-4 md:flex-row">
          <View className="md:w-72">
            <Card>
              <Text className="text-xs uppercase tracking-[2px] text-cyan-300">Identity Core</Text>
              <Text className="mt-3 text-sm text-zinc-400">Signed in as</Text>
              <Text className="mt-1 text-sm text-white" numberOfLines={2}>{email || 'Signed in'}</Text>
              <View className="mt-4 border-t border-emerald-500/20 pt-4">
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
              <Text className="mb-2 text-base font-semibold text-white">Quantum Runtime Prompt</Text>
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

            <View className="mt-4">
              <Card>
                <Text className="mb-2 text-base font-semibold text-white">Phase 4 AI Test</Text>
                <Text className="mb-2 text-xs uppercase tracking-wide text-zinc-400">Tool</Text>
                <View className="mb-3 flex-row gap-2">
                  {(['chat', 'code', 'reason'] as const).map((tool) => (
                    <Pressable
                      key={tool}
                      onPress={() => setAiTool(tool)}
                      className={`rounded-xl border px-3 py-2 ${aiTool === tool ? 'border-cyan-300 bg-cyan-500/20' : 'border-cyan-500/20 bg-[#040812]'}`}
                    >
                      <Text className={`${aiTool === tool ? 'text-cyan-100' : 'text-zinc-300'}`}>{tool}</Text>
                    </Pressable>
                  ))}
                </View>
                <Input
                  placeholder="Enter prompt for AI tool..."
                  value={aiPrompt}
                  onChangeText={setAiPrompt}
                />
                <View className="mt-3">
                  <Button label={aiLoading ? 'Running AI...' : 'Run AI'} onPress={runAiTest} />
                </View>
                {!!aiError && <View className="mt-3"><ErrorState text={aiError} /></View>}
                <View className="mt-3 rounded-xl border border-violet-500/20 bg-[#040a15] p-3">
                  <Text className="text-xs uppercase tracking-wide text-zinc-400">Result</Text>
                  <Text className="mt-2 text-zinc-100">{aiLoading ? 'Loading...' : (aiResult || 'No result yet')}</Text>
                </View>
              </Card>
            </View>
          </View>
        </View>

        <ScrollView className="max-h-[65vh]" contentContainerStyle={{ gap: 12, paddingBottom: 24 }}>
          <Card>
            <Text className="text-lg font-semibold text-white">Runtime Projects ({sortedProjects.length})</Text>
            <View className="mt-3 gap-2">
              {sortedProjects.length === 0 && <Text className="text-zinc-500">No projects yet</Text>}
              {sortedProjects.map((p) => (
                <View key={p.id} className="flex-row items-start justify-between rounded-lg border border-emerald-500/20 bg-[#040a15] px-3 py-2">
                  <Text className="mr-3 flex-1 flex-wrap text-zinc-100">{p.title}</Text>
                  <Text className={statusTextClass(p.status)}>{String(p.status || 'unknown')}</Text>
                </View>
              ))}
            </View>
          </Card>

          <Card>
            <Text className="text-lg font-semibold text-white">Agent Task Queue ({displayedTasks.length})</Text>
            <View className="mt-3 gap-3">
              {displayedTasks.length === 0 && <Text className="text-zinc-500">No tasks yet</Text>}
              {taskGroups.map((group) => (
                <View key={group.title} className="gap-2">
                  <Text className="text-xs uppercase tracking-wide text-zinc-500">{group.title}</Text>
                  {group.rows.map((t) => (
                    <View key={t.id} className="flex-row items-start justify-between rounded-lg border border-cyan-500/20 bg-[#040a15] px-3 py-2">
                      <Text className="mr-3 flex-1 flex-wrap text-zinc-100">{t.task_type}</Text>
                      <Text className={statusTextClass(t.status)}>{String(t.status || 'unknown')}</Text>
                    </View>
                  ))}
                </View>
              ))}
            </View>
          </Card>

          <Card>
            <Text className="text-lg font-semibold text-white">System Logs ({displayedLogs.length})</Text>
            <View className="mt-3 gap-2">
              {displayedLogs.length === 0 && <Text className="text-zinc-500">No logs yet</Text>}
              {displayedLogs.map((l) => (
                <View key={l.id} className="rounded-lg border border-violet-500/20 bg-[#040a15] px-3 py-2">
                  <Text className="text-xs uppercase text-zinc-400">{String(l.level || 'info')}</Text>
                  <Text className="mt-1 flex-wrap text-zinc-100">{l.message}</Text>
                </View>
              ))}
            </View>
          </Card>
        </ScrollView>
      </View>
    </View>
  );
}
