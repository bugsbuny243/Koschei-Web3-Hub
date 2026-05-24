import { Link } from 'expo-router';
import { Text, View } from 'react-native';
import { Card } from '@/components/ui';

export default function Home() {
  return (
    <View className="flex-1 bg-[#030307] px-5 pb-10 pt-14" style={{ backgroundColor: '#030307' }}>
      <View className="absolute -left-20 top-10 h-60 w-60 rounded-full bg-cyan-500/10" />
      <View className="absolute -right-16 top-40 h-72 w-72 rounded-full bg-violet-500/10" />
      <Text className="mb-3 text-4xl font-bold leading-tight text-white" style={{ color: '#ffffff' }}>
        KOSCHEI — The Immortal AI
      </Text>
      <Text className="mb-6 text-base leading-6 text-zinc-300">Skynet Matrix Quantum Runtime for Code, Image, Video, Audio and Chat.</Text>
      <View className="mb-6 flex-row flex-wrap gap-3">
        <Link href="/register" asChild>
          <Text className="rounded-2xl border border-emerald-300/40 bg-[#00ff87] px-5 py-3 font-bold text-black shadow-[0_0_22px_rgba(0,255,135,0.45)]" style={{ backgroundColor: '#00ff87' }}>
            Sign Up Free
          </Text>
        </Link>
        <Link href="/login" asChild>
          <Text className="rounded-2xl border border-cyan-500/40 bg-[#050914]/90 px-5 py-3 text-white" style={{ color: '#ffffff' }}>
            Login
          </Text>
        </Link>
      </View>
      <View className="mb-6 gap-3">
        {['Code', 'Image', 'Video', 'Audio', 'Chat'].map((f) => (
          <Card key={f}>
            <Text className="text-lg font-semibold text-emerald-300" style={{ color: '#6ee7b7' }}>
              {f}
            </Text>
          </Card>
        ))}
      </View>
      <Card>
        <Text className="mb-2 text-xs uppercase tracking-[2px] text-cyan-300">Skynet Matrix Quantum Runtime</Text>
        {['Go API Gateway', 'Python AI Orchestration', 'Together AI Model Router', 'Runtime Task Engine', 'Neon Auth + Database'].map((item) => (
          <View key={item} className="mb-2 rounded-xl border border-emerald-500/20 bg-[#030915] px-3 py-2">
            <Text className="text-sm text-zinc-100">{item}</Text>
          </View>
        ))}
      </Card>
    </View>
  );
}
