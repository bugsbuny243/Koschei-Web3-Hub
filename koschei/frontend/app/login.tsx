import { router } from 'expo-router';
import { useState } from 'react';
import { Text, View } from 'react-native';
import { neonAuth } from '@/lib/neonAuth';
import { auth } from '@/lib/auth';
import { Button, ErrorState, Input } from '@/components/ui';

export default function Login() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');

  const submit = async () => {
    setError('');
    try {
      const token = await neonAuth.signInWithEmail(email.trim(), password);
      await auth.setToken(token);
      router.replace('/dashboard');
    } catch (e: any) {
      setError(e?.message || 'auth service unavailable');
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
