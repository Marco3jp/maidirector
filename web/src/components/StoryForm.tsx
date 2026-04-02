import { useState, useEffect } from 'react';
import type { Story, CreateStoryRequest, LayerChain } from '../api/types';
import { api } from '../api/client';

interface Props {
  onCreated: (story: Story) => void;
}

export function StoryForm({ onCreated }: Props) {
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [layerChain, setLayerChain] = useState('default');
  const [chains, setChains] = useState<LayerChain[]>([]);
  const [expanded, setExpanded] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    api.getLayerChains().then(setChains).catch(console.error);
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim() || submitting) return;

    const data: CreateStoryRequest = {
      title: title.trim(),
      description: description.trim(),
      layer_chain: layerChain,
    };

    setSubmitting(true);
    try {
      const created = await api.createStory(data);
      setTitle('');
      setDescription('');
      setLayerChain('default');
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
          placeholder="新しいストーリーを追加..."
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
            placeholder="ストーリーの説明（任意）"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={2}
          />
          <div className="task-form-row">
            <label>
              レイヤーチェーン:
              <select value={layerChain} onChange={(e) => setLayerChain(e.target.value)}>
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
