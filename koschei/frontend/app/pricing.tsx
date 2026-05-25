import { Linking, Text, View } from 'react-native';
import { Card } from '@/components/ui';
import { shopierProducts } from '@/data/shopierProducts';

const formatTry = (value: number) => `${value.toLocaleString('tr-TR')} TL`;

export default function Pricing() {
  return (
    <View className="flex-1 bg-[#0a0a0a] p-6 gap-3" style={{ backgroundColor: '#0a0a0a' }}>
      <Text className="text-3xl text-white" style={{ color: '#ffffff' }}>
        Koschei Plans
      </Text>
      <Text className="text-zinc-300" style={{ color: '#d4d4d8' }}>
        Payments are handled through Shopier. Activation is manual for now.
      </Text>
      {shopierProducts
        .filter((product) => product.isActive)
        .map((product) => (
          <Card key={product.id}>
            <View className="gap-2">
              <Text className="text-white text-xl" style={{ color: '#ffffff' }}>
                {product.name}
              </Text>
              <Text className="text-white" style={{ color: '#00ff87' }}>
                {formatTry(product.priceTry)}
              </Text>
              <Text className="text-zinc-300" style={{ color: '#d4d4d8' }}>
                {product.credits.toLocaleString('tr-TR')} credits
              </Text>
              <Text className="text-zinc-300" style={{ color: '#d4d4d8' }}>
                {product.description}
              </Text>
              {product.badge ? (
                <Text className="text-xs uppercase" style={{ color: '#60a5fa' }}>
                  {product.badge}
                </Text>
              ) : null}
              <Text
                className="text-white"
                style={{ color: '#00ff87' }}
                onPress={() => Linking.openURL(product.shopierUrl)}
              >
                Buy / Upgrade
              </Text>
            </View>
          </Card>
        ))}
    </View>
  );
}
