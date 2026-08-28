import { useState, useEffect, useCallback } from 'react';
import type { Story, StoryStatus, LayerChain, TaskStatus, StoryTask, SnoozeDuration, ContextEntry } from '../api/types';
import { api } from '../api/client';

const SNOOZE_OPTIONS: { label: string; value: SnoozeDuration }[] = [
  { label: '30分', value: '30m' },
  { label: '1時間', value: '1h' },
  { label: '3時間', value: '3h' },
  { label: '明日まで（09:00）', value: 'tomorrow' },
  { label: '来週まで（月 09:00）', value: 'next_week' },
];

function formatPRUpdatedAt(iso: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime()) || d.getFullYear() < 2000) return '';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function formatPRUpdatedAtFull(iso: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime()) || d.getFullYear() < 2000) return '';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function BranchCopyChip({ branch, compact = false }: { branch: string; compact?: boolean }) {
  const [copied, setCopied] = useState(false);
  if (!branch) return null;
  const handleCopy = async (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    try {
      await navigator.clipboard.writeText(branch);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch (err) {
      console.error('failed to copy branch name:', err);
    }
  };
  return (
    <button
      type="button"
      className={`branch-copy-chip ${compact ? 'compact' : ''} ${copied ? 'copied' : ''}`}
      onClick={handleCopy}
      title={copied ? 'コピーしました' : `ブランチ名をコピー: ${branch}`}
    >
      <span className="branch-copy-chip-name">{branch}</span>
      <span className="branch-copy-chip-icon">{copied ? '✓' : '⧉'}</span>
    </button>
  );
}

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

export function extractRepoName(link: string): string | null {
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
  released: 'リリース済',
};

const TASK_STATUS_COLORS: Record<TaskStatus, string> = {
  pending: '#6b7280',
  in_progress: '#3b82f6',
  review: '#f59e0b',
  merged: '#8b5cf6',
  released: '#7c3aed',
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
    if (t.pr_released) {
      return { label: 'リリース済み', color: '#8b5cf6', className: 'pipeline-pr-state-merged pipeline-pr-state-released' };
    }
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
  collapseSignal?: number;
  expandSignal?: number;
  hiddenRepos?: Set<string>;
  onHiddenReposChange?: (next: Set<string>) => void;
}

const URL_RE = /https?:\/\/[^\s]+/g;

function renderDescription(text: string) {
  const parts: React.ReactNode[] = [];
  let last = 0;
  let match: RegExpExecArray | null;
  URL_RE.lastIndex = 0;
  while ((match = URL_RE.exec(text)) !== null) {
    if (match.index > last) parts.push(text.slice(last, match.index));
    const url = match[0];
    parts.push(
      <a key={match.index} href={url} target="_blank" rel="noopener noreferrer" className="description-link">
        {url}
      </a>
    );
    last = match.index + url.length;
  }
  if (last < text.length) parts.push(text.slice(last));
  return parts;
}

export function StoryPipeline({ stories, onRefresh, collapseSignal, expandSignal, hiddenRepos: hiddenReposProp, onHiddenReposChange }: Props) {
  const [chains, setChains] = useState<LayerChain[]>([]);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [contextId, setContextId] = useState<string | null>(null);
  const [snoozeMenuId, setSnoozeMenuId] = useState<string | null>(null);
  const [collapsedIds, setCollapsedIds] = useState<Set<string>>(new Set());
  const [editingDescriptionId, setEditingDescriptionId] = useState<string | null>(null);
  const [descriptionDraft, setDescriptionDraft] = useState('');
  const [contextEntriesMap, setContextEntriesMap] = useState<Record<string, ContextEntry[]>>({});
  const [sortOrder, setSortOrder] = useState<'desc' | 'asc'>(() => {
    const saved = typeof window !== 'undefined' ? window.localStorage.getItem('story_sort_order') : null;
    return saved === 'asc' ? 'asc' : 'desc';
  });
  const handleToggleSortOrder = () => {
    setSortOrder((prev) => {
      const next = prev === 'desc' ? 'asc' : 'desc';
      try { window.localStorage.setItem('story_sort_order', next); } catch { /* ignore */ }
      return next;
    });
  };
  const [unapprovedOnly, setUnapprovedOnly] = useState<boolean>(() => {
    try { return window.localStorage.getItem('story_unapproved_only') === '1'; } catch { return false; }
  });
  const handleToggleUnapprovedOnly = () => {
    setUnapprovedOnly((prev) => {
      const next = !prev;
      try { window.localStorage.setItem('story_unapproved_only', next ? '1' : '0'); } catch { /* ignore */ }
      return next;
    });
  };
  const hiddenRepos = hiddenReposProp ?? new Set<string>();

  const updateHiddenRepos = (next: Set<string>) => {
    onHiddenReposChange?.(next);
  };

  useEffect(() => {
    api.getLayerChains().then(setChains).catch(console.error);
  }, []);

  useEffect(() => {
    if (collapseSignal === undefined) return;
    setCollapsedIds(new Set(stories.map((s) => s.id)));
  // collapseSignal の変化だけを監視する意図的な依存リスト
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [collapseSignal]);

  useEffect(() => {
    if (expandSignal === undefined) return;
    setCollapsedIds(new Set());
  }, [expandSignal]);

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

  const handleDescriptionSave = async (story: Story, newDesc: string) => {
    try {
      await api.updateStory(story.id, { ...story, description: newDesc });
      onRefresh();
    } catch (e) {
      console.error('Failed to update description:', e);
    }
  };

  const fetchContextEntries = async (storyId: string) => {
    try {
      const entries = await api.listContext(storyId);
      setContextEntriesMap((prev) => ({ ...prev, [storyId]: entries }));
    } catch (e) {
      console.error('Failed to fetch context entries:', e);
    }
  };

  useEffect(() => {
    stories.forEach((story) => fetchContextEntries(story.id));
  // stories の変化だけを監視する意図的な依存リスト
   
  }, [stories]);

  const handleContextPanelToggle = (storyId: string) => {
    setContextId((prev) => (prev === storyId ? null : storyId));
  };

  const handleDeleteContextEntry = async (storyId: string, entryId: string) => {
    try {
      await api.deleteContext(storyId, entryId);
      setContextEntriesMap((prev) => ({
        ...prev,
        [storyId]: (prev[storyId] || []).filter((e) => e.id !== entryId),
      }));
    } catch (e) {
      console.error('Failed to delete context entry:', e);
    }
  };

  const handleUpdateContextEntry = async (storyId: string, entryId: string, content: string, entryType: string) => {
    try {
      const updated = await api.updateContext(storyId, entryId, content, entryType);
      setContextEntriesMap((prev) => ({
        ...prev,
        [storyId]: (prev[storyId] || []).map((e) => (e.id === entryId ? updated : e)),
      }));
    } catch (e) {
      console.error('Failed to update context entry:', e);
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
    if (!confirm('このタスクを削除しますか？')) return;
    try {
      await api.deleteTask(storyId, taskId);
      onRefresh();
    } catch (e) {
      console.error('Failed to delete task:', e);
    }
  };

  const handleMoveTask = async (story: Story, taskId: string, direction: 'up' | 'down') => {
    const tasks = [...story.tasks];
    const idx = tasks.findIndex((t) => t.id === taskId);
    if (idx < 0) return;
    const swapIdx = direction === 'up' ? idx - 1 : idx + 1;
    if (swapIdx < 0 || swapIdx >= tasks.length) return;

    const aOrder = tasks[idx].sort_order;
    const bOrder = tasks[swapIdx].sort_order;
    const newAOrder = aOrder === bOrder ? (direction === 'up' ? aOrder - 1 : aOrder + 1) : bOrder;
    const newBOrder = aOrder === bOrder ? aOrder : aOrder;

    try {
      await Promise.all([
        api.updateTask(story.id, tasks[idx].id, { ...tasks[idx], sort_order: newAOrder }),
        api.updateTask(story.id, tasks[swapIdx].id, { ...tasks[swapIdx], sort_order: newBOrder }),
      ]);
      onRefresh();
    } catch (e) {
      console.error('Failed to move task:', e);
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
      return dep ? dep.status !== 'merged' && dep.status !== 'released' : false;
    });
  };

  const repoNames = [...new Set(
    stories.flatMap((s) =>
      s.tasks.map((t) => extractRepoName(t.pr_url)).filter((r): r is string => r !== null)
    )
  )].sort();

  const toggleHiddenRepo = (repo: string) => {
    const next = new Set(hiddenRepos);
    if (next.has(repo)) {
      next.delete(repo);
    } else {
      next.add(repo);
    }
    updateHiddenRepos(next);
  };

  const isUnapprovedSingleTask = (s: Story): boolean => {
    if (s.tasks.length !== 1) return false;
    const t = s.tasks[0];
    if (!t.pr_url || t.pr_state !== 'OPEN' || t.pr_is_draft) return false;
    return t.pr_review_decision !== 'APPROVED';
  };

  const filteredStories = stories.filter((s) => {
    if (unapprovedOnly && !isUnapprovedSingleTask(s)) return false;
    if (hiddenRepos.size === 0) return true;
    const taskRepos = s.tasks.map((t) => extractRepoName(t.pr_url)).filter((r): r is string => r !== null);
    if (taskRepos.length === 0) return true;
    return taskRepos.some((r) => !hiddenRepos.has(r));
  });

  const now = Date.now();
  const isStorySnoozed = (s: Story): boolean =>
    s.snoozed_until ? new Date(s.snoozed_until).getTime() > now : false;
  const sortedStories = [...filteredStories].sort((a, b) => {
    const sa = isStorySnoozed(a) ? 1 : 0;
    const sb = isStorySnoozed(b) ? 1 : 0;
    if (sa !== sb) return sa - sb;
    const pa = a.priority === 0 ? 999 : a.priority;
    const pb = b.priority === 0 ? 999 : b.priority;
    if (pa !== pb) return pa - pb;
    const am = a.tasks.length > 1 ? 0 : 1;
    const bm = b.tasks.length > 1 ? 0 : 1;
    if (am !== bm) return am - bm;
    if (am === 0) return 0;
    const au = a.tasks[0]?.pr_updated_at ? new Date(a.tasks[0].pr_updated_at).getTime() : 0;
    const bu = b.tasks[0]?.pr_updated_at ? new Date(b.tasks[0].pr_updated_at).getTime() : 0;
    return sortOrder === 'desc' ? bu - au : au - bu;
  });

  if (stories.length === 0) {
    return <div className="empty-state">ストーリーがありません。新しいストーリーを追加してみましょう。</div>;
  }

  return (
    <div className="story-list">
      <div className="story-list-toolbar">
        <button
          type="button"
          className="story-sort-toggle"
          onClick={handleToggleSortOrder}
          title="同じ優先順位内のシングルタスクをPR最終更新日でソート"
        >
          {sortOrder === 'desc' ? '新しい順 ▼' : '古い順 ▲'}
        </button>
        <button
          type="button"
          className={`story-sort-toggle story-unapproved-toggle ${unapprovedOnly ? 'active' : ''}`}
          onClick={handleToggleUnapprovedOnly}
          title="レビュー可能（ドラフトでない）でまだApproveされていないオープンPRのシングルタスクだけを表示（マルチタスクは非表示）"
        >
          {unapprovedOnly ? '未Approveのみ ✓' : '未Approveのみ'}
        </button>
        {repoNames.length > 0 && (
          <div className="repo-filter-chips">
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
            <button
              className="repo-filter-clear"
              onClick={() => updateHiddenRepos(new Set(repoNames))}
            >
              すべて非表示
            </button>
            <button
              className="repo-filter-clear"
              onClick={() => updateHiddenRepos(new Set())}
            >
              すべて表示
            </button>
          </div>
        )}
      </div>
      {sortedStories.map((story) => {
        const chainLayers = getChainLayers(story.layer_chain);
        const existingNames = new Set(story.tasks.map((t) => t.name));
        const missingLayers = chainLayers.filter((name) => !existingNames.has(name));
        const isSingleTask = story.tasks.length === 0;
        const isSnoozed = story.snoozed_until ? new Date(story.snoozed_until) > new Date() : false;
        const snoozeLabel = formatSnoozedUntil(story.snoozed_until);
        const isCollapsed = collapsedIds.has(story.id);

        if (isCollapsed) {
          return (
            <div key={story.id} className={`story-card story-card-collapsed ${isSnoozed ? 'story-snoozed' : ''}`}>
              <div className="story-collapsed-row">
                <span
                  className="story-status-badge"
                  style={{ backgroundColor: STORY_STATUS_COLORS[story.status] }}
                >
                  {STORY_STATUS_LABELS[story.status]}
                </span>
                <span className="story-title story-collapsed-title">{story.title}</span>
                {story.tasks.map((t) => {
                  const repoLabel = t.pr_url ? extractRepoName(t.pr_url) : null;
                  const prNum = t.pr_number || null;
                  const chipLabel = repoLabel && prNum ? `${repoLabel} #${prNum}` : t.name;
                  return (
                    <span key={t.id} className="story-collapsed-task">
                      {t.pr_url ? (
                        <a href={t.pr_url} target="_blank" rel="noopener noreferrer" className="story-collapsed-task-link">
                          {chipLabel}
                        </a>
                      ) : (
                        t.name
                      )}
                    </span>
                  );
                })}
                <button
                  className="btn-icon story-collapsed-expand"
                  onClick={() => setCollapsedIds((prev) => { const next = new Set(prev); next.delete(story.id); return next; })}
                  title="展開"
                >
                  ▼
                </button>
              </div>
            </div>
          );
        }

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
                  onClick={() => handleContextPanelToggle(story.id)}
                  title="メモ"
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
                  className="btn-icon"
                  onClick={() => setCollapsedIds((prev) => { const next = new Set(prev); next.add(story.id); return next; })}
                  title="畳む"
                >
                  ▲
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

            {editingDescriptionId === story.id ? (
              <div className="description-edit">
                <textarea
                  className="description-textarea"
                  value={descriptionDraft}
                  onChange={(e) => setDescriptionDraft(e.target.value)}
                  rows={3}
                  autoFocus
                  placeholder="説明（URLはリンクになります）"
                />
                <div className="description-edit-actions">
                  <button
                    className="btn-sm btn-primary"
                    onClick={async () => {
                      await handleDescriptionSave(story, descriptionDraft);
                      setEditingDescriptionId(null);
                    }}
                  >
                    保存
                  </button>
                  <button
                    className="btn-sm"
                    onClick={() => setEditingDescriptionId(null)}
                  >
                    キャンセル
                  </button>
                </div>
              </div>
            ) : (
              <div
                className={`story-description-wrap ${!story.description ? 'story-description-empty' : ''}`}
                onClick={() => { setEditingDescriptionId(story.id); setDescriptionDraft(story.description); }}
                title="クリックして編集"
              >
                {story.description
                  ? <p className="story-description">{renderDescription(story.description)}</p>
                  : <span className="story-description-placeholder">説明を追加...</span>
                }
              </div>
            )}

            {story.tags.length > 0 && (
              <div className="task-tags">
                {story.tags.map((tag, i) => (
                  <span key={i} className="task-tag">{tag}</span>
                ))}
              </div>
            )}

            {story.tasks.length === 1 ? (
              <SingleTaskPRRow
                task={story.tasks[0]}
                onSetPRUrl={(prUrl) => handleSetPRUrl(story, story.tasks[0].id, prUrl)}
                onRefreshPR={() => handleRefreshPR(story.id, story.tasks[0].id)}
              />
            ) : story.tasks.length > 1 ? (
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
            ) : null}

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

            <ContextEntryList
              entries={contextEntriesMap[story.id] || []}
              onDelete={(entryId) => handleDeleteContextEntry(story.id, entryId)}
              onUpdate={(entryId, content, entryType) => handleUpdateContextEntry(story.id, entryId, content, entryType)}
            />

            {contextId === story.id && (
              <ContextInput
                storyId={story.id}
                onAdded={() => fetchContextEntries(story.id)}
              />
            )}

            {expandedId === story.id && (
              <TaskManager
                story={story}
                missingLayers={missingLayers}
                onAddTask={handleAddTask}
                onDeleteTask={handleDeleteTask}
                onMoveTask={(taskId, dir) => handleMoveTask(story, taskId, dir)}
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

function SingleTaskPRRow({
  task,
  onSetPRUrl,
  onRefreshPR,
}: {
  task: StoryTask;
  onSetPRUrl: (prUrl: string) => void;
  onRefreshPR: () => void;
}) {
  const [refreshing, setRefreshing] = useState(false);
  const [showPRInput, setShowPRInput] = useState(false);
  const [prUrlInput, setPrUrlInput] = useState('');
  const prStatus = getPRStatusLabel(task);
  const ciInfo = task.pr_check_status ? CI_STATUS_DISPLAY[task.pr_check_status] : null;

  const handleRefresh = async () => {
    setRefreshing(true);
    try {
      await onRefreshPR();
    } finally {
      setRefreshing(false);
    }
  };

  const handlePRSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!prUrlInput.trim()) return;
    onSetPRUrl(prUrlInput.trim());
    setPrUrlInput('');
    setShowPRInput(false);
  };

  const handleEditClick = () => {
    setPrUrlInput(task.pr_url || '');
    setShowPRInput(true);
  };

  const handleCancelEdit = () => {
    setPrUrlInput('');
    setShowPRInput(false);
  };

  const prLinkLabel = (() => {
    if (!task.pr_url) return null;
    const repo = extractRepoName(task.pr_url);
    const num = task.pr_number;
    if (repo && num) return `🔗 ${repo} #${num}`;
    return '🔗 PR';
  })();
  const updatedLabel = formatPRUpdatedAt(task.pr_updated_at);
  const updatedFull = formatPRUpdatedAtFull(task.pr_updated_at);
  const isDone = task.pr_state === 'MERGED';

  if (showPRInput) {
    return (
      <div className="story-single-pr-row">
        <form className="pipeline-pr-form" onSubmit={handlePRSubmit}>
          <input
            type="url"
            placeholder="PR URL..."
            value={prUrlInput}
            onChange={(e) => setPrUrlInput(e.target.value)}
            autoFocus
          />
          <button type="submit" disabled={!prUrlInput.trim()}>OK</button>
          <button type="button" onClick={handleCancelEdit} className="btn-cancel">✕</button>
        </form>
      </div>
    );
  }

  return (
    <div className="story-single-pr-row">
      {task.pr_url && prLinkLabel ? (
        <a href={task.pr_url} target="_blank" rel="noopener noreferrer" className="task-link task-link-pr">
          {prLinkLabel}
        </a>
      ) : null}
      <BranchCopyChip branch={task.pr_head_ref_name} compact={isDone} />
      {!isDone && (
        <span className="pipeline-pr-state" style={{ color: prStatus.color }}>
          {prStatus.label}
        </span>
      )}
      {!isDone && task.pr_review_decision && REVIEW_LABELS[task.pr_review_decision] && (
        <span className="pipeline-review-badge" style={{ color: REVIEW_COLORS[task.pr_review_decision] }}>
          {REVIEW_LABELS[task.pr_review_decision]}
        </span>
      )}
      {!isDone && ciInfo && (
        <span style={{ color: ciInfo.color, fontSize: '0.75rem' }}>
          {ciInfo.icon} {ciInfo.label}
        </span>
      )}
      {updatedLabel && (
        <span className="story-single-pr-updated" title={updatedFull}>
          更新: {updatedLabel}
        </span>
      )}
      {task.pr_url ? (
        <>
          <button
            className="btn-icon btn-refresh"
            onClick={handleRefresh}
            disabled={refreshing}
            title="PR状態を更新"
          >
            {refreshing ? '⟳' : '↻'}
          </button>
          <button
            className="btn-icon btn-edit-pr"
            onClick={handleEditClick}
            title="PR URLを編集"
          >
            ✎
          </button>
        </>
      ) : (
        <button
          className="btn-add-pr"
          onClick={handleEditClick}
          title="PRを紐付け"
        >
          + PR
        </button>
      )}
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

  const handleEditClick = () => {
    setPrUrlInput(task.pr_url || '');
    setShowPRInput(true);
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
  const isDone = task.pr_state === 'MERGED';

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

        {!isDone && (
          <span className={`pipeline-pr-state ${prStatus.className}`} style={{ color: prStatus.color }}>
            {prStatus.label}
          </span>
        )}

        {task.pr_url && !showPRInput ? (
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
              <button
                className="btn-icon btn-edit-pr"
                onClick={handleEditClick}
                title="PR URLを編集"
              >
                ✎
              </button>
            </div>
            {task.pr_head_ref_name && (
              <div className="pipeline-pr-branch">
                <BranchCopyChip branch={task.pr_head_ref_name} compact={isDone} />
              </div>
            )}
            <div className="pipeline-pr-meta">
              <div>
                {!isDone && task.pr_review_decision && (
                  <span
                    className="pipeline-pr-review"
                    style={{color: REVIEW_COLORS[task.pr_review_decision] || 'var(--text-muted)'}}
                  >
                    {REVIEW_LABELS[task.pr_review_decision] || task.pr_review_decision}
                  </span>
                )}
                {!isDone && ciInfo && (
                  <span className="pipeline-ci-status" style={{color: ciInfo.color}}>
                    {ciInfo.icon} {ciInfo.label}
                  </span>
                )}
              </div>
              <div>
                {formatPRUpdatedAt(task.pr_updated_at) && (
                  <span className="pipeline-pr-updated" title={formatPRUpdatedAtFull(task.pr_updated_at)}>
                    PR最終変更: {formatPRUpdatedAt(task.pr_updated_at)}
                  </span>
                )}
              </div>
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
          <option value="released">リリース済</option>
        </select>
      </div>
    </div>
  );
}

const ENTRY_TYPE_LABELS: Record<string, string> = {
  note: 'メモ',
  progress: '進捗',
  blocker: 'ブロッカー',
};

const ENTRY_TYPE_COLORS: Record<string, string> = {
  note: '#6b7280',
  progress: '#3b82f6',
  blocker: '#ef4444',
};

function ContextEntryList({
  entries,
  onDelete,
  onUpdate,
}: {
  entries: ContextEntry[];
  onDelete: (entryId: string) => void;
  onUpdate: (entryId: string, content: string, entryType: string) => void;
}) {
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editContent, setEditContent] = useState('');
  const [editType, setEditType] = useState('note');

  if (entries.length === 0) return null;

  const startEdit = (entry: ContextEntry) => {
    setEditingId(entry.id);
    setEditContent(entry.content);
    setEditType(entry.entry_type);
  };

  const cancelEdit = () => setEditingId(null);

  const saveEdit = async () => {
    if (!editingId) return;
    await onUpdate(editingId, editContent, editType);
    setEditingId(null);
  };

  return (
    <div className="context-entries">
      {entries.map((entry) =>
        editingId === entry.id ? (
          <div key={entry.id} className="context-entry context-entry-editing">
            <select
              className="context-entry-type-select"
              value={editType}
              onChange={(e) => setEditType(e.target.value)}
            >
              <option value="note">メモ</option>
              <option value="progress">進捗</option>
              <option value="blocker">ブロッカー</option>
            </select>
            <input
              className="context-entry-edit-input"
              type="text"
              value={editContent}
              onChange={(e) => setEditContent(e.target.value)}
              autoFocus
              onKeyDown={(e) => {
                if (e.key === 'Enter') saveEdit();
                if (e.key === 'Escape') cancelEdit();
              }}
            />
            <button className="btn-sm btn-primary" onClick={saveEdit}>保存</button>
            <button className="btn-sm" onClick={cancelEdit}>キャンセル</button>
          </div>
        ) : (
          <div key={entry.id} className="context-entry">
            <span
              className="context-entry-type"
              style={{ color: ENTRY_TYPE_COLORS[entry.entry_type] || '#6b7280' }}
            >
              {ENTRY_TYPE_LABELS[entry.entry_type] || entry.entry_type}
            </span>
            <span className="context-entry-content">{entry.content}</span>
            <span className="context-entry-date">
              {new Date(entry.created_at).toLocaleString('ja-JP', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
            </span>
            <button className="btn-icon btn-sm" onClick={() => startEdit(entry)} title="編集">✏️</button>
            <button className="btn-icon btn-sm btn-danger" onClick={() => onDelete(entry.id)} title="削除">✕</button>
          </div>
        )
      )}
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
    } catch (err) {
      console.error('Failed to add context:', err);
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
  onMoveTask,
  onAddDep,
  onRemoveDep,
}: {
  story: Story;
  missingLayers: string[];
  onAddTask: (storyId: string, taskName: string) => void;
  onDeleteTask: (storyId: string, taskId: string) => void;
  onMoveTask: (taskId: string, direction: 'up' | 'down') => void;
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
          {story.tasks.map((t, idx) => {
            const depTasks = story.tasks.filter((other) => t.depends_on.includes(other.id));
            const availableForDep = story.tasks.filter(
              (other) => other.id !== t.id && !t.depends_on.includes(other.id),
            );
            return (
              <div key={t.id} className="layer-manage-item">
                <div className="layer-manage-main">
                  <div className="layer-manage-order-btns">
                    <button
                      className="btn-icon"
                      onClick={() => onMoveTask(t.id, 'up')}
                      disabled={idx === 0}
                      title="上に移動"
                    >
                      ▲
                    </button>
                    <button
                      className="btn-icon"
                      onClick={() => onMoveTask(t.id, 'down')}
                      disabled={idx === story.tasks.length - 1}
                      title="下に移動"
                    >
                      ▼
                    </button>
                  </div>
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
