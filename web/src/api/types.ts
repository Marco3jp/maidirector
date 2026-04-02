export type StoryStatus = 'active' | 'completed' | 'archived';
export type LayerStatus = 'pending' | 'in_progress' | 'review' | 'merged';

export interface Story {
  id: string;
  title: string;
  description: string;
  layer_chain: string;
  status: StoryStatus;
  priority: number;
  project: string;
  links: string[];
  tags: string[];
  last_active_at: string;
  created_at: string;
  updated_at: string;
  layers: StoryLayer[];
}

export interface StoryLayer {
  id: string;
  story_id: string;
  layer_name: string;
  status: LayerStatus;
  pr_url: string;
  pr_number: number;
  pr_state: string;
  pr_title: string;
  pr_review_decision: string;
  pr_is_draft: boolean;
  pr_check_status: string;
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export interface ContextEntry {
  id: string;
  story_id: string;
  content: string;
  entry_type: 'note' | 'progress' | 'blocker';
  created_at: string;
}

export interface CreateStoryRequest {
  title: string;
  description?: string;
  layer_chain?: string;
  priority?: number;
  project?: string;
  links?: string[];
  tags?: string[];
}

export interface CreateLayerRequest {
  layer_name: string;
  status?: LayerStatus;
  pr_url?: string;
}

export interface LayerChain {
  name: string;
  layers: string[];
}

export interface ChatRequest {
  message: string;
}

export interface ScanOrgPRsResult {
  scanned: number;
  created: number;
  skipped: number;
}
