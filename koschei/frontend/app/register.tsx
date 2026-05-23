import { router } from 'expo-router';
import { useState } from 'react';
import { Text, View } from 'react-native';
import { auth } from '@/lib/auth';
import { neonAuth } from '@/lib/neonAuth';
import { Button, ErrorState, Input } from '@/components/ui';

export default function Register() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  const submit = async () => {
    setError('');
    try {
      const res: any = await neonAuth.signUpWithEmail(email.trim(), password);
      const token = neonAuth.tokenFrom(res);
      if (!token) throw new Error('auth service unavailable');
      await auth.setToken(token);
      router.replace('/dashboard');
    } catch (e: any) {
      const msg = String(e?.message || '').toLowerCase();
      if (msg.includes('already') || msg.includes('exists') || msg.includes('duplicate')) {
        setError('account already exists');
      } else if (msg.includes('email')) {
        setError('invalid email');
      } else if (msg.includes('weak') || msg.includes('password')) {
        setError('weak password');
      } else {
        setError('auth service unavailable');
      }
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
