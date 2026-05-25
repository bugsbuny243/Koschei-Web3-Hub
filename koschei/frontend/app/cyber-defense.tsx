import { useEffect, useMemo, useState } from 'react';
import { Pressable, ScrollView, Text, TextInput, View } from 'react-native';
import { api } from '@/lib/api';

const modes = [
  { key: 'security_audit', label: 'Security Audit' },
  { key: 'risk_assessment', label: 'Risk Assessment' },
  { key: 'incident_response', label: 'Incident Response Plan' },
  { key: 'compliance_checklist', label: 'Compliance Checklist' },
  { key: 'asset_review', label: 'Asset Review' },
  { key: 'policy_review', label: 'Policy Review' },
] as const;

export default function CyberDefensePage() {
  const [mode, setMode] = useState<(typeof modes)[number]['key']>('security_audit');
  const [prompt, setPrompt] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState<any>(null);
  const [history, setHistory] = useState<any[]>([]);

  const loadHistory = async () => {
    try {
      const data = await api.getCyberAnalyses();
      setHistory(Array.isArray(data?.analyses) ? data.analyses : []);
    } catch {
      setHistory([]);
    }
  };

  useEffect(() => {
    loadHistory().catch(() => {});
  }, []);

  const analyze = async () => {
    if (!prompt.trim() || loading) return;
    setLoading(true);
    setError('');
    try {
      const data = await api.analyzeCyber(mode, prompt.trim());
      setResult(data?.analysis || null);
      await loadHistory();
    } catch (e: any) {
      setError(e?.message || 'analysis_failed');
    } finally {
      setLoading(false);
    }
  };

  const severityColor = (severity: string) => {
    const s = String(severity || '').toLowerCase();
    if (s === 'critical') return 'bg-red-600/30 text-red-200 border-red-500/40';
    if (s === 'high') return 'bg-orange-600/30 text-orange-200 border-orange-500/40';
    if (s === 'medium') return 'bg-yellow-600/30 text-yellow-200 border-yellow-500/40';
    return 'bg-cyan-700/30 text-cyan-100 border-cyan-500/40';
  };

  const display = useMemo(() => result || history[0]?.analysis, [result, history]);

  return (
    <ScrollView className="flex-1 bg-[#04050b]" contentContainerStyle={{ padding: 16, paddingBottom: 40 }}>
      <View className="mx-auto w-full max-w-5xl gap-4">
        <View className="rounded-2xl border border-cyan-500/30 bg-[#080c18] p-4">
          <Text className="text-xs uppercase tracking-[2px] text-cyan-300">CYBER DEFENSE CENTER</Text>
          <Text className="mt-2 text-2xl font-semibold text-zinc-100">Defensive AI security command module</Text>
          <Text className="mt-2 text-zinc-300">Analyze systems, policies, assets, risks, and incident workflows with human-approved AI reasoning.</Text>
          <Text className="mt-3 rounded-lg border border-amber-500/40 bg-amber-500/10 p-2 text-xs text-amber-200">Defensive analysis only. Koschei does not perform unauthorized access, exploitation, credential theft, or autonomous shutdown actions.</Text>
        </View>

        <View className="rounded-2xl border border-zinc-800 bg-[#090d1a] p-4">
          <Text className="text-zinc-200">Security Scenario / Audit Prompt</Text>
          <TextInput value={prompt} onChangeText={setPrompt} multiline textAlignVertical="top" placeholder="Describe the system, company, server room, application, cloud setup, or security concern you want Koschei to analyze..." placeholderTextColor="#71717a" className="mt-2 min-h-[140px] rounded-xl border border-zinc-700 bg-[#030712] p-3 text-zinc-100" />
          <View className="mt-3 flex-row flex-wrap gap-2">{modes.map((m) => <Pressable key={m.key} onPress={() => setMode(m.key)} className={`rounded-lg border px-3 py-2 ${mode === m.key ? 'border-cyan-300 bg-cyan-500/20' : 'border-zinc-700 bg-[#101727]'}`}><Text className={mode === m.key ? 'text-cyan-100' : 'text-zinc-300'}>{m.label}</Text></Pressable>)}</View>
          <Pressable onPress={analyze} className="mt-3 rounded-xl bg-cyan-600 px-4 py-3"><Text className="text-center font-semibold text-white">{loading ? 'Analyzing...' : 'Analyze Security Scenario'}</Text></Pressable>
          {!!error && <Text className="mt-2 text-red-300">{error}</Text>}
        </View>

        <View className="rounded-2xl border border-zinc-800 bg-[#090d1a] p-4 gap-2">
          <Text className="text-lg text-zinc-100">Analysis Output</Text>
          <Text className="text-zinc-300">Executive Summary: {display?.executive_summary || '-'}</Text>
          <Text className="text-zinc-300">Assets / Scope: {(display?.scope || []).join(' • ') || '-'}</Text>
          <Text className="text-zinc-300">Human Approval Required: {String(display?.human_review_required ?? true)}</Text>
          <Text className="text-zinc-300">Defensive Recommendations:</Text>
          {(display?.risks || []).map((risk: any, idx: number) => (
            <View key={`${risk?.title || 'risk'}-${idx}`} className="rounded-lg border border-zinc-700 bg-[#060b16] p-3">
              <View className="flex-row items-center justify-between">
                <Text className="text-zinc-100">{risk?.title || 'Risk'}</Text>
                <Text className={`rounded-full border px-2 py-1 text-xs ${severityColor(risk?.severity)}`}>{risk?.severity || 'low'}</Text>
              </View>
              <Text className="mt-1 text-zinc-300">Risks: {risk?.description || '-'}</Text>
              <Text className="mt-1 text-zinc-300">Severity: {risk?.severity || '-'}</Text>
              <Text className="mt-1 text-zinc-300">Recommendation: {risk?.defensive_recommendation || '-'}</Text>
              <Text className="mt-1 text-zinc-400">Human Approval Required: {String(risk?.human_approval_required ?? true)}</Text>
            </View>
          ))}
          <Text className="text-zinc-300">Compliance Notes: {(display?.compliance_notes || []).join(' • ') || '-'}</Text>
          <Text className="text-zinc-300">Incident Steps: {(display?.incident_steps || []).join(' • ') || '-'}</Text>
          <Text className="text-zinc-300">Blocked Actions: {(display?.blocked_actions || []).join(' • ') || '-'}</Text>
          <Text className="text-zinc-300">Next Steps: {(display?.next_steps || []).join(' • ') || '-'}</Text>
          <Text className="text-zinc-300">Notes: Defensive module provides planning and review guidance only.</Text>
        </View>

        <View className="rounded-2xl border border-zinc-800 bg-[#090d1a] p-4 gap-2">
          <Text className="text-lg text-zinc-100">Latest Analyses</Text>
          {history.map((item) => (
            <Pressable key={item.id} onPress={() => setResult(item.analysis)} className="rounded-lg border border-zinc-700 bg-[#060b16] p-3">
              <Text className="text-zinc-100">{item.mode}</Text>
              <Text className="text-xs text-zinc-400" numberOfLines={2}>{item.prompt}</Text>
              <Text className="text-xs text-zinc-400">human_review_required: {String(item?.analysis?.human_review_required ?? true)}</Text>
            </Pressable>
          ))}
        </View>
      </View>
    </ScrollView>
  );
}
