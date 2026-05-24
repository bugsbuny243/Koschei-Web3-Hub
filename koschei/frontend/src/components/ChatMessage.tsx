import { Image, Text, View } from 'react-native';
import { Video, ResizeMode } from 'expo-av';
import { ChatMessage } from '@/types';

export function ChatMessageItem({ message }: { message: ChatMessage }) {
  if (message.type === 'image') return <Image source={{ uri: message.content }} style={{ width: 220, height: 220 }} />;
  if (message.type === 'video') return <Video source={{ uri: message.content }} useNativeControls resizeMode={ResizeMode.CONTAIN} style={{ width: 260, height: 220 }} />;
  return (
    <View className="mb-2 rounded-xl bg-zinc-900 p-3">
      <Text className="text-white" style={{ lineHeight: 22 }}>
        {message.content}
      </Text>
    </View>
  );
}
