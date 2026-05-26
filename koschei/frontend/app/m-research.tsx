import { useState } from 'react';
import { ScrollView, StyleSheet, Text, TextInput, View } from 'react-native';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { Button } from '@/components/ui';

export default function MResearchScreen() {
  const [topic, setTopic] = useState('');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState('');
  const [error, setError] = useState('');

  const run = async () => {
    if (!topic.trim() || loading) return;
    setLoading(true); setError(''); setResult('');
    try {
      const token = await AsyncStorage.getItem('koschei_token');
      const base = (process.env.EXPO_PUBLIC_API_URL || '').trim();
      const prompt = `Aşağıdaki konu için kısa araştırma özeti üret:\nKonu: ${topic.trim()}\nÇıktı formatı:\n1) Özet\n2) Kritik noktalar\n3) Riskler\n4) Sonraki adımlar`;
      const res = await fetch(base + '/api/ai/generate', { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + (token || '') }, body: JSON.stringify({ tool: 'reason', prompt }) });
      const data = await res.json();
      setResult(String(data?.result || data?.output || 'Araştırma çıktısı alınamadı.'));
    } catch (e: any) {
      setError(String(e?.message || 'Araştırma isteği başarısız.'));
    } finally { setLoading(false); }
  };

  return <ScrollView contentContainerStyle={styles.container}>
    <Text style={styles.title}>RESEARCH SCOUT</Text>
    <Text style={styles.sub}>Fikirini gir, hızlı araştırma özeti ve aksiyon listesi al.</Text>
    <TextInput value={topic} onChangeText={setTopic} multiline textAlignVertical="top" placeholder="Araştırılacak konu..." placeholderTextColor="#7d8ca1" style={styles.input} />
    <Button label={loading ? 'Araştırılıyor...' : 'Araştırma Başlat'} onPress={() => { run().catch(() => {}); }} />
    {!!error && <Text style={styles.error}>{error}</Text>}
    {!!result && <View style={styles.panel}><Text style={styles.panelTitle}>Araştırma Çıktısı</Text><Text style={styles.output}>{result}</Text></View>}
  </ScrollView>;
}

const styles = StyleSheet.create({ container: { flexGrow: 1, padding: 16, backgroundColor: '#070b14', gap: 12 }, title: { color: '#f7fbff', fontSize: 24, fontWeight: '800' }, sub: { color: '#8fa6c0' }, input: { minHeight: 130, borderRadius: 12, borderWidth: 1, borderColor: '#203046', backgroundColor: '#0c1424', color: '#eef6ff', padding: 12 }, panel: { borderRadius: 12, borderWidth: 1, borderColor: '#223752', backgroundColor: '#0b1220', padding: 12 }, panelTitle: { color: '#9cd6ff', fontWeight: '700', marginBottom: 8 }, output: { color: '#d7e8ff' }, error: { color: '#ff7d7d' } });
