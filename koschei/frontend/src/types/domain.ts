export type OwnerOrderStatus =
  | 'draft'
  | 'requirements_received'
  | 'in_production'
  | 'ready_for_review'
  | 'ready_for_delivery'
  | 'delivered'
  | 'revision_requested'
  | 'completed'
  | 'cancelled';

export interface AiTool {
  id: string;
  name: string;
  capability: string;
  category: 'code' | 'app-builder' | 'web-builder' | 'game-builder' | 'image' | 'video' | 'audio';
  model: string;
  description: string;
}

export interface UserProject {
  id: string;
  title: string;
  category: string;
  updatedAt: string;
  creditsUsed: number;
}

export interface GenerationHistoryItem {
  id: string;
  toolId: string;
  title: string;
  status: 'queued' | 'running' | 'completed' | 'failed';
  createdAt: string;
}

export interface DeliveryPackage {
  id: string;
  orderId: string;
  summary: string;
  assets: string[];
  notes: string;
}

export interface OwnerOrder {
  id: string;
  clientName: string;
  status: OwnerOrderStatus;
  sourcePlatform: 'fiverr' | 'direct' | 'other';
  serviceType: string;
  dueDate: string;
}

export interface ServiceTemplate {
  id: string;
  name: string;
  description: string;
  checklist: string[];
  basePriceUsd: number;
}
