import { ReactNode } from 'react';
import { Pressable, Text, TextInput, View } from 'react-native';

export const Button = ({ label, onPress }: { label: string; onPress: () => void }) => (
  <Pressable className="rounded-xl bg-[#00ff87] px-4 py-3" onPress={onPress}><Text className="font-bold text-black">{label}</Text></Pressable>
);
export const Card = ({ children }: { children: ReactNode }) => <View className="rounded-2xl border border-zinc-800 bg-zinc-950 p-4">{children}</View>;
export const Input = (props: any) => <TextInput placeholderTextColor="#6b7280" className="rounded-xl border border-zinc-700 bg-zinc-900 px-3 py-3 text-white" {...props} />;
export const LoadingState = ({ text = 'Loading...' }: { text?: string }) => <Text className="text-zinc-400">{text}</Text>;
export const ErrorState = ({ text }: { text: string }) => <Text className="text-red-400">{text}</Text>;
