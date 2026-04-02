import type { Task, CreateTaskRequest, ContextEntry } from './types';

const BASE = '/api';

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export const api = {
  listTasks(status?: string, project?: string): Promise<Task[]> {
    const params = new URLSearchParams();
    if (status) params.set('status', status);
    if (project) params.set('project', project);
    const qs = params.toString();
    return request(`/tasks${qs ? `?${qs}` : ''}`);
  },

  getTask(id: string): Promise<Task> {
    return request(`/tasks/${id}`);
  },

  createTask(data: CreateTaskRequest): Promise<Task> {
    return request('/tasks', { method: 'POST', body: JSON.stringify(data) });
  },

  updateTask(id: string, data: Partial<Task>): Promise<Task> {
    return request(`/tasks/${id}`, { method: 'PUT', body: JSON.stringify(data) });
  },

  deleteTask(id: string): Promise<void> {
    return request(`/tasks/${id}`, { method: 'DELETE' });
  },

  addContext(taskId: string, content: string, entryType: string): Promise<ContextEntry> {
    return request(`/tasks/${taskId}/context`, {
      method: 'POST',
      body: JSON.stringify({ content, entry_type: entryType }),
    });
  },

  listContext(taskId: string): Promise<ContextEntry[]> {
    return request(`/tasks/${taskId}/context`);
  },

  async *suggestStream(): AsyncGenerator<string> {
    const res = await fetch(`${BASE}/suggest`, { method: 'POST' });
    if (!res.body) throw new Error('No response body');

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const data = JSON.parse(line.slice(6));
          if (data.chunk) yield data.chunk;
          if (data.done) return;
          if (data.error) throw new Error(data.error);
        }
      }
    }
  },

  async *chatStream(message: string): AsyncGenerator<string> {
    const res = await fetch(`${BASE}/chat`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message }),
    });
    if (!res.body) throw new Error('No response body');

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const data = JSON.parse(line.slice(6));
          if (data.chunk) yield data.chunk;
          if (data.done) return;
          if (data.error) throw new Error(data.error);
        }
      }
    }
  },
};
