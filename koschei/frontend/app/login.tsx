import { router } from 'expo-router';
import { useState } from 'react';
import { Text, View } from 'react-native';
import { auth } from '@/lib/auth';
import { neonAuth } from '@/lib/neonAuth';
import { Button, ErrorState, Input } from '@/components/ui';

export default function Login() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  const submit = async () => {
    setError('');
    try {
      const res: any = await neonAuth.signInWithEmail(email.trim(), password);
      const token = neonAuth.tokenFrom(res);
      if (!token) throw new Error('auth service unavailable');
      await auth.setToken(token);
      router.replace('/dashboard');
    } catch (e: any) {
      const msg = String(e?.message || '').toLowerCase();
      if (msg.includes('invalid') || msg.includes('credential') || msg.includes('password')) {
        setError('invalid email/password');
      } else {
        setError('auth service unavailable');
      }
    }
  };

  return (
    <View className="flex-1 bg-[#0a0a0a] p-6 gap-3" style={{ backgroundColor: '#0a0a0a' }}>
      <Text className="text-3xl text-white" style={{ color: '#ffffff' }}>
        Login
      </Text>
      <Input placeholder="Email" autoCapitalize="none" value={email} onChangeText={setEmail} />
      <Input placeholder="Password" secureTextEntry value={password} onChangeText={setPassword} />
      {!!error && <ErrorState text={error} />}
      <Button label="Login" onPress={submit} />
    </View>
  );
}
