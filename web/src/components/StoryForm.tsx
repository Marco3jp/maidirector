import { useState, useEffect } from 'react';
import type { Story, CreateStoryRequest, LayerChain } from '../api/types';
import { api } from '../api/client';

interface Props {
  onCreated: (story: Story) => void;
}

export function StoryForm({ onCreated }: Props) {
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [layerChain, setLayerChain] = useState('');
  const [priority, setPriority] = useState(3);
  const [project, setProject] = useState('');
  const [tagsInput, setTagsInput] = useState('');
  const [chains, setChains] = useState<LayerChain[]>([]);
  const [expanded, setExpanded] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    api.getLayerChains().then(setChains).catch(console.error);
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim() || submitting) return;

    const tags = tagsInput.trim()
      ? tagsInput.split(',').map((t) => t.trim()).filter(Boolean)
      : [];

    const data: CreateStoryRequest = {
      title: title.trim(),
      description: description.trim(),
      layer_chain: layerChain || undefined,
      priority: priority || undefined,
      project: project.trim() || undefined,
      tags: tags.length > 0 ? tags : undefined,
    };

    setSubmitting(true);
    try {
      const created = await api.createStory(data);
      setTitle('');
      setDescription('');
      setLayerChain('');
    setPriority(3);
      setProject('');
      setTagsInput('');
      setExpanded(false);
      onCreated(created);
    } catch (e) {
      console.error('Failed to create story:', e);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <form className="task-form" onSubmit={handleSubmit}>
      <div className="task-form-main">
        <input
          type="text"
          placeholder="新しいストーリー / タスクを追加..."
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          onFocus={() => setExpanded(true)}
        />
        <button type="submit" disabled={!title.trim() || submitting}>
          {submitting ? '追加中...' : '追加'}
        </button>
      </div>

      {expanded && (
        <div className="task-form-details">
          <textarea
            placeholder="説明（任意）"
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
                <option value={0}>なし</option>
                <option value={1}>P1 (最高)</option>
                <option value={2}>P2</option>
                <option value={3}>P3 (普通)</option>
                <option value={4}>P4</option>
                <option value={5}>P5 (低)</option>
              </select>
            </label>
          </div>
          <div className="task-form-row">
            <input
              type="text"
              placeholder="タグ（カンマ区切り）"
              value={tagsInput}
              onChange={(e) => setTagsInput(e.target.value)}
            />
            <label>
              レイヤーチェーン:
              <select value={layerChain} onChange={(e) => setLayerChain(e.target.value)}>
                <option value="">なし（単タスク）</option>
                {chains.map((c) => (
                  <option key={c.name} value={c.name}>
                    {c.name} ({c.layers.join(' → ')})
                  </option>
                ))}
              </select>
            </label>
          </div>
        </div>
      )}
    </form>
  );
}
