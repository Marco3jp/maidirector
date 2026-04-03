import type { ContextEntry, Story, CreateStoryRequest, StoryTask, CreateTaskRequest, LayerChain, ScanOrgPRsResult, SnoozeDuration } from './types';

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
  listStories(status?: string): Promise<Story[]> {
    const params = new URLSearchParams();
    if (status) params.set('status', status);
    const qs = params.toString();
    return request(`/stories${qs ? `?${qs}` : ''}`);
  },

  getStory(id: string): Promise<Story> {
    return request(`/stories/${id}`);
  },

  createStory(data: CreateStoryRequest): Promise<Story> {
    return request('/stories', { method: 'POST', body: JSON.stringify(data) });
  },

  updateStory(id: string, data: Partial<Story>): Promise<Story> {
    return request(`/stories/${id}`, { method: 'PUT', body: JSON.stringify(data) });
  },

  deleteStory(id: string): Promise<void> {
    return request(`/stories/${id}`, { method: 'DELETE' });
  },

  addTask(storyId: string, data: CreateTaskRequest): Promise<StoryTask> {
    return request(`/stories/${storyId}/tasks`, { method: 'POST', body: JSON.stringify(data) });
  },

  updateTask(storyId: string, taskId: string, data: Partial<StoryTask>): Promise<StoryTask> {
    return request(`/stories/${storyId}/tasks/${taskId}`, { method: 'PUT', body: JSON.stringify(data) });
  },

  deleteTask(storyId: string, taskId: string): Promise<void> {
    return request(`/stories/${storyId}/tasks/${taskId}`, { method: 'DELETE' });
  },

  refreshTaskPR(storyId: string, taskId: string): Promise<StoryTask> {
    return request(`/stories/${storyId}/tasks/${taskId}/refresh-pr`, { method: 'POST' });
  },

  addTaskDep(storyId: string, taskId: string, dependsOnId: string): Promise<void> {
    return request(`/stories/${storyId}/tasks/${taskId}/deps`, {
      method: 'POST',
      body: JSON.stringify({ depends_on_id: dependsOnId }),
    });
  },

  removeTaskDep(storyId: string, taskId: string, depId: string): Promise<void> {
    return request(`/stories/${storyId}/tasks/${taskId}/deps/${depId}`, { method: 'DELETE' });
  },

  refreshAllPRs(): Promise<Story[]> {
    return request('/stories/refresh-prs', { method: 'POST' });
  },

  scanOrgPRs(): Promise<ScanOrgPRsResult> {
    return request('/stories/scan-org-prs', { method: 'POST' });
  },

  snoozeStory(id: string, duration: SnoozeDuration): Promise<Story> {
    return request(`/stories/${id}/snooze`, { method: 'POST', body: JSON.stringify({ duration }) });
  },

  unsnoozeStory(id: string): Promise<Story> {
    return request(`/stories/${id}/unsnooze`, { method: 'POST' });
  },

  addContext(storyId: string, content: string, entryType: string): Promise<ContextEntry> {
    return request(`/stories/${storyId}/context`, {
      method: 'POST',
      body: JSON.stringify({ content, entry_type: entryType }),
    });
  },

  listContext(storyId: string): Promise<ContextEntry[]> {
    return request(`/stories/${storyId}/context`);
  },

  updateContext(storyId: string, entryId: string, content: string, entryType: string): Promise<ContextEntry> {
    return request(`/stories/${storyId}/context/${entryId}`, {
      method: 'PUT',
      body: JSON.stringify({ content, entry_type: entryType }),
    });
  },

  deleteContext(storyId: string, entryId: string): Promise<void> {
    return request(`/stories/${storyId}/context/${entryId}`, { method: 'DELETE' });
  },

  getLayerChains(): Promise<LayerChain[]> {
    return request('/layer-chains');
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
