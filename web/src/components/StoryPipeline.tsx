import { useState, useEffect, useCallback } from 'react';
import type { Story, StoryStatus, LayerChain, LayerStatus, StoryLayer } from '../api/types';
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

const STORY_STATUS_LABELS: Record<StoryStatus, string> = {
  active: 'アクティブ',
  completed: '完了',
  archived: 'アーカイブ',
};

const STORY_STATUS_COLORS: Record<StoryStatus, string> = {
  active: '#3b82f6',
  completed: '#22c55e',
  archived: '#6b7280',
};

const REVIEW_LABELS: Record<string, string> = {
  APPROVED: 'Approved',
  CHANGES_REQUESTED: 'Changes Requested',
  REVIEW_REQUIRED: 'Review Required',
};

const REVIEW_COLORS: Record<string, string> = {
  APPROVED: '#22c55e',
  CHANGES_REQUESTED: '#ef4444',
  REVIEW_REQUIRED: '#f59e0b',
};

const CI_STATUS_DISPLAY: Record<string, { label: string; color: string; icon: string }> = {
  SUCCESS: { label: 'CI pass', color: '#22c55e', icon: '✓' },
  FAILURE: { label: 'CI fail', color: '#ef4444', icon: '✗' },
  PENDING: { label: 'CI 実行中', color: '#f59e0b', icon: '⏳' },
};

function getPRStatusLabel(layer: StoryLayer): { label: string; color: string; className: string } {
  if (!layer.pr_url) {
    return { label: '未着手', color: '#6b7280', className: 'pipeline-pr-state-none' };
  }
  if (layer.pr_state === 'MERGED') {
    return { label: 'マージ済み', color: '#8b5cf6', className: 'pipeline-pr-state-merged' };
  }
  if (layer.pr_is_draft) {
    return { label: 'オープン（ドラフト）', color: '#6b7280', className: 'pipeline-pr-state-draft' };
  }
  if (layer.pr_state === 'OPEN') {
    return { label: 'オープン', color: '#22c55e', className: 'pipeline-pr-state-open' };
  }
  if (layer.pr_state === 'CLOSED') {
    return { label: 'クローズ', color: '#ef4444', className: 'pipeline-pr-state-closed' };
  }
  return { label: '未着手', color: '#6b7280', className: 'pipeline-pr-state-none' };
}

interface Props {
  stories: Story[];
  onRefresh: () => void;
}

