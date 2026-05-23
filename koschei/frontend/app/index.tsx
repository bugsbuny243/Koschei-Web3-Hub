import { Link } from 'expo-router';
import { Text, View } from 'react-native';
import { Card } from '@/components/ui';

export default function Home() {
  return <View className="flex-1 bg-[#0a0a0a] p-6"><Text className="mb-6 text-4xl font-bold text-white">KOSCHEI — The Immortal AI</Text><View className="mb-6 flex-row gap-3"><Link href="/register" asChild><Text className="rounded-xl bg-[#00ff87] px-4 py-3 font-bold text-black">Sign Up Free</Text></Link><Link href="/login" asChild><Text className="rounded-xl border border-zinc-700 px-4 py-3 text-white">Login</Text></Link></View><View className="gap-3">{['Code','Image','Video','Audio','Chat'].map(f=><Card key={f}><Text className="text-white">{f}</Text></Card>)}</View></View>;
}
