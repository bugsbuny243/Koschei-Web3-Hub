import { Text, View } from 'react-native';

export default function Billing() {
  return (
    <View className="flex-1 bg-[#0a0a0a] p-6" style={{ backgroundColor: '#0a0a0a' }}>
      <Text className="text-3xl text-white" style={{ color: '#ffffff' }}>
        Billing
      </Text>
      <Text className="text-zinc-300" style={{ color: '#ffffff' }}>
        Use existing Shopier flow from pricing and manual activation API endpoint.
      </Text>
    </View>
  );
}
