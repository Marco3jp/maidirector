import { useState, useEffect, useCallback } from 'react';
import type { Story, LayerChain, LayerStatus } from '../api/types';
import { api } from '../api/client';

const LAYER_STATUS_LABELS: Record<LayerStatus, string> = {
  pending: '未着手',
  in_progress: '作業中',
  review: 'レビュー中',
  merged: 'マージ済',
};

const LAYER_STATUS_COLORS: Record<LayerStatus, string> = {
  pending: '#6b7280',
  in_progress: '#3b82f6',
  review: '#f59e0b',
  merged: '#22c55e',
};

interface Props {
  stories: Story[];
  onRefresh: () => void;
}

export function StoryPipeline({ stories, onRefresh }: Props) {
  const [chains, setChains] = useState<LayerChain[]>([]);
  const [expandedId, setExpandedId] = useState<string | null>(null);

  useEffect(() => {
    api.getLayerChains().then(setChains).catch(console.error);
  }, []);

  const handleDelete = async (id: string) => {
    if (!confirm('このストーリーを削除しますか？')) return;
    try {
      await api.deleteStory(id);
      onRefresh();
    } catch (e) {
      console.error('Failed to delete story:', e);
    }
  };

  const handleLayerStatusChange = async (story: Story, layerId: string, newStatus: LayerStatus) => {
    try {
      const layer = story.layers.find((l) => l.id === layerId);
      if (!layer) return;
      await api.updateLayer(story.id, layerId, { ...layer, status: newStatus });
      onRefresh();
    } catch (e) {
      console.error('Failed to update layer:', e);
    }
  };

  const handleAddLayer = async (storyId: string, layerName: string) => {
    try {
      await api.addLayer(storyId, { layer_name: layerName });
      onRefresh();
    } catch (e) {
      console.error('Failed to add layer:', e);
    }
  };

  const handleDeleteLayer = async (storyId: string, layerId: string) => {
    try {
      await api.deleteLayer(storyId, layerId);
      onRefresh();
    } catch (e) {
      console.error('Failed to delete layer:', e);
    }
  };

  const getChainLayers = useCallback(
    (chainName: string): string[] => {
      const chain = chains.find((c) => c.name === chainName);
      return chain ? chain.layers : [];
    },
    [chains],
  );

  const isBlocked = (story: Story, layerIndex: number): boolean => {
    if (layerIndex === 0) return false;
    const prevLayer = story.layers[layerIndex - 1];
    return prevLayer ? prevLayer.status !== 'merged' : false;
  };

  if (stories.length === 0) {
    return <div className="empty-state">ストーリーがありません。新しいストーリーを追加してみましょう。</div>;
  }

  return (
    <div className="story-list">
      {stories.map((story) => {
        const chainLayers = getChainLayers(story.layer_chain);
        const missingLayers = chainLayers.filter(
          (name) => !story.layers.some((l) => l.layer_name === name),
        );

        return (
          <div key={story.id} className="story-card">
            <div className="story-header">
              <div className="story-info">
                <h3 className="story-title">{story.title}</h3>
                <span className="story-chain-badge">{story.layer_chain}</span>
              </div>
              <div className="story-actions-top">
                <button
                  className="btn-icon"
                  onClick={() => setExpandedId(expandedId === story.id ? null : story.id)}
                  title="レイヤー管理"
                >
                  ⚙
                </button>
                <button
                  className="btn-icon btn-danger"
                  onClick={() => handleDelete(story.id)}
                  title="削除"
                >
                  ✕
                </button>
              </div>
            </div>

            {story.description && (
              <p className="story-description">{story.description}</p>
            )}

            <div className="pipeline">
              {story.layers.map((layer, idx) => {
                const blocked = isBlocked(story, idx);
                return (
                  <div
                    key={layer.id}
                    className={`pipeline-stage ${blocked ? 'pipeline-blocked' : ''}`}
                  >
                    <div className="pipeline-connector">
                      {idx > 0 && <span className="pipeline-arrow">→</span>}
                    </div>
                    <div
                      className="pipeline-card"
                      style={{ borderTopColor: LAYER_STATUS_COLORS[layer.status] }}
                    >
                      <div className="pipeline-layer-name">{layer.layer_name}</div>
                      <span
                        className="pipeline-status"
                        style={{ backgroundColor: LAYER_STATUS_COLORS[layer.status] }}
                      >
                        {blocked && layer.status !== 'merged' ? 'ブロック中' : LAYER_STATUS_LABELS[layer.status]}
                      </span>
                      {layer.pr_url && (
                        <a
                          href={layer.pr_url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="pipeline-pr-link"
                        >
                          PR #{layer.pr_number || '?'}
                        </a>
                      )}
                      <select
                        className="pipeline-status-select"
                        value={layer.status}
                        onChange={(e) =>
                          handleLayerStatusChange(story, layer.id, e.target.value as LayerStatus)
                        }
                      >
                        <option value="pending">未着手</option>
                        <option value="in_progress">作業中</option>
                        <option value="review">レビュー中</option>
                        <option value="merged">マージ済</option>
                      </select>
                    </div>
                  </div>
                );
              })}
              {story.layers.length === 0 && (
                <div className="pipeline-empty">レイヤーがありません</div>
              )}
            </div>

            {expandedId === story.id && (
              <LayerManager
                story={story}
                missingLayers={missingLayers}
                onAddLayer={handleAddLayer}
                onDeleteLayer={handleDeleteLayer}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

function LayerManager({
  story,
  missingLayers,
  onAddLayer,
  onDeleteLayer,
}: {
  story: Story;
  missingLayers: string[];
  onAddLayer: (storyId: string, layerName: string) => void;
  onDeleteLayer: (storyId: string, layerId: string) => void;
}) {
  const [customName, setCustomName] = useState('');

  const handleAddCustom = (e: React.FormEvent) => {
    e.preventDefault();
    if (!customName.trim()) return;
    onAddLayer(story.id, customName.trim());
    setCustomName('');
  };

  return (
    <div className="layer-manager">
      <div className="layer-manager-header">レイヤー管理</div>

      {missingLayers.length > 0 && (
        <div className="layer-quick-add">
          {missingLayers.map((name) => (
            <button
              key={name}
              className="layer-add-btn"
              onClick={() => onAddLayer(story.id, name)}
            >
              + {name}
            </button>
          ))}
        </div>
      )}

      <form className="layer-custom-add" onSubmit={handleAddCustom}>
        <input
          type="text"
          placeholder="カスタムレイヤー名..."
          value={customName}
          onChange={(e) => setCustomName(e.target.value)}
        />
        <button type="submit" disabled={!customName.trim()}>
          追加
        </button>
      </form>

      {story.layers.length > 0 && (
        <div className="layer-list-manage">
          {story.layers.map((layer) => (
            <div key={layer.id} className="layer-manage-item">
              <span>{layer.layer_name}</span>
              <button
                className="btn-icon btn-danger"
                onClick={() => onDeleteLayer(story.id, layer.id)}
              >
                ✕
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
