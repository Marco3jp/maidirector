import { useState, useEffect, useCallback, useRef } from 'react';
import type { Task, Story } from './api/types';
import { api } from './api/client';
import { useWebSocket } from './hooks/useWebSocket';
import { TaskList } from './components/TaskList';
import { TaskForm } from './components/TaskForm';
import { StoryForm } from './components/StoryForm';
import { StoryPipeline } from './components/StoryPipeline';
import { SuggestPanel } from './components/SuggestPanel';
import { ChatPanel } from './components/ChatPanel';
import './App.css';

type FilterStatus = '' | 'todo' | 'in_progress' | 'blocked' | 'done';
type MainView = 'tasks' | 'stories';

function App() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [stories, setStories] = useState<Story[]>([]);
  const [filterStatus, setFilterStatus] = useState<FilterStatus>('');
  const [mainView, setMainView] = useState<MainView>('tasks');
  const [activeTab, setActiveTab] = useState<'suggest' | 'chat'>('suggest');
  const suppressWsRefreshRef = useRef(false);

  const loadTasks = useCallback(async () => {
    try {
      const data = await api.listTasks(filterStatus || undefined);
      setTasks(data);
    } catch (e) {
      console.error('Failed to load tasks:', e);
    }
  }, [filterStatus]);

  const loadStories = useCallback(async () => {
    try {
      const data = await api.listStories();
      setStories(data);
    } catch (e) {
      console.error('Failed to load stories:', e);
    }
  }, []);

  useEffect(() => {
    loadTasks();
  }, [loadTasks]);

  useEffect(() => {
    loadStories();
  }, [loadStories]);

  const handleTaskCreated = useCallback((newTask: Task) => {
    if (!filterStatus || filterStatus === newTask.status) {
      setTasks((prev) => {
        if (prev.some((t) => t.id === newTask.id)) return prev;
        return [newTask, ...prev];
      });
    }
    suppressWsRefreshRef.current = true;
    setTimeout(() => { suppressWsRefreshRef.current = false; }, 500);
  }, [filterStatus]);

  const handleStoryCreated = useCallback((newStory: Story) => {
    setStories((prev) => {
      if (prev.some((s) => s.id === newStory.id)) return prev;
      return [newStory, ...prev];
    });
  }, []);

  useWebSocket((msg) => {
    if (msg.type === 'task_created' && suppressWsRefreshRef.current) {
      return;
    }
    if (msg.type === 'task_created' || msg.type === 'task_updated' || msg.type === 'task_deleted') {
      loadTasks();
    }
    if (msg.type === 'story_created' || msg.type === 'story_updated' || msg.type === 'story_deleted') {
      loadStories();
    }
  });

  const activeTasks = tasks.filter((t) => t.status !== 'done');
  const doneTasks = tasks.filter((t) => t.status === 'done');

  return (
    <div className="app">
      <header className="app-header">
        <h1>maidirector</h1>
        <span className="app-subtitle">ローカル開発AIアシスタント</span>
      </header>

      <nav className="nav-tabs">
        <button
          className={`nav-tab ${mainView === 'tasks' ? 'active' : ''}`}
          onClick={() => setMainView('tasks')}
        >
          タスク
        </button>
        <button
          className={`nav-tab ${mainView === 'stories' ? 'active' : ''}`}
          onClick={() => setMainView('stories')}
        >
          ストーリー
        </button>
      </nav>

      <main className="app-main">
        <section className="panel panel-tasks">
          {mainView === 'tasks' ? (
            <>
              <div className="panel-header">
                <h2>タスク</h2>
                <div className="filter-bar">
                  {(['', 'todo', 'in_progress', 'blocked', 'done'] as FilterStatus[]).map((s) => (
                    <button
                      key={s}
                      className={`filter-btn ${filterStatus === s ? 'active' : ''}`}
                      onClick={() => setFilterStatus(s)}
                    >
                      {s === '' ? 'すべて' : s === 'todo' ? 'TODO' : s === 'in_progress' ? '進行中' : s === 'blocked' ? 'ブロック' : '完了'}
                    </button>
                  ))}
                </div>
              </div>

              <TaskForm onCreated={handleTaskCreated} />

              <div className="task-sections">
                {activeTasks.length > 0 && (
                  <TaskList tasks={activeTasks} onRefresh={loadTasks} />
                )}
                {doneTasks.length > 0 && filterStatus === '' && (
                  <details className="done-section">
                    <summary>完了済み ({doneTasks.length})</summary>
                    <TaskList tasks={doneTasks} onRefresh={loadTasks} />
                  </details>
                )}
                {tasks.length === 0 && (
                  <div className="empty-state">タスクがありません</div>
                )}
              </div>
            </>
          ) : (
            <>
              <div className="panel-header">
                <h2>ストーリー</h2>
              </div>

              <StoryForm onCreated={handleStoryCreated} />

              <StoryPipeline stories={stories} onRefresh={loadStories} />
            </>
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
          {activeTab === 'suggest' ? <SuggestPanel /> : <ChatPanel />}
        </section>
      </main>
    </div>
  );
}

export default App;
