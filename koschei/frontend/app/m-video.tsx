import { useState } from 'react';
import { ScrollView, StyleSheet, Text, TextInput } from 'react-native';
import { Button } from '@/components/ui';
import AsyncStorage from '@react-native-async-storage/async-storage';

export default function MVideoScreen() {
  const [prompt, setPrompt] = useState('');
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState('');
  const [error, setError] = useState('');

  const createVideo = async () => {
    if (!prompt.trim() || loading) return;
    setLoading(true);
    setError('');
    setStatus('');
    try {
      const token = await AsyncStorage.getItem('koschei_token');
      const base = (process.env.EXPO_PUBLIC_API_URL || '').trim();
      const response = await fetch(base + '/api/ai/video', { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + (token || '') }, body: JSON.stringify({ prompt: prompt.trim() }) });
      const res = await response.json();
      const jobId = String(res?.job_id || res?.id || '');
      setStatus(jobId ? `Video işi alındı. Job: ${jobId}` : 'Video işi başlatıldı.');
    } catch (e: any) {
      setError(String(e?.message || 'Video işi başlatılamadı.'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <ScrollView contentContainerStyle={styles.container}>
      <Text style={styles.title}>VIDEO LAB</Text>
      <Text style={styles.sub}>Sahne açıklaması gir, video işi kuyruğa alınsın.</Text>
      <TextInput value={prompt} onChangeText={setPrompt} multiline textAlignVertical="top" placeholder="Örn: neon şehirde yağmurlu drone geçişi" placeholderTextColor="#7d8ca1" style={styles.input} />
      <Button onPress={() => { createVideo().catch(() => {}); }} label={loading ? 'Başlatılıyor...' : 'Video İşi Başlat'} />
      {!!status && <Text style={styles.ok}>{status}</Text>}
      {!!error && <Text style={styles.error}>{error}</Text>}
    </ScrollView>
  );
}

const styles = StyleSheet.create({ container: { flexGrow: 1, padding: 16, backgroundColor: '#070b14', gap: 12 }, title: { color: '#f7fbff', fontSize: 24, fontWeight: '800' }, sub: { color: '#8fa6c0', fontSize: 14 }, input: { minHeight: 150, borderRadius: 12, borderWidth: 1, borderColor: '#203046', backgroundColor: '#0c1424', color: '#eef6ff', padding: 12 }, ok: { color: '#85f6c8' }, error: { color: '#ff7d7d' } });
