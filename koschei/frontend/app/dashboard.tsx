import { useEffect, useMemo, useState } from 'react';
import { Platform, Pressable, ScrollView, Text, View } from 'react-native';
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

const statusBgClass = (status: string) => {
  const normalized = String(status || '').toLowerCase();
  if (normalized === 'completed') return 'border-green-500/30 bg-green-500/10';
  if (normalized === 'queued') return 'border-yellow-500/30 bg-yellow-500/10';
  if (normalized === 'running' || normalized === 'processing') return 'border-cyan-500/30 bg-cyan-500/10';
  if (normalized === 'review_needed') return 'border-amber-500/30 bg-amber-500/10';
  if (normalized === 'failed') return 'border-red-500/30 bg-red-500/10';
  return 'border-zinc-500/30 bg-zinc-500/10';
};

const statusPriority = (status: string) => {
  const normalized = String(status || '').toLowerCase();
  if (normalized === 'running' || normalized === 'processing') return 0;
  if (normalized === 'queued') return 1;
  if (normalized === 'review_needed') return 2;
  if (normalized === 'completed') return 3;
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

const formatTime = (raw: any) => {
  if (!raw) return '-';
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return String(raw);
  return d.toLocaleString();
};

const SectionCard = ({ title, subtitle, right, children }: any) => (
  <Card>
    <View className="flex-row items-start justify-between gap-3">
      <View className="flex-1">
        <Text className="text-xs uppercase tracking-[2px] text-cyan-300">{title}</Text>
        {!!subtitle && <Text className="mt-1 text-sm text-zinc-400">{subtitle}</Text>}
      </View>
      {right}
    </View>
    <View className="mt-3">{children}</View>
  </Card>
);

const StatusPill = ({ status }: { status: string }) => (
  <View className={`rounded-full border px-2 py-1 ${statusBgClass(status)}`}>
    <Text className={`text-xs uppercase ${statusTextClass(status)}`}>{String(status || 'unknown')}</Text>
  </View>
);

const taskDescription: Record<string, string> = {
  intake: 'Input analysis',
  blueprint: 'MVP/product plan',
  architecture: 'Infra/API/database plan',
  file_plan: 'File/package map',
  build_steps: 'Proposed tool plan',
  review: 'Guardrail and human review',
  delivery: 'Delivery package plan',
};

export default function Dashboard() {
  const mediaEnabled = String(process.env.EXPO_PUBLIC_ENABLE_MEDIA_MODULES || 'false').toLowerCase() === 'true';
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
  const [artifactLoading, setArtifactLoading] = useState(false);
  const [artifacts, setArtifacts] = useState<any[]>([]);
  const [artifactDetail, setArtifactDetail] = useState<any>(null);
  const [expandedFileId, setExpandedFileId] = useState('');
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

  const sortedProjects = useMemo(() => [...projects].sort((a, b) => parseTimestamp(b) - parseTimestamp(a) || statusPriority(a.status) - statusPriority(b.status)), [projects]);
  const newestProject = useMemo(() => (sortedProjects.length > 0 ? sortedProjects[0] : null), [sortedProjects]);
  const displayedTasks = useMemo(() => {
    const sortedAll = [...tasks].sort((a, b) => parseTimestamp(b) - parseTimestamp(a) || statusPriority(a.status) - statusPriority(b.status));
    if (newestProject?.id) {
      const filtered = sortedAll.filter((t) => t?.project_id === newestProject.id);
      if (filtered.length > 0) return filtered;
    }
    return sortedAll;
  }, [tasks, newestProject]);
  const displayedLogs = useMemo(() => [...logs].sort((a, b) => parseTimestamp(b) - parseTimestamp(a)).slice(0, 5), [logs]);
  const displayedJobs = useMemo(() => [...aiJobs].sort((a, b) => parseTimestamp(b) - parseTimestamp(a)).slice(0, 5), [aiJobs]);

  const refreshRuntimeData = async () => {
    const projectRows: any[] = await api.getRuntimeProjects();
    const taskRows: any[] = await api.getRuntimeTasks();
    setProjects(projectRows);
    setTasks(taskRows);
    const sortedByTime = [...projectRows].sort((a, b) => parseTimestamp(b) - parseTimestamp(a));
    const newest = sortedByTime[0];
    if (newest?.id) {
      try { setArtifacts(await api.getProjectArtifacts(newest.id)); } catch { setArtifacts([]); }
    }
    if (newest && String(newest?.status || '').toLowerCase() === 'failed') {
      const failedLogs: any[] = await api.getRuntimeLogs(newest.id);
      const latestFailureLog = [...failedLogs].sort((a, b) => parseTimestamp(b) - parseTimestamp(a)).find((l) => String(l?.level || '').toLowerCase() === 'error');
      setLatestOutput({ failed_message: latestFailureLog?.message || 'Runtime project failed.' });
    } else if (newest?.id && ['completed', 'review_needed'].includes(String(newest?.status || '').toLowerCase())) {
      const projectTasks = taskRows.filter((t) => t?.project_id === newest.id);
      const blueprintTask = projectTasks.find((t) => String(t?.task_type || '').toLowerCase() === 'blueprint');
      if (blueprintTask?.output_json) {
        const output = safeJson(blueprintTask.output_json);
        const arch = safeJson(projectTasks.find((t) => t?.task_type === 'architecture')?.output_json);
        const steps = safeJson(projectTasks.find((t) => t?.task_type === 'build_steps')?.output_json);
        const review = safeJson(projectTasks.find((t) => t?.task_type === 'review')?.output_json);
        const delivery = safeJson(projectTasks.find((t) => t?.task_type === 'delivery')?.output_json);
        setLatestOutput({
          ...output,
          contract_version: output?.contract_version || '-',
          intake_summary: output?.output?.summary || output?.intake?.summary || '-',
          user_intent: output?.user_intent || '-',
          mvp_scope: output?.output?.mvp_scope || output?.mvp_scope || [],
          file_plan: safeJson(projectTasks.find((t) => t?.task_type === 'file_plan')?.output_json)?.output?.files || [],
          required_infrastructure: arch?.output?.required_infrastructure || arch?.required_infrastructure || [],
          proposed_tool_calls: steps?.output?.proposed_tool_calls || [],
          review_status: review?.output?.review_status || '-',
          guardrail_flags: review?.output?.guardrail_flags || [],
          validation_status: review?.validation?.valid === false || delivery?.validation?.valid === false ? 'invalid' : 'valid',
          validation_review_needed: Boolean(review?.validation?.review_needed ?? delivery?.validation?.review_needed),
          validation_blocked: Boolean(review?.validation?.blocked ?? delivery?.validation?.blocked),
          validation_warnings: review?.validation?.warnings || delivery?.validation?.warnings || [],
          validation_errors: review?.validation?.errors || delivery?.validation?.errors || [],
          delivery_package: delivery?.output?.delivery_package || [],
          next_steps: delivery?.output?.next_steps || [],
          project_status: String(newest?.status || '').toLowerCase(),
        });
      } else setLatestOutput(null);
    } else setLatestOutput(null);
    if (newest?.id) setLogs(await api.getRuntimeLogs(newest.id)); else setLogs([]);
  };

  const refreshMe = async () => {
    const me: any = await api.me();
    if (me?.user) {
      setCredits(readProfileField<number>(me.user, 'credits', 'Credits', 0) || 0);
      setPlan(readProfileField<string>(me.user, 'plan', 'Plan', 'free') || 'free');
      setEmail(readProfileField<string>(me.user, 'email', 'Email', '') || '');
    }
  };

  const refreshAiJobs = async () => {
    try { const rows: any = await api.get('/api/ai/jobs'); setAiJobs(Array.isArray(rows?.jobs) ? rows.jobs : []); } catch { setAiJobs([]); }
  };

  useEffect(() => { (async () => {
    const token = await auth.getToken();
    if (!token) return router.replace('/login');
    try { await refreshMe(); await refreshRuntimeData(); await refreshAiJobs(); } catch (e: any) {
      if (String(e?.message || '').includes('401')) { await auth.clearToken(); router.replace('/login'); }
    }
  })(); }, []);

  useEffect(() => {
    const active = projects.some((p) => ['running', 'processing'].includes(String(p?.status || '').toLowerCase()));
    if (!active) return;
    const timer = setInterval(() => refreshRuntimeData().catch(() => {}), 3000);
    return () => clearInterval(timer);
  }, [projects]);

  const send = async () => {
    if (runtimeLoading) return;
    try {
      setRuntimeLoading(true); setError('');
      await api.createRuntimeProject({ title: `Project ${new Date().toISOString()}`, prompt });
      await refreshRuntimeData(); setPrompt('');
    } catch (e: any) {
      const detailRaw = typeof e?.data?.detail === 'string' ? e.data.detail.trim() : '';
      const timeoutLike = /timeout|context deadline exceeded|client\.timeout exceeded|awaiting headers/i.test(detailRaw);
      const friendly = timeoutLike ? 'Runtime AI provider timed out. Credits were not charged. Try again with a shorter prompt or wait for fallback/async artifact worker improvements.' : '';
      const backendDetail = detailRaw ? `: ${detailRaw}` : '';
      const composed = `${e?.message || 'runtime_request_failed'}${backendDetail}`;
      setError(friendly ? `${friendly} | ${composed}` : composed);
      await refreshMe();
    } finally { setRuntimeLoading(false); }
  };

  const generateArtifact = async () => {
    if (!newestProject?.id || artifactLoading) return;
    try { setArtifactLoading(true); await api.generateArtifact(newestProject.id, false); await refreshRuntimeData(); }
    catch (e: any) { setError(e?.message || 'artifact_generation_failed'); }
    finally { setArtifactLoading(false); }
  };

  const loadArtifactDetail = async (artifactId: string) => setArtifactDetail(await api.getArtifact(artifactId));
  const loadArtifactFile = async (artifactId: string, fileId: string) => {
    const data = await api.getArtifactFile(artifactId, fileId);
    setExpandedFileId(fileId);
    setArtifactDetail((prev: any) => ({ ...prev, files: (prev?.files || []).map((f: any) => f.id === fileId ? { ...f, content: data?.content } : f) }));
  };
  const downloadArtifactZip = async (artifactId: string) => {
    if (Platform.OS !== 'web') return setError('Protected download links for mobile will be added next.');
    const blob = await api.downloadArtifactZip(artifactId);
    const objectUrl = URL.createObjectURL(blob);
    const anchor = document.createElement('a'); anchor.href = objectUrl; anchor.download = 'koschei-artifact.zip'; document.body.appendChild(anchor); anchor.click(); document.body.removeChild(anchor); URL.revokeObjectURL(objectUrl);
  };

  const runAiTest = async () => {
    if (aiLoading) return;
    setAiError(''); setAiResult(''); setAiLoading(true);
    try {
      const token = await AsyncStorage.getItem('koschei_token');
      const response = await fetch(`${(process.env.EXPO_PUBLIC_API_URL || '').trim()}/api/ai/generate`, { method: 'POST', headers: { Authorization: `Bearer ${token || ''}`, 'Content-Type': 'application/json' }, body: JSON.stringify({ tool: aiTool, prompt: aiPrompt }) });
      const data = await response.json().catch(() => ({}));
      if (!response.ok) return setAiError(`${data?.error || data?.message || 'ai_request_failed'} (status ${response.status})`);
      setAiResult(typeof data?.result === 'string' ? data.result : JSON.stringify(data));
      await refreshAiJobs();
    } catch (e: any) { setAiError(e?.message || 'ai_request_failed'); }
    finally { setAiLoading(false); await refreshMe().catch(() => {}); }
  };

  const pipeline = ['intake', 'blueprint', 'architecture', 'file_plan', 'build_steps', 'review', 'delivery'];
  const isFailed = Boolean(latestOutput?.failed_message);
  const technicalDetail = isFailed ? String(latestOutput?.failed_message || '') : (error.includes('|') ? error.split('|')[1]?.trim() : '');

  return (
    <ScrollView className="flex-1 bg-[#020207]" contentContainerStyle={{ padding: 14, paddingBottom: 32 }}>
      <View className="absolute -left-16 top-24 h-72 w-72 rounded-full bg-cyan-500/10" />
      <View className="absolute -right-16 bottom-24 h-72 w-72 rounded-full bg-violet-500/10" />
      <View className="mx-auto w-full max-w-6xl gap-3">
        <SectionCard title="KOSCHEI COMMAND CENTER" subtitle="Agentic Runtime Factory • Artifact Builder • AI Production Cockpit" right={<StatusPill status="ONLINE" />}>
          <Text className="text-xs text-zinc-400">{email} • {plan} • {credits} credits</Text>
          <View className="mt-3 gap-2 md:flex-row">
            <Button label="UI Lab" onPress={() => router.push('/ui-lab')} />
            <Button label="Pricing" onPress={() => router.push('/pricing')} />
            <Button label="Logout" onPress={async () => { await auth.clearToken(); router.replace('/login'); }} />
          </View>
        </SectionCard>

        <View className="gap-3 md:flex-row">
          <View className="md:w-[320px]">
            <SectionCard title="Operator Core" subtitle="Identity / credits / access level">
              <Text className="text-zinc-300">{email || 'Signed in'}</Text>
              <View className="mt-2 flex-row gap-2"><StatusPill status={plan} /><StatusPill status={`${credits} CR`} /></View>
              {credits <= 2 && <Text className="mt-2 text-amber-300">Low credits</Text>}
            </SectionCard>
          </View>
          <View className="flex-1 gap-3">
            <SectionCard title="Runtime Factory" subtitle="Turn an idea into an agentic production contract and delivery plan.">
              <Input placeholder="Describe the app, game, automation, SaaS, or AI system you want Koschei to build..." value={prompt} onChangeText={setPrompt} />
              <View className="mt-3"><Button label={runtimeLoading ? 'Queueing Runtime Project...' : 'Create Runtime Project'} onPress={send} disabled={runtimeLoading} /></View>
              {!!error && <View className="mt-2"><ErrorState text={error.split('|')[0]} /></View>}
              {!!technicalDetail && <Text className="mt-2 text-xs text-zinc-500" numberOfLines={3}>Technical detail: {technicalDetail}</Text>}
              <View className="mt-3 flex-row flex-wrap gap-2">
                {pipeline.map((step) => {
                  const task = displayedTasks.find((t) => String(t?.task_type || '').toLowerCase() === step);
                  return <View key={step} className={`rounded-full border px-2 py-1 ${statusBgClass(task?.status || 'queued')}`}><Text className={`text-[10px] uppercase ${statusTextClass(task?.status || 'queued')}`}>{step.replace('_', ' ')}</Text></View>;
                })}
              </View>
            </SectionCard>
            <SectionCard title="AI Console" subtitle="Quick test console for chat/code/reason routes.">
              <View className="mb-2 flex-row gap-2">{(['chat', 'code', 'reason'] as const).map((tool) => <Pressable key={tool} onPress={() => setAiTool(tool)} className={`rounded-xl border px-3 py-2 ${aiTool === tool ? 'border-cyan-300 bg-cyan-500/20' : 'border-cyan-500/20 bg-[#040812]'}`}><Text className={aiTool === tool ? 'text-cyan-100' : 'text-zinc-300'}>{tool}</Text></Pressable>)}</View>
              <Input placeholder="Enter prompt for AI tool..." value={aiPrompt} onChangeText={setAiPrompt} />
              <View className="mt-2"><Button label={aiLoading ? 'Running AI...' : 'Run AI'} onPress={runAiTest} /></View>
              {!!aiError && <View className="mt-2"><ErrorState text={aiError} /></View>}
              <Text className="mt-2 text-xs text-zinc-300" numberOfLines={6}>{aiLoading ? 'Loading...' : (aiResult || 'No result yet')}</Text>
            </SectionCard>
          </View>
        </View>
        <SectionCard title="Strategic Modules" subtitle="Current active direction for Koschei.">
          <View className="gap-2">
            <View className="rounded-lg border border-cyan-500/30 bg-cyan-500/10 p-3"><Text className="text-zinc-100">Runtime Factory — ACTIVE</Text></View>
            <View className="rounded-lg border border-violet-500/30 bg-violet-500/10 p-3"><Text className="text-zinc-100">Artifact Forge — ACTIVE</Text></View>
            <View className="rounded-lg border border-cyan-500/30 bg-cyan-500/10 p-3"><Text className="text-zinc-100">AI Console — ACTIVE</Text></View>
            <View className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3"><Text className="text-zinc-100">Owner God Mode — NEXT</Text><View className="mt-2"><Button label="Prepare Owner Mode" onPress={() => router.push('/owner')} /></View></View>
            <View className="rounded-lg border border-cyan-500/30 bg-cyan-500/10 p-3"><Text className="text-zinc-100">Public SaaS — ACTIVE</Text></View>
            <View className="rounded-lg border border-zinc-700 bg-[#040a15] p-3"><Text className="text-zinc-300">Media Factory — PAUSED</Text></View>
            <View className="rounded-lg border border-zinc-700 bg-[#040a15] p-3"><Text className="text-zinc-300">Cyber Defense — PAUSED / Enterprise Future</Text></View>
          </View>
        </SectionCard>

        <SectionCard title="Latest Project Status" subtitle={newestProject?.title || 'No runtime project yet'} right={<StatusPill status={String(newestProject?.status || 'unknown')} />}>
          <Text className="text-xs text-zinc-400">Created: {formatTime(newestProject?.created_at)} • Updated: {formatTime(newestProject?.updated_at)}</Text>
          {isFailed && <Text className="mt-2 text-red-300">Runtime provider timed out or failed. Credits were not charged.</Text>}
        </SectionCard>

        <SectionCard title="Runtime Contract" subtitle="Premium production contract view">
          {!latestOutput || isFailed ? <Text className="text-zinc-500">No production contract yet.</Text> : <View className="gap-2">
            {latestOutput?.project_status === 'review_needed' && <Text className="rounded-lg border border-amber-500/30 bg-amber-500/10 p-2 text-amber-300">Human review required before artifact generation.</Text>}
            <Text className="text-zinc-100">Contract: v{latestOutput.contract_version} • {latestOutput.project_title} • {latestOutput.project_type}</Text>
            <Text className="text-zinc-300">Intent: {latestOutput.user_intent}</Text>
            <Text className="text-zinc-300">Intake: {latestOutput.intake_summary}</Text>
            <Text className="text-zinc-300">MVP Scope: {Array.isArray(latestOutput.mvp_scope) ? latestOutput.mvp_scope.join(' • ') : '-'}</Text>
            <Text className="text-zinc-300">Infrastructure: {Array.isArray(latestOutput.required_infrastructure) ? latestOutput.required_infrastructure.join(' • ') : '-'}</Text>
            <Text className="text-zinc-300">Proposed Tool Calls — Not executed yet</Text>
            {(latestOutput.proposed_tool_calls || []).map((c: any, i: number) => <Text key={i} className="text-xs text-zinc-400">{c?.tool_name || '-'} • {c?.risk_level || '-'} • approval:{String(c?.requires_human_approval)}</Text>)}
            <View className="mt-2"><Button label={artifactLoading ? 'Generating artifact package...' : 'Generate Artifact Package'} onPress={generateArtifact} disabled={!newestProject || String(newestProject?.status || '').toLowerCase() !== 'completed' || artifactLoading || latestOutput?.project_status === 'review_needed'} /></View>
          </View>}
        </SectionCard>

        <SectionCard title={`Artifact Forge (${artifacts.length})`} subtitle="Generate downloadable delivery packages from completed runtime contracts.">
          {artifacts.length === 0 && <Text className="text-zinc-500">No artifacts yet</Text>}
          <View className="gap-2">{artifacts.map((a) => <View key={a.id} className="rounded-lg border border-cyan-500/20 bg-[#040a15] p-2"><Text className="text-zinc-100">{a.title || a.id}</Text><Text className="text-xs text-zinc-400">{a.status} • files:{a.file_count}</Text><Text className="text-zinc-300" numberOfLines={2}>{a.summary || '-'}</Text><View className="mt-2 gap-2 md:flex-row"><Button label="View Files" onPress={() => loadArtifactDetail(a.id)} />{a.status === 'completed' && <Button label="Download ZIP" onPress={() => downloadArtifactZip(a.id)} />}</View></View>)}</View>
          {!!artifactDetail?.files && artifactDetail.files.map((f: any) => <View key={f.id} className="mt-2 rounded border border-violet-500/20 p-2"><Text className="text-xs text-zinc-200">{f.path} [{f.language || '-'} | {f.action || '-'}]</Text><Button label="Preview" onPress={() => loadArtifactFile(artifactDetail.id, f.id)} />{expandedFileId === f.id && !!f.content && <Text selectable className="mt-1 text-xs text-zinc-300" numberOfLines={12}>{String(f.content)}</Text>}</View>)}
        </SectionCard>

        <SectionCard title={`Project Fleet (${sortedProjects.length})`}>
          {sortedProjects.map((p) => <View key={p.id} className="mb-2 flex-row items-center justify-between rounded-lg border border-zinc-700 bg-[#040a15] px-3 py-2"><View className="flex-1"><Text className="text-zinc-100" numberOfLines={1}>{p.title}</Text><Text className="text-xs text-zinc-500">{formatTime(p.updated_at || p.created_at)}</Text></View><StatusPill status={String(p.status || 'unknown')} /></View>)}
        </SectionCard>

        <SectionCard title={`Agent Pipeline (${displayedTasks.length})`}>
          {displayedTasks.map((t) => <View key={t.id} className="mb-2 rounded-lg border border-cyan-500/20 bg-[#040a15] px-3 py-2"><View className="flex-row items-center justify-between"><Text className="text-zinc-100">{t.task_type}</Text><StatusPill status={String(t.status || 'unknown')} /></View><Text className="mt-1 text-xs text-zinc-400">{taskDescription[String(t.task_type || '').toLowerCase()] || 'Task processing'}</Text></View>)}
        </SectionCard>

        <SectionCard title={`Model Activity (${displayedJobs.length})`}>
          {displayedJobs.map((j) => <View key={j.id} className="mb-2 rounded-lg border border-cyan-500/20 bg-[#040a15] px-3 py-2"><View className="flex-row items-center justify-between"><Text className="text-zinc-100">{String(j.tool || 'unknown')}</Text><StatusPill status={String(j.status || 'unknown')} /></View><Text className="text-xs text-zinc-400" numberOfLines={2}>{String(j.prompt || '')}</Text><Text className="text-[11px] text-zinc-500">{formatTime(j.created_at)}</Text></View>)}
        </SectionCard>

        <SectionCard title={`System Telemetry (${displayedLogs.length})`}>
          {displayedLogs.map((l) => <View key={l.id} className={`mb-2 rounded-lg border px-3 py-2 ${String(l.level || '').toLowerCase() === 'error' ? 'border-red-500/30 bg-red-500/10' : 'border-violet-500/20 bg-[#040a15]'}`}><Text className="text-xs uppercase text-zinc-400">{String(l.level || 'info')}</Text><Text className="text-zinc-100" numberOfLines={3}>{String(l.message || '')}</Text></View>)}
        </SectionCard>
      </View>
    </ScrollView>
  );
}
