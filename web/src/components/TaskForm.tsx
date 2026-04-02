import { useState } from 'react';
import type { CreateTaskRequest } from '../api/types';
import { api } from '../api/client';

interface Props {
  onCreated: () => void;
}

export function TaskForm({ onCreated }: Props) {
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [project, setProject] = useState('');
  const [priority, setPriority] = useState(3);
  const [expanded, setExpanded] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim()) return;

    const data: CreateTaskRequest = {
      title: title.trim(),
      description: description.trim(),
      project: project.trim(),
      priority,
    };

    try {
      await api.createTask(data);
      setTitle('');
      setDescription('');
      setProject('');
      setPriority(3);
      setExpanded(false);
      onCreated();
    } catch (e) {
      console.error('Failed to create task:', e);
    }
  };

  return (
    <form className="task-form" onSubmit={handleSubmit}>
      <div className="task-form-main">
        <input
          type="text"
          placeholder="新しいタスクを追加..."
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          onFocus={() => setExpanded(true)}
        />
        <button type="submit" disabled={!title.trim()}>追加</button>
      </div>

      {expanded && (
        <div className="task-form-details">
          <textarea
            placeholder="詳細（任意）"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={2}
          />
          <div className="task-form-row">
            <input
              type="text"
              placeholder="プロジェクト"
              value={project}
              onChange={(e) => setProject(e.target.value)}
            />
            <label>
              優先度:
              <select value={priority} onChange={(e) => setPriority(Number(e.target.value))}>
                <option value={1}>P1 (最高)</option>
                <option value={2}>P2</option>
                <option value={3}>P3 (普通)</option>
                <option value={4}>P4</option>
                <option value={5}>P5 (低)</option>
              </select>
            </label>
          </div>
        </div>
      )}
    </form>
  );
}
