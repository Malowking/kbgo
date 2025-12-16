import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import Layout from '@/components/Layout';
import KnowledgeBase from '@/pages/KnowledgeBase';
import Documents from '@/pages/Documents';
import Chat from '@/pages/Chat';
import Models from '@/pages/Models';
import MCP from '@/pages/MCP';

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Layout />}>
          <Route index element={<Navigate to="/knowledge-base" replace />} />
          <Route path="knowledge-base" element={<KnowledgeBase />} />
          <Route path="documents" element={<Documents />} />
          <Route path="chat" element={<Chat />} />
          <Route path="models" element={<Models />} />
          <Route path="mcp" element={<MCP />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;