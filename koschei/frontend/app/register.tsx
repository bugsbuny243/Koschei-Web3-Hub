import AsyncStorage from '@react-native-async-storage/async-storage';
import { router } from 'expo-router';
import { useState } from 'react';
import { Text, View } from 'react-native';
import { neonAuth } from '@/lib/neonAuth';
import { Button, ErrorState, Input } from '@/components/ui';

const API_BASE = (process.env.EXPO_PUBLIC_API_URL || '').trim();
const TOKEN_KEY = 'koschei_token';

async function verifySession(token: string) {
  const target = API_BASE ? `${API_BASE}/api/me` : '/api/me';
  const res = await fetch(target, {
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
  });

  const payload: any = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(payload?.error || payload?.message || `Request failed (${res.status})`);
  }
}

export default function Register() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  const submit = async () => {
    setError('');
    try {
      const response: any = await neonAuth.signUpWithEmail(email.trim(), password);
      const token = neonAuth.tokenFrom(response);
      if (!token) throw new Error('auth_token_missing: Neon Auth succeeded but no access token returned');

      await AsyncStorage.setItem(TOKEN_KEY, token);
      await verifySession(token);
      router.replace('/dashboard');
    } catch (e: any) {
      console.error('auth error', e);
      console.error('neon auth url present', Boolean(process.env.EXPO_PUBLIC_NEON_AUTH_URL));
      setError(String(e?.message || e || 'unknown_error').slice(0, 180));
    }
  };

  return (
    <View className="flex-1 bg-[#0a0a0a] p-6 gap-3" style={{ backgroundColor: '#0a0a0a' }}>
      <Text className="text-3xl text-white" style={{ color: '#ffffff' }}>
        Register
      </Text>
      <Input placeholder="Email" autoCapitalize="none" value={email} onChangeText={setEmail} />
      <Input placeholder="Password" secureTextEntry value={password} onChangeText={setPassword} />
      {!!error && <ErrorState text={error} />}
      <Button label="Create Account" onPress={submit} />
    </View>
  );
}
