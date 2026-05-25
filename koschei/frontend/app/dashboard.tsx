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
  if (normalized === 'running' || normalized === 'processing') return 'text-cyan-300';
  if (normalized === 'review_needed') return 'text-amber-300';
  if (normalized === 'failed') return 'text-red-400';
  return 'text-zinc-300';
};

const statusPriority = (status: string) => {
  const normalized = String(status || '').toLowerCase();
  if (normalized === 'completed') return 0;
  if (normalized === 'running' || normalized === 'processing') return 1;
  if (normalized === 'review_needed') return 2;
  if (normalized === 'queued') return 3;
  if (normalized === 'failed') return 4;
  return 5;
};

const parseTimestamp = (item: any) => {
  const raw = item?.updated_at || item?.created_at || item?.timestamp || item?.createdAt || item?.updatedAt;
  if (!raw) return 0;
  const parsed = Date.parse(String(raw));
  return Number.isNaN(parsed) ? 0 : parsed;
};



const readProfileField = <T,>(user: any, lower: string, upper: string, fallback: T): T => {
  const value = user?.[lower] ?? user?.[upper];
  return (value ?? fallback) as T;
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
  const [aiJobs, setAiJobs] = useState<any[]>([]);
  const [email, setEmail] = useState('');
  const [latestOutput, setLatestOutput] = useState<any>(null);
  const [runtimeLoading, setRuntimeLoading] = useState(false);

  const safeJson = (value: any) => {
    if (value && typeof value === 'object') return value;
    if (typeof value === 'string') {
      try {
        return JSON.parse(value);
      } catch {
        return {};
      }
    }
    return {};
  };

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
    const sortedByTime = [...projectRows].sort((a, b) => parseTimestamp(b) - parseTimestamp(a));
    const latestSuccessfulProject = sortedByTime.find((p) => ['completed', 'review_needed'].includes(String(p?.status || '').toLowerCase()));
    const latestFailedProject = sortedByTime.find((p) => String(p?.status || '').toLowerCase() === 'failed');
    const newest = sortedByTime[0];

    if (newest && String(newest?.status || '').toLowerCase() === 'failed' && latestFailedProject?.id === newest.id) {
      const failedLogs: any[] = await api.getRuntimeLogs(newest.id);
      const latestFailureLog = [...failedLogs].sort((a, b) => parseTimestamp(b) - parseTimestamp(a)).find((l) => String(l?.level || '').toLowerCase() === 'error');
      setLatestOutput({ failed_message: latestFailureLog?.message || 'Runtime project failed.' });
    } else if (latestSuccessfulProject?.id) {
      const projectTasks = taskRows.filter((t) => t?.project_id === latestSuccessfulProject.id);
      const blueprintTask = projectTasks.find((t) => String(t?.task_type || '').toLowerCase() === 'blueprint');
      if (blueprintTask?.output_json) {
        const output = safeJson(blueprintTask.output_json);
        const arch = safeJson(projectTasks.find((t) => t?.task_type === 'architecture')?.output_json);
        const steps = safeJson(projectTasks.find((t) => t?.task_type === 'build_steps')?.output_json);
        const review = safeJson(projectTasks.find((t) => t?.task_type === 'review')?.output_json);
        const delivery = safeJson(projectTasks.find((t) => t?.task_type === 'delivery')?.output_json);
        setLatestOutput({
          ...output,
          required_infrastructure: arch?.output?.required_infrastructure || arch?.required_infrastructure || [],
          proposed_tool_calls: steps?.output?.proposed_tool_calls || [],
          review_status: review?.output?.review_status || '-',
          guardrail_flags: review?.output?.guardrail_flags || [],
          delivery_package: delivery?.output?.delivery_package || [],
          next_steps: delivery?.output?.next_steps || [],
          raw_ai_output: delivery?.raw_ai_output || output?.raw_ai_output,
          project_status: String(latestSuccessfulProject?.status || '').toLowerCase(),
        });
      } else {
        setLatestOutput(null);
      }
    } else {
      setLatestOutput(null);
    }

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

  const refreshMe = async () => {
    const me: any = await api.me();
    if (me?.user) {
      const profileCredits = readProfileField<number>(me.user, 'credits', 'Credits', 0);
      const profilePlan = readProfileField<string>(me.user, 'plan', 'Plan', 'free');
      const profileEmail = readProfileField<string>(me.user, 'email', 'Email', '');
      setCredits(typeof profileCredits === 'number' ? profileCredits : 0);
      setPlan(typeof profilePlan === 'string' && profilePlan.trim() ? profilePlan : 'free');
      setEmail(typeof profileEmail === 'string' ? profileEmail : '');
    }
  };

  const refreshAiJobs = async () => {
    try {
      const rows: any = await api.get('/api/ai/jobs');
      setAiJobs(Array.isArray(rows?.jobs) ? rows.jobs : []);
    } catch {
      setAiJobs([]);
    }
  };

  useEffect(() => {
    const loadMe = async () => {
      const token = await auth.getToken();
      if (!token) {
        router.replace('/login');
        return;
      }
      try {
        await refreshMe();
        await refreshRuntimeData();
        await refreshAiJobs();
      } catch (e: any) {
        if (String(e?.message || '').includes('401')) {
          await auth.clearToken();
          router.replace('/login');
          return;
        }
        setAiJobs([]);
      }
    };
    loadMe();
  }, []);


  useEffect(() => {
    const hasActiveProjects = projects.some((p) => ['running', 'processing'].includes(String(p?.status || '').toLowerCase()));
    if (!hasActiveProjects) {
      refreshMe().catch(() => {});
      refreshAiJobs().catch(() => {});
      return;
    }
    const timer = setInterval(() => {
      refreshRuntimeData().catch(() => {});
    }, 3000);
    return () => clearInterval(timer);
  }, [projects]);

  const send = async () => {
    if (runtimeLoading) return;
    try {
      setRuntimeLoading(true);
      setError('');
      const data: any = await api.createRuntimeProject({ title: `Project ${new Date().toISOString()}`, prompt });
      if (String(data?.status || '').toLowerCase() === 'running') {
        setError('Runtime project queued. Processing...');
      }
      await refreshRuntimeData();
      setPrompt('');
    } catch (e: any) {
      const detailRaw = typeof e?.data?.detail === 'string' ? e.data.detail.trim() : '';
      const timeoutLike = /timeout|context deadline exceeded|client\.timeout exceeded/i.test(detailRaw);
      const friendly = timeoutLike ? 'Runtime AI provider timed out. Credits not charged. Try again or use a shorter prompt.' : '';
      const backendDetail = detailRaw ? `: ${detailRaw}` : '';
      const base = e?.message || 'runtime_request_failed';
      const creditInfo = e?.data?.credits_charged === false ? ' — Credits not charged' : '';
      const composed = `${base}${backendDetail}${creditInfo}`;
      setError(friendly ? `${friendly} | ${composed}` : composed);
      await refreshMe();
    } finally {
      setRuntimeLoading(false);
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
        const detail = typeof data?.detail === 'string' && data.detail.trim() ? ` — ${data.detail}` : '';
        const creditInfo = data?.credits_charged === false ? ' — Credits not charged' : '';
        const errorText = `${data?.error || data?.message || 'ai_request_failed'}${detail}${creditInfo} (status ${response.status})`;
        setAiError(errorText);
        return;
      }

      setAiResult(typeof data?.result === 'string' ? data.result : JSON.stringify(data));
      await refreshAiJobs();

    } catch (e: any) {
      setAiError(e?.message || 'ai_request_failed');
    } finally {
      try {
        await refreshMe();
        await refreshAiJobs();
      } catch (_) {}
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
                <Button label={runtimeLoading ? 'Queueing Runtime Project...' : 'Create Runtime Project'} onPress={send} disabled={runtimeLoading} />
              </View>
              {!!error && <View className="mt-3"><ErrorState text={error} /></View>}
            </Card>

            <View className="mt-4">
              <Card>
                <Text className="mb-2 text-base font-semibold text-white">AI Debug Console (Phase 4 AI Test)</Text>
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
                  <Text selectable className="mt-2 text-zinc-100" style={{ lineHeight: 22 }}>{aiLoading ? 'Loading...' : (aiResult || 'No result yet')}</Text>
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
            <Text className="text-lg font-semibold text-white">Latest Runtime Output</Text>
            {!latestOutput && <Text className="mt-2 text-zinc-500">No completed output yet</Text>}
            {!!latestOutput && !latestOutput.failed_message && (
              <View className="mt-2 gap-2">
                <Text className="text-zinc-100">Title: {String(latestOutput.project_title || '-')}</Text>
                <Text className="text-zinc-100">Type: {String(latestOutput.project_type || '-')}</Text>
                <Text className="text-zinc-100">MVP: {Array.isArray(latestOutput?.output?.mvp_scope || latestOutput.mvp_scope) ? (latestOutput?.output?.mvp_scope || latestOutput.mvp_scope).join(' • ') : '-'}</Text>
                <Text className="text-zinc-100">Infrastructure: {Array.isArray(latestOutput.required_infrastructure) ? latestOutput.required_infrastructure.join(' • ') : '-'}</Text>
                <Text className="text-zinc-100">Proposed tool calls (Not executed yet): {Array.isArray(latestOutput.proposed_tool_calls) ? latestOutput.proposed_tool_calls.map((c: any) => c?.tool_name).filter(Boolean).join(' • ') : '-'}</Text>
                <Text className="text-zinc-100">Review Status: {String(latestOutput.review_status || '-')}</Text>
                <Text className="text-zinc-100">Guardrail Flags: {Array.isArray(latestOutput.guardrail_flags) ? latestOutput.guardrail_flags.join(' • ') : '-'}</Text>
                <Text className="text-zinc-100">Delivery Package: {Array.isArray(latestOutput.delivery_package) ? latestOutput.delivery_package.join(' • ') : '-'}</Text>
                <Text className="text-zinc-100">Next Steps: {Array.isArray(latestOutput.next_steps) ? latestOutput.next_steps.join(' • ') : '-'}</Text>
                {!!latestOutput.raw_ai_output && (
                  <View className="rounded-lg border border-cyan-500/20 bg-[#040a15] p-2">
                    <Text className="text-xs uppercase tracking-wide text-zinc-400">Raw AI Output</Text>
                    <Text selectable className="mt-1 text-xs text-zinc-300" style={{ lineHeight: 18 }}>{String(latestOutput.raw_ai_output)}</Text>
                  </View>
                )}
              </View>
            )}
            {!!latestOutput?.failed_message && (
              <View className="mt-2 gap-2">
                <Text className="text-red-300">Latest runtime failed: {String(latestOutput.failed_message)}</Text>
              </View>
            )}
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
            <Text className="text-lg font-semibold text-white">AI Jobs ({aiJobs.length})</Text>
            <View className="mt-3 gap-2">
              {aiJobs.length === 0 && <Text className="text-zinc-500">No AI jobs yet</Text>}
              {aiJobs.map((j) => (
                <View key={j.id} className="rounded-lg border border-cyan-500/20 bg-[#040a15] px-3 py-2">
                  <View className="flex-row items-start justify-between">
                    <Text className="text-zinc-100">{String(j.tool || 'unknown')}</Text>
                    <Text className={statusTextClass(j.status)}>{String(j.status || 'unknown')}</Text>
                  </View>
                  <Text className="mt-1 text-xs text-zinc-400" numberOfLines={2}>{String(j.prompt || '')}</Text>
                  <Text className="mt-1 text-[11px] text-zinc-500">{String(j.created_at || '')}</Text>
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
