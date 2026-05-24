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
    throw new Error(`api_me_failed_${res.status}: ${payload?.error || payload?.message || 'unknown'}`);
  }
}

export default function Login() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  const submit = async () => {
    setError('');
    try {
      const response: any = await neonAuth.signInWithEmail(email.trim(), password);
      let token = neonAuth.tokenFrom(response);
      if (!token) {
        token = await neonAuth.getToken();
      }
      if (!token) throw new Error('auth_token_missing');
      if (token.split('.').length !== 3) throw new Error('auth_token_not_jwt');

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
    <View className="flex-1 items-center justify-center bg-[#020207] p-6" style={{ backgroundColor: '#020207' }}>
      <View className="absolute -left-20 top-20 h-64 w-64 rounded-full bg-cyan-500/10" />
      <View className="absolute -right-24 bottom-16 h-72 w-72 rounded-full bg-violet-500/10" />
      <View className="w-full max-w-md rounded-3xl border border-emerald-500/20 bg-[#060a13]/95 p-6 shadow-[0_0_30px_rgba(0,255,135,0.15)] gap-3">
        <Text className="text-3xl font-semibold text-white" style={{ color: '#ffffff' }}>
          Enter Koschei Runtime
        </Text>
        <Text className="mb-2 text-sm text-zinc-400">Secure Neon Auth access</Text>
        <Input placeholder="Email" autoCapitalize="none" value={email} onChangeText={setEmail} />
        <Input placeholder="Password" secureTextEntry value={password} onChangeText={setPassword} />
        {!!error && <ErrorState text={error} />}
        <Button label="Login" onPress={submit} />
      </View>
    </View>
  );
}
