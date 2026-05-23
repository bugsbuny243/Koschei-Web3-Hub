import { router } from 'expo-router';
import { useState } from 'react';
import { Text, View } from 'react-native';
import { api } from '@/lib/api';
import { auth } from '@/lib/auth';
import { Button, ErrorState, Input } from '@/components/ui';

export default function Login() {
  const [email, setEmail] = useState(''); const [password, setPassword] = useState(''); const [error, setError] = useState('');
  const submit = async () => { setError(''); try { const res:any = await api.login({ email, password }); if (!res.token) throw new Error('Missing token in response.'); await auth.setToken(res.token); router.replace('/dashboard'); } catch (e:any) { setError(e.message); } };
  return <View className="flex-1 bg-[#0a0a0a] p-6 gap-3"><Text className="text-3xl text-white">Login</Text><Input placeholder="Email" autoCapitalize="none" value={email} onChangeText={setEmail}/><Input placeholder="Password" secureTextEntry value={password} onChangeText={setPassword}/>{!!error && <ErrorState text={error}/>}<Button label="Login" onPress={submit}/></View>;
}
