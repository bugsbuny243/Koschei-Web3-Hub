import { Text, View } from 'react-native';

export default function MImageScreen() {
  const enabled = String(process.env.EXPO_PUBLIC_ENABLE_MEDIA_MODULES || 'false').toLowerCase() === 'true';
  if (!enabled) {
    return (
      <View className="flex-1 items-center justify-center bg-[#04050b] p-6">
        <Text className="text-center text-2xl font-bold text-zinc-100">MEDIA FACTORY PAUSED</Text>
        <Text className="mt-3 text-center text-zinc-300">Media Factory is paused. Koschei is currently focused on Runtime Factory, Artifact Forge, and Owner God Mode.</Text>
      </View>
    );
  }
  return <View className="flex-1 items-center justify-center bg-[#04050b]"><Text className="text-zinc-300">This tool is available inside the AI Console on /dashboard.</Text></View>;
}
