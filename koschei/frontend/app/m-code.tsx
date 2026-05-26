import { useState } from 'react';
import { ScrollView, StyleSheet, Text, TextInput, View } from 'react-native';
import { Button } from '@/components/ui';
import AsyncStorage from '@react-native-async-storage/async-storage';

export default function MCodeScreen() {
  const [prompt, setPrompt] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState('');

  const generate = async () => {
    if (!prompt.trim() || loading) return;
    setLoading(true);
    setError('');
    setResult('');
    try {
      const token = await AsyncStorage.getItem('koschei_token');
      const base = (process.env.EXPO_PUBLIC_API_URL || '').trim();
      const res = await fetch(base + '/api/ai/generate', { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + (token || '') }, body: JSON.stringify({ tool: 'code', prompt: prompt.trim() }) });
      const data = await res.json();
      const text = String(data?.result || data?.output || '').trim();
      if (!text) {
        setError('Kod çıktısı alınamadı.');
        return;
      }
      setResult(text);
    } catch (e: any) {
      setError(String(e?.message || 'Kod üretimi başarısız.'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <ScrollView contentContainerStyle={styles.container}>
      <Text style={styles.title}>CODE ENGINE</Text>
      <Text style={styles.sub}>Üretim hazır kod akışı için komutunu yaz.</Text>
      <TextInput
        value={prompt}
        onChangeText={setPrompt}
        multiline
        textAlignVertical="top"
        placeholder="Örn: Go + React için auth + dashboard iskeleti üret"
        placeholderTextColor="#7d8ca1"
        style={styles.input}
      />
      <Button onPress={() => { generate().catch(() => {}); }} label={loading ? 'Üretiliyor...' : 'Kod Üret'} />
      {!!error && <Text style={styles.error}>{error}</Text>}
      {!!result && <Text style={styles.output}>{result}</Text>}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: { flexGrow: 1, padding: 16, backgroundColor: '#070b14', gap: 12 },
  title: { color: '#f7fbff', fontSize: 24, fontWeight: '800' },
  sub: { color: '#8fa6c0', fontSize: 14 },
  input: { minHeight: 150, borderRadius: 12, borderWidth: 1, borderColor: '#203046', backgroundColor: '#0c1424', color: '#eef6ff', padding: 12 },
  error: { color: '#ff7d7d' },
  output: { color: '#d7e8ff', fontFamily: 'monospace', backgroundColor: '#0b1220', borderWidth: 1, borderColor: '#223752', borderRadius: 12, padding: 12 },
});
