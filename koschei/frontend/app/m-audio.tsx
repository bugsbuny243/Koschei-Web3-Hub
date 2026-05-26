import AsyncStorage from '@react-native-async-storage/async-storage';
import { Audio } from 'expo-av';
import { useRef, useState } from 'react';
import {
  View,
  Text,
  TextInput,
  Pressable,
  ScrollView,
  ActivityIndicator,
  StyleSheet,
} from 'react-native';

export default function MAudioScreen() {
  const isEnabled = true;
  const [input, setInput] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [hasAudio, setHasAudio] = useState(false);
  const soundRef = useRef<Audio.Sound | null>(null);

  const generate = async () => {
    if (!isEnabled) {
      setError('Module paused');
      return;
    }
    if (!input.trim()) {
      setError('Metin gir');
      return;
    }

    setLoading(true);
    setError('');

    try {
      const token = await AsyncStorage.getItem('koschei_token');
      const base = (process.env.EXPO_PUBLIC_API_URL || '').trim();

      const response = await fetch(base + '/api/ai/audio', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: 'Bearer ' + (token || ''),
        },
        body: JSON.stringify({ input, voice: 'tara' }),
      });

      const data = await response.json();

      if (data?.error) {
        setError(String(data.error));
        return;
      }

      if (data?.audio_base64) {
        const dataUri = 'data:audio/mpeg;base64,' + String(data.audio_base64);

        if (soundRef.current) {
          await soundRef.current.unloadAsync();
        }

        const { sound } = await Audio.Sound.createAsync(
          { uri: dataUri },
          { shouldPlay: true }
        );

        soundRef.current = sound;
        setHasAudio(true);
        return;
      }

      setError('Geçersiz yanıt');
    } catch (err: any) {
      setError(err?.message ? String(err.message) : 'Bir hata oluştu');
    } finally {
      setLoading(false);
    }
  };

  const replay = async () => {
    try {
      if (!soundRef.current) {
        return;
      }

      await soundRef.current.replayAsync();
    } catch (err: any) {
      setError(err?.message ? String(err.message) : 'Ses oynatılamadı');
    }
  };

  return (
    <ScrollView contentContainerStyle={styles.container}>
      {!isEnabled ? <Text style={styles.paused}>This module is currently paused. Koschei is focusing on Cyber Defense and Runtime Factory.</Text> : null}
      <Text style={styles.title}>SESLENDİR</Text>

      <TextInput
        value={input}
        onChangeText={setInput}
        placeholder="Seslendirilecek metni yaz..."
        placeholderTextColor="#8b8b8b"
        multiline
        textAlignVertical="top"
        style={styles.input}
      />

      <Pressable onPress={generate} style={[styles.button, !isEnabled ? styles.disabled : null]} disabled={!isEnabled}>
        <Text style={styles.buttonText}>SESLENDİR</Text>
      </Pressable>

      {loading ? (
        <View style={styles.loadingWrap}>
          <ActivityIndicator size="small" color="#ffffff" />
          <Text style={styles.loadingText}>Üretiliyor...</Text>
        </View>
      ) : null}

      {hasAudio ? (
        <Pressable onPress={replay} style={styles.button}>
          <Text style={styles.buttonText}>TEKRAR ÇAL</Text>
        </Pressable>
      ) : null}

      {error ? <Text style={styles.error}>{error}</Text> : null}
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
  paused: { color: '#facc15', fontSize: 14 },
  disabled: { opacity: 0.5 },
});
