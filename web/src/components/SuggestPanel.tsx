import { useState } from 'react';
import { api } from '../api/client';

export function SuggestPanel() {
  const [suggestion, setSuggestion] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSuggest = async () => {
    setLoading(true);
    setSuggestion('');

    try {
      let text = '';
      for await (const chunk of api.suggestStream()) {
        text += chunk;
        setSuggestion(text);
      }
    } catch (e) {
      setSuggestion(`エラー: ${e instanceof Error ? e.message : '不明なエラー'}`);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="suggest-panel">
      <div className="suggest-header">
        <h2>次にやること</h2>
        <button onClick={handleSuggest} disabled={loading} className="btn-suggest">
          {loading ? '考え中...' : '提案してもらう'}
        </button>
      </div>
      {suggestion && (
        <div className="suggest-content">
          <pre>{suggestion}</pre>
        </div>
      )}
    </div>
  );
}
