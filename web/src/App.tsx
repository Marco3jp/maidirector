import { useState, useEffect, useCallback } from 'react';
import type { Task } from './api/types';
import { api } from './api/client';
import { useWebSocket } from './hooks/useWebSocket';
import { TaskList } from './components/TaskList';
import { TaskForm } from './components/TaskForm';
import { SuggestPanel } from './components/SuggestPanel';
import { ChatPanel } from './components/ChatPanel';
import './App.css';

type FilterStatus = '' | 'todo' | 'in_progress' | 'blocked' | 'done';

function App() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [filterStatus, setFilterStatus] = useState<FilterStatus>('');
  const [activeTab, setActiveTab] = useState<'suggest' | 'chat'>('suggest');

  const loadTasks = useCallback(async () => {
    try {
      const data = await api.listTasks(filterStatus || undefined);
      setTasks(data);
    } catch (e) {
      console.error('Failed to load tasks:', e);
    }
  }, [filterStatus]);

  useEffect(() => {
    loadTasks();
  }, [loadTasks]);

  useWebSocket((msg) => {
    if (msg.type === 'task_created' || msg.type === 'task_updated' || msg.type === 'task_deleted') {
      loadTasks();
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

      <main className="app-main">
        <section className="panel panel-tasks">
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

          <TaskForm onCreated={loadTasks} />

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
