import { useState } from 'react';
import type { Task } from '../api/types';
import { api } from '../api/client';

interface Props {
  tasks: Task[];
  onRefresh: () => void;
}

const STATUS_LABELS: Record<string, string> = {
  todo: 'TODO',
  in_progress: '進行中',
  blocked: 'ブロック中',
  done: '完了',
};

const STATUS_COLORS: Record<string, string> = {
  todo: '#6b7280',
  in_progress: '#3b82f6',
  blocked: '#ef4444',
  done: '#22c55e',
};

export function TaskList({ tasks, onRefresh }: Props) {
  const [editingId, setEditingId] = useState<string | null>(null);

  const handleStatusChange = async (task: Task, newStatus: Task['status']) => {
    try {
      await api.updateTask(task.id, { ...task, status: newStatus });
      onRefresh();
    } catch (e) {
      console.error('Failed to update task:', e);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('このタスクを削除しますか？')) return;
    try {
      await api.deleteTask(id);
      onRefresh();
    } catch (e) {
      console.error('Failed to delete task:', e);
    }
  };

  if (tasks.length === 0) {
    return <div className="empty-state">タスクがありません。新しいタスクを追加してみましょう。</div>;
  }

  return (
    <div className="task-list">
      {tasks.map((task) => (
        <div key={task.id} className={`task-card status-${task.status}`}>
          <div className="task-header">
            <span
              className="task-status"
              style={{ backgroundColor: STATUS_COLORS[task.status] }}
            >
              {STATUS_LABELS[task.status]}
            </span>
            <span className="task-priority">P{task.priority}</span>
            {task.project && <span className="task-project">{task.project}</span>}
          </div>

          <h3 className="task-title">{task.title}</h3>

          {task.description && (
            <p className="task-description">{task.description}</p>
          )}

          {task.links.length > 0 && (
            <div className="task-links">
              {task.links.map((link, i) => (
                <a key={i} href={link} target="_blank" rel="noopener noreferrer" className="task-link">
                  {new URL(link).hostname}
                </a>
              ))}
            </div>
          )}

          {task.tags.length > 0 && (
            <div className="task-tags">
              {task.tags.map((tag, i) => (
                <span key={i} className="task-tag">{tag}</span>
              ))}
            </div>
          )}

          <div className="task-actions">
            <select
              value={task.status}
              onChange={(e) => handleStatusChange(task, e.target.value as Task['status'])}
            >
              <option value="todo">TODO</option>
              <option value="in_progress">進行中</option>
              <option value="blocked">ブロック中</option>
              <option value="done">完了</option>
            </select>
            <button className="btn-icon" onClick={() => setEditingId(editingId === task.id ? null : task.id)} title="メモ追加">
              📝
            </button>
            <button className="btn-icon btn-danger" onClick={() => handleDelete(task.id)} title="削除">
              ✕
            </button>
          </div>

          {editingId === task.id && (
            <ContextInput taskId={task.id} onAdded={() => { setEditingId(null); onRefresh(); }} />
          )}
        </div>
      ))}
    </div>
  );
}

function ContextInput({ taskId, onAdded }: { taskId: string; onAdded: () => void }) {
  const [content, setContent] = useState('');
  const [entryType, setEntryType] = useState('note');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!content.trim()) return;
    try {
      await api.addContext(taskId, content, entryType);
      setContent('');
      onAdded();
    } catch (e) {
      console.error('Failed to add context:', e);
    }
  };

  return (
    <form className="context-input" onSubmit={handleSubmit}>
      <select value={entryType} onChange={(e) => setEntryType(e.target.value)}>
        <option value="note">メモ</option>
        <option value="progress">進捗</option>
        <option value="blocker">ブロッカー</option>
      </select>
      <input
        type="text"
        placeholder="メモを追加..."
        value={content}
        onChange={(e) => setContent(e.target.value)}
        autoFocus
      />
      <button type="submit">追加</button>
    </form>
  );
}