export function StoryPipeline({ stories, onRefresh }: Props) {
  const [chains, setChains] = useState<LayerChain[]>([]);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [contextId, setContextId] = useState<string | null>(null);
  const [refreshingAll, setRefreshingAll] = useState(false);

  useEffect(() => {
    api.getLayerChains().then(setChains).catch(console.error);
  }, []);

  const handleRefreshAllPRs = async () => {
    setRefreshingAll(true);
    try {
      await api.refreshAllPRs();
      onRefresh();
    } catch (e) {
      console.error('Failed to refresh all PRs:', e);
    } finally {
      setRefreshingAll(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('このストーリーを削除しますか？')) return;
    try {
      await api.deleteStory(id);
      onRefresh();
    } catch (e) {
      console.error('Failed to delete story:', e);
    }
  };

  const handleStoryStatusChange = async (story: Story, newStatus: StoryStatus) => {
    try {
      await api.updateStory(story.id, { ...story, status: newStatus });
      onRefresh();
    } catch (e) {
      console.error('Failed to update story:', e);
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

  const handleSetPRUrl = async (story: Story, layerId: string, prUrl: string) => {
    try {
      const layer = story.layers.find((l) => l.id === layerId);
      if (!layer) return;
      await api.updateLayer(story.id, layerId, { ...layer, pr_url: prUrl });
      onRefresh();
    } catch (e) {
      console.error('Failed to set PR URL:', e);
    }
  };

  const handleRefreshPR = async (storyId: string, layerId: string) => {
    try {
      await api.refreshLayerPR(storyId, layerId);
      onRefresh();
    } catch (e) {
      console.error('Failed to refresh PR:', e);
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

  const hasLayers = stories.some((s) => s.layers.length > 0);

  if (stories.length === 0) {
    return <div className="empty-state">ストーリーがありません。新しいストーリーを追加してみましょう。</div>;
  }

  return (
    <div className="story-list">
      {hasLayers && (
        <div className="story-list-toolbar">
          <button
            className="btn-refresh-all"
            onClick={handleRefreshAllPRs}
            disabled={refreshingAll}
          >
            {refreshingAll ? '↻ 更新中...' : '↻ 全PR情報を更新'}
          </button>
        </div>
      )}
      {stories.map((story) => {
        const chainLayers = getChainLayers(story.layer_chain);
        const missingLayers = chainLayers.filter(
          (name) => !story.layers.some((l) => l.layer_name === name),
        );
        const isSingleTask = story.layers.length === 0;

        return (
          <div key={story.id} className={`story-card ${isSingleTask ? 'story-single-task' : ''}`}>
            <div className="story-header">
              <div className="story-info">
                <span
                  className="story-status-badge"
                  style={{ backgroundColor: STORY_STATUS_COLORS[story.status] }}
                >
                  {STORY_STATUS_LABELS[story.status]}
                </span>
                {story.priority > 0 && <span className="task-priority">P{story.priority}</span>}
                {story.project && <span className="task-project">{story.project}</span>}
                <h3 className="story-title">{story.title}</h3>
                {!isSingleTask && story.layer_chain && (
                  <span className="story-chain-badge">{story.layer_chain}</span>
                )}
              </div>
              <div className="story-actions-top">
                <button
                  className="btn-icon"
                  onClick={() => setContextId(contextId === story.id ? null : story.id)}
                  title="メモ追加"
                >
                  📝
                </button>
                {!isSingleTask && (
                  <button
                    className="btn-icon"
                    onClick={() => setExpandedId(expandedId === story.id ? null : story.id)}
                    title="レイヤー管理"
                  >
                    ⚙
                  </button>
                )}
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

            {story.tags.length > 0 && (
              <div className="task-tags">
                {story.tags.map((tag, i) => (
                  <span key={i} className="task-tag">{tag}</span>
                ))}
              </div>
            )}

            {story.links.length > 0 && (
              <div className="task-links">
                {story.links.map((link, i) => (
                  <a key={i} href={link} target="_blank" rel="noopener noreferrer" className="task-link">
                    {(() => { try { return new URL(link).hostname; } catch { return link; } })()}
                  </a>
                ))}
              </div>
            )}

            {!isSingleTask && (
              <div className="pipeline">
                {story.layers.map((layer, idx) => {
                  const blocked = isBlocked(story, idx);
                  return (
                    <PipelineStage
                      key={layer.id}
                      layer={layer}
                      storyId={story.id}
                      idx={idx}
                      blocked={blocked}
                      onStatusChange={(newStatus) => handleLayerStatusChange(story, layer.id, newStatus)}
                      onSetPRUrl={(prUrl) => handleSetPRUrl(story, layer.id, prUrl)}
                      onRefreshPR={() => handleRefreshPR(story.id, layer.id)}
                    />
                  );
                })}
              </div>
            )}

            <div className="story-footer">
              <select
                className="story-status-select"
                value={story.status}
                onChange={(e) => handleStoryStatusChange(story, e.target.value as StoryStatus)}
              >
                <option value="active">アクティブ</option>
                <option value="completed">完了</option>
                <option value="archived">アーカイブ</option>
              </select>
            </div>

            {contextId === story.id && (
              <ContextInput storyId={story.id} onAdded={() => { setContextId(null); onRefresh(); }} />
            )}

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

function PipelineStage({
  layer,
  storyId,
  idx,
  blocked,
  onStatusChange,
  onSetPRUrl,
  onRefreshPR,
}: {
  layer: StoryLayer;
  storyId: string;
  idx: number;
  blocked: boolean;
  onStatusChange: (status: LayerStatus) => void;
  onSetPRUrl: (prUrl: string) => void;
  onRefreshPR: () => void;
}) {
  const [showPRInput, setShowPRInput] = useState(false);
  const [prUrlInput, setPrUrlInput] = useState('');
  const [refreshing, setRefreshing] = useState(false);

  const handlePRSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!prUrlInput.trim()) return;
    onSetPRUrl(prUrlInput.trim());
    setPrUrlInput('');
    setShowPRInput(false);
  };

  const handleRefresh = async () => {
    setRefreshing(true);
    try {
      await onRefreshPR();
    } finally {
      setRefreshing(false);
    }
  };

  const prStatus = getPRStatusLabel(layer);
  const ciInfo = layer.pr_check_status ? CI_STATUS_DISPLAY[layer.pr_check_status] : null;

  return (
    <div className={`pipeline-stage ${blocked ? 'pipeline-blocked' : ''}`}>
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
          {LAYER_STATUS_LABELS[layer.status]}
        </span>

        <span className={`pipeline-pr-state ${prStatus.className}`} style={{ color: prStatus.color }}>
          {prStatus.label}
        </span>

        {layer.pr_url ? (
          <div className="pipeline-pr-info">
            <div className="pipeline-pr-header">
              <a
                href={layer.pr_url}
                target="_blank"
                rel="noopener noreferrer"
                className="pipeline-pr-link"
              >
                PR #{layer.pr_number || '?'}
              </a>
              <button
                className="btn-icon btn-refresh"
                onClick={handleRefresh}
                disabled={refreshing}
                title="PR情報を更新"
              >
                {refreshing ? '...' : '↻'}
              </button>
            </div>
            {layer.pr_title && (
              <div className="pipeline-pr-title" title={layer.pr_title}>
                {layer.pr_title.length > 30 ? layer.pr_title.slice(0, 30) + '…' : layer.pr_title}
              </div>
            )}
            <div className="pipeline-pr-meta">
              {layer.pr_review_decision && (
                <span
                  className="pipeline-pr-review"
                  style={{ color: REVIEW_COLORS[layer.pr_review_decision] || 'var(--text-muted)' }}
                >
                  {REVIEW_LABELS[layer.pr_review_decision] || layer.pr_review_decision}
                </span>
              )}
              {ciInfo && (
                <span className="pipeline-ci-status" style={{ color: ciInfo.color }}>
                  {ciInfo.icon} {ciInfo.label}
                </span>
              )}
            </div>
          </div>
        ) : (
          <div className="pipeline-pr-actions">
            {showPRInput ? (
              <form className="pipeline-pr-form" onSubmit={handlePRSubmit}>
                <input
                  type="url"
                  placeholder="PR URL..."
                  value={prUrlInput}
                  onChange={(e) => setPrUrlInput(e.target.value)}
                  autoFocus
                />
                <button type="submit" disabled={!prUrlInput.trim()}>OK</button>
                <button type="button" onClick={() => setShowPRInput(false)} className="btn-cancel">✕</button>
              </form>
            ) : (
              <button
                className="btn-add-pr"
                onClick={() => setShowPRInput(true)}
              >
                + PR
              </button>
            )}
          </div>
        )}

        <select
          className="pipeline-status-select"
          value={layer.status}
          onChange={(e) => onStatusChange(e.target.value as LayerStatus)}
        >
          <option value="pending">未着手</option>
          <option value="in_progress">作業中</option>
          <option value="review">レビュー中</option>
          <option value="merged">マージ済</option>
        </select>
      </div>
    </div>
  );
}

function ContextInput({ storyId, onAdded }: { storyId: string; onAdded: () => void }) {
  const [content, setContent] = useState('');
  const [entryType, setEntryType] = useState('note');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!content.trim()) return;
    try {
      await api.addContext(storyId, content, entryType);
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
              {layer.pr_url && (
                <span className="layer-manage-pr">
                  PR #{layer.pr_number || '?'} ({layer.pr_state || '未取得'})
                </span>
              )}
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
