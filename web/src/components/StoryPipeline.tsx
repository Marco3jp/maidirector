import { useState, useEffect, useCallback } from 'react';
import type { Story, StoryStatus, LayerChain, TaskStatus, StoryTask, SnoozeDuration } from '../api/types';
import { api } from '../api/client';

const SNOOZE_OPTIONS: { label: string; value: SnoozeDuration }[] = [
  { label: '30分', value: '30m' },
  { label: '1時間', value: '1h' },
  { label: '3時間', value: '3h' },
  { label: '明日まで（09:00）', value: 'tomorrow' },
  { label: '来週まで（月 09:00）', value: 'next_week' },
];

function formatSnoozedUntil(snoozedUntil: string | null): string | null {
  if (!snoozedUntil) return null;
  const until = new Date(snoozedUntil);
  const now = new Date();
  if (until <= now) return null;
  const diffMs = until.getTime() - now.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 60) return `${diffMin}分後に復帰`;
  const diffH = Math.floor(diffMin / 60);
  if (diffH < 24) return `${diffH}時間後に復帰`;
  const diffD = Math.floor(diffH / 24);
  return `${diffD}日後に復帰`;
}

function extractRepoName(link: string): string | null {
  try {
    const url = new URL(link);
    if (url.hostname !== 'github.com') return null;
    const parts = url.pathname.split('/').filter(Boolean);
    if (parts.length >= 2) return parts[1];
    return null;
  } catch {
    return null;
  }
}

const TASK_STATUS_LABELS: Record<TaskStatus, string> = {
  pending: '未着手',
  in_progress: '作業中',
  review: 'レビュー中',
  merged: 'マージ済',
};

