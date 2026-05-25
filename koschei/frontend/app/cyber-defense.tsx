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
const quickPrompts = ['Banka sunucu odası denetimi', 'Web app güvenlik risk analizi', 'Incident response planı', 'KVKK/GDPR kontrol listesi', 'Cloud asset review', 'Personel erişim politikası'];

export default function CyberDefensePage() {
  const [mode, setMode] = useState<(typeof modes)[number]['key']>('security_audit');
  const [prompt, setPrompt] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState<any>(null);
  const [history, setHistory] = useState<any[]>([]);
  const [warnings, setWarnings] = useState<string[]>([]);
  const [creditsCharged, setCreditsCharged] = useState<boolean | null>(null);

  const loadHistory = async () => {
    try { const data = await api.getCyberAnalyses(); setHistory(Array.isArray(data?.analyses) ? data.analyses : []); } catch { setHistory([]); }
  };
  useEffect(() => { loadHistory().catch(() => {}); }, []);

  const analyze = async () => {
    if (!prompt.trim() || loading) return;
    setLoading(true); setError(''); setWarnings([]); setCreditsCharged(null);
    try {
      const data = await api.analyzeCyber(mode, prompt.trim());
      setResult(data?.analysis || null);
      setWarnings(Array.isArray(data?.warnings) ? data.warnings : []);
      setCreditsCharged(Boolean(data?.credits_charged));
      await loadHistory();
    } catch (e: any) {
      const msg = String(e?.message || 'analysis_failed');
      setError(/timed out/i.test(msg) ? 'Cyber analysis provider timed out. Credits not charged.' : msg);
      setCreditsCharged(false);
    } finally { setLoading(false); }
  };

  const display = useMemo(() => result || history[0]?.analysis, [result, history]);
  const severityColor = (s: string) => s === 'critical' ? 'bg-red-600/30 text-red-200 border-red-500/40' : s === 'high' ? 'bg-orange-600/30 text-orange-200 border-orange-500/40' : s === 'medium' ? 'bg-yellow-600/30 text-yellow-200 border-yellow-500/40' : 'bg-cyan-700/30 text-cyan-100 border-cyan-500/40';

  return <ScrollView className="flex-1 bg-[#04050b]" contentContainerStyle={{ padding: 16, paddingBottom: 40 }}><View className="mx-auto w-full max-w-5xl gap-4">
    <View className="rounded-2xl border border-cyan-500/30 bg-[#080c18] p-4"><Text className="text-xs uppercase tracking-[2px] text-cyan-300">CYBER DEFENSE CENTER</Text></View>
    <View className="rounded-2xl border border-zinc-800 bg-[#090d1a] p-4">
      <TextInput value={prompt} onChangeText={setPrompt} multiline textAlignVertical="top" placeholder="Describe..." placeholderTextColor="#71717a" className="mt-2 min-h-[140px] rounded-xl border border-zinc-700 bg-[#030712] p-3 text-zinc-100" />
      <View className="mt-2 flex-row flex-wrap gap-2">{quickPrompts.map((chip) => <Pressable key={chip} onPress={() => setPrompt((p) => p ? `${p}\n${chip}` : chip)} className="rounded-full border border-cyan-500/30 bg-cyan-500/10 px-3 py-1"><Text className="text-xs text-cyan-100">{chip}</Text></Pressable>)}</View>
      <View className="mt-3 flex-row flex-wrap gap-2">{modes.map((m) => <Pressable key={m.key} onPress={() => setMode(m.key)} className={`rounded-lg border px-3 py-2 ${mode === m.key ? 'border-cyan-300 bg-cyan-500/20' : 'border-zinc-700 bg-[#101727]'}`}><Text className={mode === m.key ? 'text-cyan-100' : 'text-zinc-300'}>{m.label}</Text></Pressable>)}</View>
      <Pressable onPress={analyze} className="mt-3 rounded-xl bg-cyan-600 px-4 py-3"><Text className="text-center font-semibold text-white">{loading ? 'Analyzing...' : 'Analyze Security Scenario'}</Text></Pressable>
      {creditsCharged !== null && <Text className="mt-2 text-zinc-300">Credits charged: {String(creditsCharged)}</Text>}
      {!!warnings.length && <Text className="mt-2 text-amber-300">Warnings: {warnings.join(' • ')}</Text>}
      {!!error && <Text className="mt-2 text-red-300">{error}</Text>}
    </View>
    <View className="rounded-2xl border border-zinc-800 bg-[#090d1a] p-4 gap-2">
      <Text className="text-zinc-300">Executive Summary: {display?.executive_summary || '-'}</Text>
      <Text className="text-zinc-300">Blocked Actions: {(display?.blocked_actions || []).join(' • ') || '-'}</Text>
      {(display?.risks || []).map((risk: any, idx: number) => <View key={idx}><Text className={`rounded-full border px-2 py-1 text-xs ${severityColor(String(risk?.severity || 'low'))}`}>{risk?.severity || 'low'}</Text></View>)}
    </View>
  </View></ScrollView>;
}
