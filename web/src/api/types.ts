export type StoryStatus = 'active' | 'completed' | 'archived';
export type TaskStatus = 'pending' | 'in_progress' | 'review' | 'merged' | 'released';

export interface Story {
  id: string;
  title: string;
  description: string;
  layer_chain: string;
  status: StoryStatus;
  priority: number;
  project: string;
  tags: string[];
  snoozed_until: string | null;
  last_active_at: string;
  created_at: string;
  updated_at: string;
  tasks: StoryTask[];
}

export interface StoryTask {
  id: string;
  story_id: string;
  name: string;
  status: TaskStatus;
  pr_url: string;
  pr_number: number;
  pr_state: string;
  pr_title: string;
  pr_review_decision: string;
  pr_is_draft: boolean;
  pr_check_status: string;
  pr_head_ref_name: string;
  pr_updated_at: string;
  pr_released: boolean;
  sort_order: number;
  depends_on: string[];
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
  tags?: string[];
}

export interface CreateTaskRequest {
  name: string;
  status?: TaskStatus;
  pr_url?: string;
}

export interface LayerChain {
  name: string;
  layers: string[];
}

export type SnoozeDuration = '30m' | '1h' | '3h' | 'tomorrow' | 'next_week';

export interface ScanOrgPRsResult {
  scanned: number;
  created: number;
  skipped: number;
}
