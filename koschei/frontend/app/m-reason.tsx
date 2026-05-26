import { useState } from 'react';
import { ScrollView, StyleSheet, Text, TextInput } from 'react-native';
import { Button } from '@/components/ui';
import AsyncStorage from '@react-native-async-storage/async-storage';

export default function MReasonScreen() {
  const [prompt, setPrompt] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState('');
  const [error, setError] = useState('');

  const analyze = async () => {
    if (!prompt.trim() || loading) return;
    setLoading(true); setError(''); setResult('');
    try {
      const token = await AsyncStorage.getItem('koschei_token');
      const base = (process.env.EXPO_PUBLIC_API_URL || '').trim();
      const res = await fetch(base + '/api/ai/generate', { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + (token || '') }, body: JSON.stringify({ tool: 'reason', prompt: prompt.trim() }) });
      const data = await res.json();
      setResult(String(data?.result || data?.output || 'Analiz üretilmedi.'));
    } catch (e: any) {
      setError(String(e?.message || 'Analiz başarısız.'));
    } finally { setLoading(false); }
  };

  return <ScrollView contentContainerStyle={styles.container}>
    <Text style={styles.title}>REASON MATRIX</Text>
    <Text style={styles.sub}>Karmaşık fikirleri adım adım analiz eden düşünce motoru.</Text>
    <TextInput value={prompt} onChangeText={setPrompt} multiline textAlignVertical="top" placeholder="Analiz edilecek konuyu yaz..." placeholderTextColor="#7d8ca1" style={styles.input} />
    <Button onPress={() => { analyze().catch(() => {}); }} label={loading ? 'Analiz ediliyor...' : 'Derin Analiz Başlat'} />
    {!!error && <Text style={styles.error}>{error}</Text>}
    {!!result && <Text style={styles.output}>{result}</Text>}
  </ScrollView>;
}

const styles = StyleSheet.create({ container: { flexGrow: 1, padding: 16, backgroundColor: '#070b14', gap: 12 }, title: { color: '#f7fbff', fontSize: 24, fontWeight: '800' }, sub: { color: '#8fa6c0', fontSize: 14 }, input: { minHeight: 140, borderRadius: 12, borderWidth: 1, borderColor: '#203046', backgroundColor: '#0c1424', color: '#eef6ff', padding: 12 }, error: { color: '#ff7d7d' }, output: { color: '#d7e8ff', backgroundColor: '#0b1220', borderRadius: 12, borderWidth: 1, borderColor: '#223752', padding: 12 } });
