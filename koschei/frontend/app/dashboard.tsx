import { useState } from 'react';
import { ScrollView, Text, View } from 'react-native';
import { ChatMessageItem } from '@/components/ChatMessage';
import { Sidebar } from '@/components/Sidebar';
import { Button, ErrorState, Input } from '@/components/ui';
import { api } from '@/lib/api';

export default function Dashboard() {
  const [prompt,setPrompt]=useState(''); const [messages,setMessages]=useState<any[]>([]); const [error,setError]=useState('');
  const [credits,setCredits]=useState(0);
  const send = async ()=>{ try { setError(''); const res:any=await api.chat({prompt}); const content = res.output || res.message || 'No output'; const type = /https?:\/\/.+\.(png|jpg|jpeg|gif)/i.test(content)?'image':/https?:\/\/.+\.(mp4|webm)/i.test(content)?'video':content.includes('```')?'code':'text'; setMessages(m=>[...m,{id:Date.now().toString(),role:'user',content:prompt,type:'text'},{id:(Date.now()+1).toString(),role:'assistant',content,type}]); setPrompt(''); if (typeof res.credits_remaining==='number') setCredits(res.credits_remaining); } catch(e:any){ setError(e.message); }};
  return <View className="flex-1 bg-[#0a0a0a] p-4 md:flex-row gap-4"><Sidebar credits={credits} plan="free"/><View className="flex-1"><ScrollView className="mb-3">{messages.map(m=><ChatMessageItem key={m.id} message={m}/>)}</ScrollView>{!!error && <ErrorState text={error}/>}<Input placeholder="Ask Koschei..." value={prompt} onChangeText={setPrompt}/><View className="mt-2"><Button label="Send" onPress={send}/></View></View></View>;
}
