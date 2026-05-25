import React, { useState } from 'react';
import {
  View,
  Text,
  TextInput,
  Pressable,
  Image,
  ScrollView,
  ActivityIndicator,
  StyleSheet,
} from 'react-native';
import AsyncStorage from '@react-native-async-storage/async-storage';

export default function MImageScreen() {
  const isEnabled = String(process.env.EXPO_PUBLIC_ENABLE_IMAGE_FORGE || 'false').toLowerCase() === 'true' && String(process.env.ENABLE_MEDIA_MODULES || 'false').toLowerCase() === 'true';
  const [prompt, setPrompt] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState('');

  const generate = async () => {
    if (!isEnabled) {
      setError('Module paused');
      return;
    }
    if (!prompt.trim()) {
      setError('Komut gir');
      return;
    }

    setLoading(true);
    setError('');
    setResult('');

    try {
      const token = await AsyncStorage.getItem('koschei_token');
      const base = (process.env.EXPO_PUBLIC_API_URL || '').trim();

      const response = await fetch(base + '/api/ai/image', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: 'Bearer ' + (token || ''),
        },
        body: JSON.stringify({ prompt, width: 1024, height: 1024 }),
      });

      const data = await response.json();

      if (data?.error) {
        setError(String(data.error));
        return;
      }

      if (data?.url) {
        setResult(String(data.url));
        return;
      }

      if (data?.b64_json) {
        setResult('data:image/png;base64,' + String(data.b64_json));
        return;
      }

      setError('Geçersiz yanıt');
    } catch (err: any) {
      setError(err?.message ? String(err.message) : 'Bir hata oluştu');
    } finally {
      setLoading(false);
    }
  };

  return (
    <ScrollView contentContainerStyle={styles.container}>
      {!isEnabled ? <Text style={styles.paused}>This module is currently paused. Koschei is focusing on Cyber Defense and Runtime Factory.</Text> : null}
      <Text style={styles.title}>GÖRSEL ÜRET</Text>

      <TextInput
        value={prompt}
        onChangeText={setPrompt}
        placeholder="Ne çizmek istersin?"
        placeholderTextColor="#8b8b8b"
        multiline
        textAlignVertical="top"
        style={styles.input}
      />

      <Pressable onPress={generate} style={[styles.button, !isEnabled ? styles.disabled : null]} disabled={!isEnabled}>
        <Text style={styles.buttonText}>ÜRET</Text>
      </Pressable>

      {loading ? (
        <View style={styles.loadingWrap}>
          <ActivityIndicator size="small" color="#ffffff" />
          <Text style={styles.loadingText}>Üretiliyor...</Text>
        </View>
      ) : null}

      {error ? <Text style={styles.error}>{error}</Text> : null}

      {result ? <Image source={{ uri: result }} style={styles.image} /> : null}
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    flexGrow: 1,
    backgroundColor: '#121212',
    padding: 16,
    gap: 12,
  },
  title: {
    color: '#f5f5f5',
    fontSize: 24,
    fontWeight: '700',
  },
  input: {
    minHeight: 140,
    borderWidth: 1,
    borderColor: '#2b2b2b',
    borderRadius: 12,
    padding: 12,
    color: '#f5f5f5',
    backgroundColor: '#1b1b1b',
  },
  button: {
    backgroundColor: '#2a2a2a',
    borderRadius: 12,
    paddingVertical: 12,
    alignItems: 'center',
  },
  buttonText: {
    color: '#f5f5f5',
    fontWeight: '700',
    fontSize: 16,
  },
  loadingWrap: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  loadingText: {
    color: '#f5f5f5',
  },
  error: {
    color: '#ff6b6b',
    fontSize: 14,
  },
  image: {
    width: '100%',
    aspectRatio: 1,
    borderRadius: 12,
  },
  paused: { color: '#facc15', fontSize: 14 },
  disabled: { opacity: 0.5 },
});
