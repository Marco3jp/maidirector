import { useState, useEffect, useCallback, useRef } from 'react';
import type { Story, StoryStatus } from './api/types';
import { api } from './api/client';
import { useWebSocket } from './hooks/useWebSocket';
import { StoryForm } from './components/StoryForm';
import { StoryPipeline } from './components/StoryPipeline';
import { SuggestPanel } from './components/SuggestPanel';
import { ChatPanel } from './components/ChatPanel';
import './App.css';

type FilterStatus = '' | StoryStatus;

function App() {
  const [stories, setStories] = useState<Story[]>([]);
  const [filterStatus, setFilterStatus] = useState<FilterStatus>('');
  const [activeTab, setActiveTab] = useState<'suggest' | 'chat'>('suggest');
  const [scanning, setScanning] = useState(false);
  const [scanResult, setScanResult] = useState<string | null>(null);
  const suppressWsRefreshRef = useRef(false);

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

  const handleStoryCreated = useCallback((newStory: Story) => {
    setStories((prev) => {
      if (prev.some((s) => s.id === newStory.id)) return prev;
      return [newStory, ...prev];
    });
    suppressWsRefreshRef.current = true;
    setTimeout(() => { suppressWsRefreshRef.current = false; }, 500);
  }, []);

  const handleScanOrgPRs = useCallback(async () => {
    setScanning(true);
    setScanResult(null);
    try {
      const result = await api.scanOrgPRs();
      setScanResult(`${result.scanned}件スキャン → ${result.created}件作成 / ${result.skipped}件スキップ`);
      if (result.created > 0) {
        loadStories();
      }
      setTimeout(() => setScanResult(null), 5000);
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

  return (
    <div className="app">
      <header className="app-header">
        <h1>maidirector</h1>
        <span className="app-subtitle">ローカル開発AIアシスタント</span>
      </header>

      <main className="app-main">
        <section className="panel panel-tasks">
          <div className="panel-header">
            <h2>ストーリー</h2>
            <div className="panel-header-actions">
              <button
                className="btn-scan-org"
                onClick={handleScanOrgPRs}
                disabled={scanning}
              >
                {scanning ? '🔍 スキャン中...' : '🔍 組織PRスキャン'}
              </button>
              <div className="filter-bar">
                {(['' , 'active', 'completed', 'archived'] as FilterStatus[]).map((s) => (
                  <button
                    key={s}
                    className={`filter-btn ${filterStatus === s ? 'active' : ''}`}
                    onClick={() => setFilterStatus(s)}
                  >
                    {s === '' ? 'すべて' : s === 'active' ? 'アクティブ' : s === 'completed' ? '完了' : 'アーカイブ'}
                  </button>
                ))}
              </div>
            </div>
          </div>

          {scanResult && (
            <div className="scan-result-banner">
              {scanResult}
            </div>
          )}

          <StoryForm onCreated={handleStoryCreated} />

          <StoryPipeline stories={filterStatus === '' ? activeStories : stories} onRefresh={loadStories} />

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

        <section className="panel panel-ai">
          <div className="tab-bar">
            <button
              className={`tab-btn ${activeTab === 'suggest' ? 'active' : ''}`}
              onClick={() => setActiveTab('suggest')}
            >
              次にやること
            </button>
            <button
              className={`tab-btn ${activeTab === 'chat' ? 'active' : ''}`}
              onClick={() => setActiveTab('chat')}
            >
              チャット
            </button>
          </div>
          <div style={{ display: activeTab === 'suggest' ? 'contents' : 'none' }}>
            <SuggestPanel />
          </div>
          <div style={{ display: activeTab === 'chat' ? 'contents' : 'none' }}>
            <ChatPanel />
          </div>
        </section>
      </main>
    </div>
  );
}

export default App;
