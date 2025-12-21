import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { Toaster } from 'react-hot-toast';
import Layout from '@/components/Layout';
import KnowledgeBase from '@/pages/KnowledgeBase';
import KnowledgeBaseDetail from '@/pages/KnowledgeBase/Detail';
import DocumentDetail from '@/pages/DocumentDetail';
import Documents from '@/pages/Documents';
import Chat from '@/pages/Chat';
import Models from '@/pages/Models';
import MCP from '@/pages/MCP';
import AgentBuilder from '@/pages/AgentBuilder';
import AgentChat from '@/pages/AgentChat';

function App() {
  return (
    <BrowserRouter>
      {/* Toast 通知容器 */}
      <Toaster
        position="top-right"
        toastOptions={{
          duration: 3000,
          style: {
            background: '#fff',
            color: '#333',
            boxShadow: '0 4px 12px rgba(0, 0, 0, 0.15)',
            borderRadius: '8px',
            padding: '12px 16px',
          },
          success: {
            iconTheme: {
              primary: '#10b981',
              secondary: '#fff',
            },
          },
          error: {
            iconTheme: {
              primary: '#ef4444',
              secondary: '#fff',
            },
          },
        }}
      />

      <Routes>
        {/* Full Screen Pages without Layout */}
        <Route path="/agent-chat" element={<AgentChat />} />
        <Route path="/chat" element={<Chat />} />

        {/* Other routes with Layout */}
        <Route path="/" element={<Layout />}>
          <Route index element={<Navigate to="/knowledge-base" replace />} />
          <Route path="knowledge-base" element={<KnowledgeBase />} />
          <Route path="knowledge-base/:id" element={<KnowledgeBaseDetail />} />
          <Route path="kb/:kbId/document/:docId" element={<DocumentDetail />} />
          <Route path="documents" element={<Documents />} />
          <Route path="models" element={<Models />} />
          <Route path="mcp" element={<MCP />} />
          <Route path="agent-builder" element={<AgentBuilder />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;