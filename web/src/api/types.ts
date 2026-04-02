export interface Task {
  id: string;
  title: string;
  description: string;
  status: 'todo' | 'in_progress' | 'blocked' | 'done';
  priority: number;
  project: string;
  links: string[];
  tags: string[];
  blocked_by: string[];
  created_at: string;
  updated_at: string;
  last_active_at: string;
}

export interface ContextEntry {
  id: string;
  task_id: string;
  content: string;
  entry_type: 'note' | 'progress' | 'blocker';
  created_at: string;
}

export interface CreateTaskRequest {
  title: string;
  description?: string;
  status?: Task['status'];
  priority?: number;
  project?: string;
  links?: string[];
  tags?: string[];
  blocked_by?: string[];
}

export interface ChatRequest {
  message: string;
}

export type StoryStatus = 'active' | 'completed' | 'archived';
export type LayerStatus = 'pending' | 'in_progress' | 'review' | 'merged';

export interface Story {
  id: string;
  title: string;
  description: string;
  layer_chain: string;
  status: StoryStatus;
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
  sort_order: number;
  created_at: string;
  updated_at: string;
}

export interface CreateStoryRequest {
  title: string;
  description?: string;
  layer_chain?: string;
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