const TASK_STATUS_COLORS: Record<TaskStatus, string> = {
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

function getPRStatusLabel(t: StoryTask): { label: string; color: string; className: string } {
  if (!t.pr_url) {
    return { label: '未着手', color: '#6b7280', className: 'pipeline-pr-state-none' };
  }
  if (t.pr_state === 'MERGED') {
    return { label: 'マージ済み', color: '#8b5cf6', className: 'pipeline-pr-state-merged' };
  }
  if (t.pr_is_draft) {
    return { label: 'オープン（ドラフト）', color: '#6b7280', className: 'pipeline-pr-state-draft' };
  }
  if (t.pr_state === 'OPEN') {
    return { label: 'オープン', color: '#22c55e', className: 'pipeline-pr-state-open' };
  }
  if (t.pr_state === 'CLOSED') {
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
  const [snoozeMenuId, setSnoozeMenuId] = useState<string | null>(null);
  const [hiddenRepos, setHiddenRepos] = useState<Set<string>>(new Set());

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

  const handleStoryStatusChange = async (story: Story, newStatus: StoryStatus) => {
    try {
      await api.updateStory(story.id, { ...story, status: newStatus });
      onRefresh();
    } catch (e) {
      console.error('Failed to update story:', e);
    }
  };

  const handleStoryPriorityChange = async (story: Story, newPriority: number) => {
    try {
      await api.updateStory(story.id, { ...story, priority: newPriority });
      onRefresh();
    } catch (e) {
      console.error('Failed to update story priority:', e);
    }
  };

  const handleSnooze = async (id: string, duration: SnoozeDuration) => {
    setSnoozeMenuId(null);
    try {
      await api.snoozeStory(id, duration);
      onRefresh();
    } catch (e) {
      console.error('Failed to snooze story:', e);
    }
  };

  const handleUnsnooze = async (id: string) => {
    try {
      await api.unsnoozeStory(id);
      onRefresh();
    } catch (e) {
      console.error('Failed to unsnooze story:', e);
    }
  };

  const handleTaskStatusChange = async (story: Story, taskId: string, newStatus: TaskStatus) => {
    try {
      const t = story.tasks.find((t) => t.id === taskId);
      if (!t) return;
      await api.updateTask(story.id, taskId, { ...t, status: newStatus });
      onRefresh();
    } catch (e) {
      console.error('Failed to update task:', e);
    }
  };

  const handleSetPRUrl = async (story: Story, taskId: string, prUrl: string) => {
    try {
      const t = story.tasks.find((t) => t.id === taskId);
      if (!t) return;
      await api.updateTask(story.id, taskId, { ...t, pr_url: prUrl });
      onRefresh();
    } catch (e) {
      console.error('Failed to set PR URL:', e);
    }
  };

  const handleRefreshPR = async (storyId: string, taskId: string) => {
    try {
      await api.refreshTaskPR(storyId, taskId);
      onRefresh();
    } catch (e) {
      console.error('Failed to refresh PR:', e);
    }
  };

  const handleAddTask = async (storyId: string, taskName: string) => {
    try {
      await api.addTask(storyId, { name: taskName });
      onRefresh();
    } catch (e) {
      console.error('Failed to add task:', e);
    }
  };

  const handleDeleteTask = async (storyId: string, taskId: string) => {
    try {
      await api.deleteTask(storyId, taskId);
      onRefresh();
    } catch (e) {
      console.error('Failed to delete task:', e);
    }
  };

  const handleAddDep = async (storyId: string, taskId: string, dependsOnId: string) => {
    try {
      await api.addTaskDep(storyId, taskId, dependsOnId);
      onRefresh();
    } catch (e) {
      console.error('Failed to add dep:', e);
    }
  };

  const handleRemoveDep = async (storyId: string, taskId: string, depId: string) => {
    try {
      await api.removeTaskDep(storyId, taskId, depId);
      onRefresh();
    } catch (e) {
      console.error('Failed to remove dep:', e);
    }
  };

  const getChainLayers = useCallback(
    (chainName: string): string[] => {
      const chain = chains.find((c) => c.name === chainName);
      return chain ? chain.layers : [];
    },
    [chains],
  );

  const isBlocked = (story: Story, t: StoryTask): boolean => {
    if (!t.depends_on || t.depends_on.length === 0) return false;
    const taskById = Object.fromEntries(story.tasks.map((s) => [s.id, s]));
    return t.depends_on.some((depId) => {
      const dep = taskById[depId];
      return dep ? dep.status !== 'merged' : false;
    });
  };

  const singleTaskStories = stories.filter((s) => s.tasks.length === 0);
  const repoNames = [...new Set(
    singleTaskStories.flatMap((s) => s.links.map(extractRepoName).filter((r): r is string => r !== null))
  )].sort();

  const toggleHiddenRepo = (repo: string) => {
    setHiddenRepos((prev) => {
      const next = new Set(prev);
      if (next.has(repo)) {
        next.delete(repo);
      } else {
        next.add(repo);
      }
      return next;
    });
  };

  const filteredStories = stories.filter((s) => {
    if (s.tasks.length > 0) return true;
    if (hiddenRepos.size === 0) return true;
    const repo = s.links.map(extractRepoName).find((r): r is string => r !== null);
    return !repo || !hiddenRepos.has(repo);
  });

  if (stories.length === 0) {
    return <div className="empty-state">ストーリーがありません。新しいストーリーを追加してみましょう。</div>;
  }

  return (
    <div className="story-list">
      <div className="story-list-toolbar">
        {repoNames.length > 0 && (
          <div className="repo-filter-chips">
            {hiddenRepos.size > 0 && (
              <button
                className="repo-filter-clear"
                onClick={() => setHiddenRepos(new Set())}
              >
                すべて表示
              </button>
            )}
            {repoNames.map((repo) => {
              const hidden = hiddenRepos.has(repo);
              return (
                <button
                  key={repo}
                  className={`repo-filter-chip ${hidden ? 'hidden' : ''}`}
                  onClick={() => toggleHiddenRepo(repo)}
                  title={hidden ? `${repo} を表示する` : `${repo} を非表示にする`}
                >
                  {repo}
                </button>
              );
            })}
          </div>
        )}
      </div>
      {filteredStories.map((story) => {
        const chainLayers = getChainLayers(story.layer_chain);
        const existingNames = new Set(story.tasks.map((t) => t.name));
        const missingLayers = chainLayers.filter((name) => !existingNames.has(name));
        const isSingleTask = story.tasks.length === 0;
        const isSnoozed = story.snoozed_until ? new Date(story.snoozed_until) > new Date() : false;
        const snoozeLabel = formatSnoozedUntil(story.snoozed_until);

        return (
          <div key={story.id} className={`story-card ${isSingleTask ? 'story-single-task' : ''} ${isSnoozed ? 'story-snoozed' : ''}`}>
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
                {isSnoozed && snoozeLabel && (
                  <span className="snooze-badge">⏰ {snoozeLabel}</span>
                )}
              </div>
              <div className="story-actions-top">
                {isSnoozed ? (
                  <button
                    className="btn-icon"
                    onClick={() => handleUnsnooze(story.id)}
                    title="スヌーズ解除"
                  >
                    ⏰
                  </button>
                ) : (
                  <div className="snooze-menu-wrap">
                    <button
                      className="btn-icon"
                      onClick={() => setSnoozeMenuId(snoozeMenuId === story.id ? null : story.id)}
                      title="スヌーズ"
                    >
                      ⏰
                    </button>
                    {snoozeMenuId === story.id && (
                      <div className="snooze-dropdown">
                        {SNOOZE_OPTIONS.map((opt) => (
                          <button
                            key={opt.value}
                            className="snooze-option"
                            onClick={() => handleSnooze(story.id, opt.value)}
                          >
                            {opt.label}
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                )}
                <button
                  className="btn-icon"
                  onClick={() => setContextId(contextId === story.id ? null : story.id)}
                  title="メモ追加"
                >
                  📝
                </button>
                <button
                  className="btn-icon"
                  onClick={() => setExpandedId(expandedId === story.id ? null : story.id)}
                  title="タスク管理"
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
                {story.tasks.map((t, idx) => {
                  const blocked = isBlocked(story, t);
                  const prevTask = idx > 0 ? story.tasks[idx - 1] : null;
                  const showArrow = prevTask !== null && t.depends_on.includes(prevTask.id);
                  return (
                    <PipelineStage
                      key={t.id}
                      task={t}
                      storyId={story.id}
                      showArrow={showArrow}
                      blocked={blocked}
                      onStatusChange={(newStatus) => handleTaskStatusChange(story, t.id, newStatus)}
                      onSetPRUrl={(prUrl) => handleSetPRUrl(story, t.id, prUrl)}
                      onRefreshPR={() => handleRefreshPR(story.id, t.id)}
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
              <select
                className="story-priority-select"
                value={story.priority}
                onChange={(e) => handleStoryPriorityChange(story, Number(e.target.value))}
              >
                <option value={0}>優先度なし</option>
                <option value={1}>P1 (最高)</option>
                <option value={2}>P2</option>
                <option value={3}>P3 (普通)</option>
                <option value={4}>P4</option>
                <option value={5}>P5 (低)</option>
              </select>
            </div>

            {contextId === story.id && (
              <ContextInput storyId={story.id} onAdded={() => { setContextId(null); onRefresh(); }} />
            )}

            {expandedId === story.id && (
              <TaskManager
                story={story}
                missingLayers={missingLayers}
                onAddTask={handleAddTask}
                onDeleteTask={handleDeleteTask}
                onAddDep={handleAddDep}
                onRemoveDep={handleRemoveDep}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}

function PipelineStage({
  task,
  storyId,
  showArrow,
  blocked,
  onStatusChange,
  onSetPRUrl,
  onRefreshPR,
}: {
  task: StoryTask;
  storyId: string;
  showArrow: boolean;
  blocked: boolean;
  onStatusChange: (status: TaskStatus) => void;
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

  const prStatus = getPRStatusLabel(task);
  const ciInfo = task.pr_check_status ? CI_STATUS_DISPLAY[task.pr_check_status] : null;

  return (
    <div className={`pipeline-stage ${blocked ? 'pipeline-blocked' : ''}`}>
      <div className="pipeline-connector">
        {showArrow && <span className="pipeline-arrow">→</span>}
      </div>
      <div
        className="pipeline-card"
        style={{ borderTopColor: TASK_STATUS_COLORS[task.status] }}
      >
        <div className="pipeline-layer-name">{task.name}</div>
        <span
          className="pipeline-status"
          style={{ backgroundColor: TASK_STATUS_COLORS[task.status] }}
        >
          {TASK_STATUS_LABELS[task.status]}
        </span>

        <span className={`pipeline-pr-state ${prStatus.className}`} style={{ color: prStatus.color }}>
          {prStatus.label}
        </span>

        {task.pr_url ? (
          <div className="pipeline-pr-info">
            <div className="pipeline-pr-header">
              <a
                href={task.pr_url}
                target="_blank"
                rel="noopener noreferrer"
                className="pipeline-pr-link"
              >
                PR #{task.pr_number || '?'}
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
            {task.pr_title && (
              <div className="pipeline-pr-title" title={task.pr_title}>
                {task.pr_title.length > 30 ? task.pr_title.slice(0, 30) + '…' : task.pr_title}
              </div>
            )}
            <div className="pipeline-pr-meta">
              {task.pr_review_decision && (
                <span
                  className="pipeline-pr-review"
                  style={{ color: REVIEW_COLORS[task.pr_review_decision] || 'var(--text-muted)' }}
                >
                  {REVIEW_LABELS[task.pr_review_decision] || task.pr_review_decision}
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
          value={task.status}
          onChange={(e) => onStatusChange(e.target.value as TaskStatus)}
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

function TaskManager({
  story,
  missingLayers,
  onAddTask,
  onDeleteTask,
  onAddDep,
  onRemoveDep,
}: {
  story: Story;
  missingLayers: string[];
  onAddTask: (storyId: string, taskName: string) => void;
  onDeleteTask: (storyId: string, taskId: string) => void;
  onAddDep: (storyId: string, taskId: string, dependsOnId: string) => void;
  onRemoveDep: (storyId: string, taskId: string, depId: string) => void;
}) {
  const [customName, setCustomName] = useState('');

  const handleAddCustom = (e: React.FormEvent) => {
    e.preventDefault();
    if (!customName.trim()) return;
    onAddTask(story.id, customName.trim());
    setCustomName('');
  };

  return (
    <div className="layer-manager">
      <div className="layer-manager-header">タスク管理</div>

      {missingLayers.length > 0 && (
        <div className="layer-quick-add">
          {missingLayers.map((name) => (
            <button
              key={name}
              className="layer-add-btn"
              onClick={() => onAddTask(story.id, name)}
            >
              + {name}
            </button>
          ))}
        </div>
      )}

      <form className="layer-custom-add" onSubmit={handleAddCustom}>
        <input
          type="text"
          placeholder="タスク名..."
          value={customName}
          onChange={(e) => setCustomName(e.target.value)}
        />
        <button type="submit" disabled={!customName.trim()}>
          追加
        </button>
      </form>

      {story.tasks.length > 0 && (
        <div className="layer-list-manage">
          {story.tasks.map((t) => {
            const depTasks = story.tasks.filter((other) => t.depends_on.includes(other.id));
            const availableForDep = story.tasks.filter(
              (other) => other.id !== t.id && !t.depends_on.includes(other.id),
            );
            return (
              <div key={t.id} className="layer-manage-item">
                <div className="layer-manage-main">
                  <span>{t.name}</span>
                  {t.pr_url && (
                    <span className="layer-manage-pr">
                      PR #{t.pr_number || '?'} ({t.pr_state || '未取得'})
                    </span>
                  )}
                  <button
                    className="btn-icon btn-danger"
                    onClick={() => onDeleteTask(story.id, t.id)}
                  >
                    ✕
                  </button>
                </div>
                <div className="layer-manage-deps">
                  {depTasks.map((dep) => (
                    <span key={dep.id} className="dep-badge">
                      {dep.name}
                      <button
                        className="dep-remove"
                        onClick={() => onRemoveDep(story.id, t.id, dep.id)}
                        title="依存を解除"
                      >
                        ×
                      </button>
                    </span>
                  ))}
                  {availableForDep.length > 0 && (
                    <select
                      className="dep-add-select"
                      value=""
                      onChange={(e) => {
                        if (e.target.value) onAddDep(story.id, t.id, e.target.value);
                      }}
                    >
                      <option value="">+ 依存を追加...</option>
                      {availableForDep.map((other) => (
                        <option key={other.id} value={other.id}>
                          {other.name}
                        </option>
                      ))}
                    </select>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
