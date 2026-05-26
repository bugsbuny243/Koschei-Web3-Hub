import { useState } from 'react';
import { ScrollView, StyleSheet, Text, TextInput, View } from 'react-native';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { Button } from '@/components/ui';

export default function MPromptLabScreen() {
  const [goal, setGoal] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState('');

  const optimize = async () => {
    if (!goal.trim() || loading) return;
    setLoading(true); setError(''); setResult('');
    try {
      const token = await AsyncStorage.getItem('koschei_token');
      const base = (process.env.EXPO_PUBLIC_API_URL || '').trim();
      const prompt = `Kullanıcı hedefi: ${goal.trim()}\nBunu daha iyi sonuç verecek bir prompta dönüştür.\nÇıktı:\n- Final Prompt\n- Neden bu daha iyi\n- 3 varyasyon`;
      const res = await fetch(base + '/api/ai/generate', { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + (token || '') }, body: JSON.stringify({ tool: 'chat', prompt }) });
      const data = await res.json();
      setResult(String(data?.result || data?.output || 'Prompt optimize edilemedi.'));
    } catch (e: any) {
      setError(String(e?.message || 'Prompt optimizasyonu başarısız.'));
    } finally { setLoading(false); }
  };

  return <ScrollView contentContainerStyle={styles.container}>
    <Text style={styles.title}>PROMPT LAB</Text>
    <Text style={styles.sub}>Hedefini yaz, daha güçlü prompt versiyonları üretelim.</Text>
    <TextInput value={goal} onChangeText={setGoal} multiline textAlignVertical="top" placeholder="Ne üretmek istiyorsun?" placeholderTextColor="#7d8ca1" style={styles.input} />
    <Button label={loading ? 'Optimize ediliyor...' : 'Prompt Optimize Et'} onPress={() => { optimize().catch(() => {}); }} />
    {!!error && <Text style={styles.error}>{error}</Text>}
    {!!result && <View style={styles.panel}><Text style={styles.panelTitle}>Optimize Prompt Çıktısı</Text><Text style={styles.output}>{result}</Text></View>}
  </ScrollView>;
}

const styles = StyleSheet.create({ container: { flexGrow: 1, padding: 16, backgroundColor: '#070b14', gap: 12 }, title: { color: '#f7fbff', fontSize: 24, fontWeight: '800' }, sub: { color: '#8fa6c0' }, input: { minHeight: 130, borderRadius: 12, borderWidth: 1, borderColor: '#203046', backgroundColor: '#0c1424', color: '#eef6ff', padding: 12 }, panel: { borderRadius: 12, borderWidth: 1, borderColor: '#223752', backgroundColor: '#0b1220', padding: 12 }, panelTitle: { color: '#9cd6ff', fontWeight: '700', marginBottom: 8 }, output: { color: '#d7e8ff' }, error: { color: '#ff7d7d' } });
