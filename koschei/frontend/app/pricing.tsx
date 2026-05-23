import { Linking, Text, View } from 'react-native';
import { Card } from '@/components/ui';
const links={starter:'https://www.shopier.com/TradeVisual/47465449',pro:'https://www.shopier.com/TradeVisual/47465484',studio:'https://www.shopier.com/TradeVisual/47465499'};
export default function Pricing(){return <View className="flex-1 bg-[#0a0a0a] p-6 gap-3"><Text className="text-3xl text-white">Pricing</Text>{Object.entries(links).map(([k,v])=><Card key={k}><Text className="text-white" onPress={()=>Linking.openURL(v)}>{k.toUpperCase()} Upgrade</Text></Card>)}</View>}
