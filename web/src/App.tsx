import { useState, useEffect, useCallback, useRef } from 'react';
import type { Story, StoryStatus } from './api/types';
import { api } from './api/client';
import { useWebSocket } from './hooks/useWebSocket';
import { StoryForm } from './components/StoryForm';
import { StoryPipeline, extractRepoName } from './components/StoryPipeline';
import './App.css';

type FilterStatus = '' | StoryStatus;

function App() {
  const [stories, setStories] = useState<Story[]>([]);
  const [filterStatus, setFilterStatus] = useState<FilterStatus>('');
  const [scanning, setScanning] = useState(false);
  const [scanResult, setScanResult] = useState<string | null>(null);
  const [lastScanAt, setLastScanAt] = useState<Date | null>(null);
  const [collapseSignal, setCollapseSignal] = useState(0);
  const [expandSignal, setExpandSignal] = useState(0);
  const [hiddenRepos, setHiddenRepos] = useState<Set<string>>(() => {
    try {
      const saved = localStorage.getItem('maidirector:hiddenRepos');
      return saved ? new Set<string>(JSON.parse(saved)) : new Set<string>();
    } catch {
      return new Set<string>();
    }
  });
  const suppressWsRefreshRef = useRef(false);

  const updateHiddenRepos = useCallback((next: Set<string>) => {
    setHiddenRepos(next);
    try {
      localStorage.setItem('maidirector:hiddenRepos', JSON.stringify([...next]));
    } catch { /* ignore */ }
  }, []);
  const snoozeTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const loadStories = useCallback(async () => {
    try {
      const data = await api.listStories(filterStatus || undefined);
      setStories(data);
    } catch (e) {
      console.error('Failed to load stories:', e);
    }
  }, [filterStatus]);

  useEffect(() => {
    loadStories();
  }, [loadStories]);

  useEffect(() => {
    if (snoozeTimerRef.current) {
      clearTimeout(snoozeTimerRef.current);
      snoozeTimerRef.current = null;
    }

    const now = Date.now();
    const futureTimes = stories
      .map((s) => s.snoozed_until ? new Date(s.snoozed_until).getTime() : null)
      .filter((t): t is number => t !== null && t > now);

    if (futureTimes.length > 0) {
      const nearest = Math.min(...futureTimes);
      const delay = nearest - now + 500;
      snoozeTimerRef.current = setTimeout(() => {
        loadStories();
      }, delay);
    }

    return () => {
      if (snoozeTimerRef.current) clearTimeout(snoozeTimerRef.current);
    };
  }, [stories, loadStories]);

  const handleStoryCreated = useCallback((newStory: Story) => {
    setStories((prev) => {
      if (prev.some((s) => s.id === newStory.id)) return prev;
      return [newStory, ...prev];
    });
    suppressWsRefreshRef.current = true;
    setTimeout(() => { suppressWsRefreshRef.current = false; }, 500);
  }, []);

  const handleScanAndRefresh = useCallback(async () => {
    setScanning(true);
    setScanResult(null);
    try {
      const [result] = await Promise.all([
        api.scanOrgPRs(),
        api.refreshAllPRs(),
      ]);
      setLastScanAt(new Date());
      if (result.created > 0) {
        setScanResult(`${result.created}件追加`);
        setTimeout(() => setScanResult(null), 5000);
      }
      loadStories();
    } catch (e) {
      const msg = e instanceof Error ? e.message : 'スキャンに失敗しました';
      setScanResult(msg);
      setTimeout(() => setScanResult(null), 5000);
    } finally {
      setScanning(false);
    }
  }, [loadStories]);

  useWebSocket((msg) => {
    if (msg.type === 'story_created' && suppressWsRefreshRef.current) {
      return;
    }
    if (msg.type === 'story_created' || msg.type === 'story_updated' || msg.type === 'story_deleted') {
      loadStories();
    }
  });

  const activeStories = stories.filter((s) => s.status === 'active');
  const completedStories = stories.filter((s) => s.status === 'completed');
  const archivedStories = stories.filter((s) => s.status === 'archived');

  const visibleActiveCount = activeStories.filter((s) => {
    if (hiddenRepos.size === 0) return true;
    const taskRepos = s.tasks.map((t) => extractRepoName(t.pr_url)).filter((r): r is string => r !== null);
    if (taskRepos.length === 0) return true;
    return taskRepos.some((r) => !hiddenRepos.has(r));
  }).length;

  return (
    <div className="app">
      <main className="app-main">
        <section className="panel panel-tasks">
          <div className="panel-header">
            <div className="panel-header-actions">
              <div className="filter-bar">
                {(['' , 'active', 'completed', 'archived'] as FilterStatus[]).map((s) => (
                  <button
                    key={s}
                    className={`filter-btn ${filterStatus === s ? 'active' : ''}`}
                    onClick={() => setFilterStatus(s)}
                  >
                    {s === '' ? 'すべて' : s === 'active' ? <>アクティブ <span className="filter-btn-count">{visibleActiveCount}</span></> : s === 'completed' ? '完了' : 'アーカイブ'}
                  </button>
                ))}
                <span className="filter-bar-divider" />
                <button
                  className="filter-btn filter-btn-icon"
                  onClick={() => setCollapseSignal((n) => n + 1)}
                  title="すべて畳む"
                >
                  一括▲
                </button>
                <button
                  className="filter-btn filter-btn-icon"
                  onClick={() => setExpandSignal((n) => n + 1)}
                  title="すべて展開"
                >
                  一括▼
                </button>
                <span className="filter-bar-divider" />
                <button
                  className="filter-btn"
                  onClick={handleScanAndRefresh}
                  disabled={scanning}
                >
                  {scanning ? '↻ 更新中...' : '↻ PR更新'}
                </button>
                {lastScanAt && (
                  <span className="last-scan-time">
                    {lastScanAt.toLocaleTimeString('ja-JP', { hour: '2-digit', minute: '2-digit' })}
                  </span>
                )}
              </div>
            </div>
          </div>

          {scanResult && (
            <div className="scan-result-banner">
              {scanResult}
            </div>
          )}

          <StoryForm onCreated={handleStoryCreated} />

          <StoryPipeline stories={filterStatus === '' ? activeStories : stories} onRefresh={loadStories} collapseSignal={collapseSignal} expandSignal={expandSignal} hiddenRepos={hiddenRepos} onHiddenReposChange={updateHiddenRepos} />

          {filterStatus === '' && completedStories.length > 0 && (
            <details className="done-section">
              <summary>完了済み ({completedStories.length})</summary>
              <StoryPipeline stories={completedStories} onRefresh={loadStories} />
            </details>
          )}

          {filterStatus === '' && archivedStories.length > 0 && (
            <details className="done-section">
              <summary>アーカイブ ({archivedStories.length})</summary>
              <StoryPipeline stories={archivedStories} onRefresh={loadStories} />
            </details>
          )}

          {stories.length === 0 && (
            <div className="empty-state">ストーリーがありません</div>
          )}
        </section>
      </main>
    </div>
  );
}

export default App;
