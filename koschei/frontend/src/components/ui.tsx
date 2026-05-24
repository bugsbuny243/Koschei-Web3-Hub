import { ReactNode } from 'react';
import { Pressable, Text, TextInput, View } from 'react-native';

export const Button = ({ label, onPress, disabled = false }: { label: string; onPress: () => void; disabled?: boolean }) => (
  <Pressable disabled={disabled} className={`rounded-2xl border px-5 py-4 ${disabled ? 'border-zinc-600 bg-zinc-700/60' : 'border-emerald-300/40 bg-[#00ff87] shadow-[0_0_18px_rgba(0,255,135,0.4)]'}`} onPress={onPress}><Text className={`text-base font-bold ${disabled ? 'text-zinc-300' : 'text-black'}`}>{label}</Text></Pressable>
);
export const Card = ({ children }: { children: ReactNode }) => <View className="rounded-2xl border border-emerald-500/20 bg-[#060a13]/90 p-4 shadow-[0_0_24px_rgba(56,189,248,0.12)]">{children}</View>;
export const Input = (props: any) => <TextInput placeholderTextColor="#7c8ea8" className="rounded-2xl border border-cyan-500/20 bg-[#040812] px-4 py-4 text-base text-white" {...props} />;
export const LoadingState = ({ text = 'Loading...' }: { text?: string }) => <Text className="text-zinc-400">{text}</Text>;
export const ErrorState = ({ text }: { text: string }) => <Text className="text-red-400">{text}</Text>;
