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
