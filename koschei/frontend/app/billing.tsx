import { Linking, Text, View } from 'react-native';
import { Card } from '@/components/ui';
import { shopierProducts } from '@/data/shopierProducts';

export default function Billing() {
  const activeProducts = shopierProducts.filter((product) => product.isActive);

  return (
    <View className="flex-1 bg-[#0a0a0a] p-6 gap-3" style={{ backgroundColor: '#0a0a0a' }}>
      <Text className="text-3xl text-white" style={{ color: '#ffffff' }}>
        Billing & Activation
      </Text>
      <Text className="text-zinc-300" style={{ color: '#d4d4d8' }}>
        Koschei currently uses Shopier checkout links.
      </Text>
      <Text className="text-zinc-300" style={{ color: '#d4d4d8' }}>
        After payment, plan/credits are activated manually by owner/admin.
      </Text>
      <Text className="text-zinc-300" style={{ color: '#d4d4d8' }}>
        Automatic Shopier webhook/payment reconciliation is planned for a later billing phase.
      </Text>
      <Text className="text-zinc-300" style={{ color: '#d4d4d8' }}>
        If payment is completed but credits are not visible yet, contact owner/support.
      </Text>

      {activeProducts.map((product) => (
        <Card key={product.id}>
          <View className="gap-2">
            <Text className="text-white text-lg" style={{ color: '#ffffff' }}>
              {product.name}
            </Text>
            <Text className="text-zinc-300" style={{ color: '#d4d4d8' }}>
              {product.priceTry.toLocaleString('tr-TR')} TL • {product.credits.toLocaleString('tr-TR')} credits
            </Text>
            <Text className="text-white" style={{ color: '#00ff87' }} onPress={() => Linking.openURL(product.shopierUrl)}>
              Open in Shopier
            </Text>
          </View>
        </Card>
      ))}
    </View>
  );
}
