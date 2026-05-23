import { Linking, Text, View } from 'react-native';
import { Card } from '@/components/ui';

const links = {
  starter: 'https://www.shopier.com/TradeVisual/47465449',
  pro: 'https://www.shopier.com/TradeVisual/47465484',
  studio: 'https://www.shopier.com/TradeVisual/47465499',
};

export default function Pricing() {
  return (
    <View className="flex-1 bg-[#0a0a0a] p-6 gap-3" style={{ backgroundColor: '#0a0a0a' }}>
      <Text className="text-3xl text-white" style={{ color: '#ffffff' }}>
        Pricing
      </Text>
      {Object.entries(links).map(([k, v]) => (
        <Card key={k}>
          <Text className="text-white" style={{ color: '#00ff87' }} onPress={() => Linking.openURL(v)}>
            {k.toUpperCase()} Upgrade
          </Text>
        </Card>
      ))}
    </View>
  );
}
