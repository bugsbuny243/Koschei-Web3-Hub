export type Plan = { id: string; name: string; monthly_credits: number; price_try: number };
export type UserSession = { token: string };
export type ChatMessageType = 'text' | 'code' | 'image' | 'video';
export type ChatMessage = { id: string; role: 'user' | 'assistant'; content: string; type: ChatMessageType };
