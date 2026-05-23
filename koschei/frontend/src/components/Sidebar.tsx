import { Text, View } from 'react-native';

export function Sidebar({ credits, plan }: { credits: number; plan: string }) {
  return <View className="w-full gap-3 rounded-2xl border border-zinc-800 bg-zinc-950 p-4 md:w-72"><Text className="text-xl font-bold text-white">KOSCHEI</Text><Text className="text-zinc-300">Credits: {credits}</Text><Text className="text-[#00ff87]">Plan: {plan}</Text></View>;
}
