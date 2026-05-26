import { Text, View } from 'react-native';

export default function CyberDefensePage() {
  const enabled = String(process.env.EXPO_PUBLIC_ENABLE_CYBER_DEFENSE || 'false').toLowerCase() === 'true';
  if (!enabled) {
    return (
      <View className="flex-1 items-center justify-center bg-[#04050b] p-6">
        <Text className="text-center text-2xl font-bold text-zinc-100">CYBER DEFENSE PAUSED</Text>
        <Text className="mt-3 text-center text-zinc-300">Cyber Defense Center is paused. Koschei is focused on Runtime Factory, Artifact Forge, and Owner God Mode.</Text>
      </View>
    );
  }
  return <View className="flex-1 items-center justify-center bg-[#04050b]"><Text className="text-zinc-300">Cyber Defense is currently limited and available only when explicitly enabled.</Text></View>;
}
