import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, FileText, TestTube, Settings } from 'lucide-react';
import { knowledgeBaseApi } from '@/services';
import { useAppStore } from '@/store';
import type { KnowledgeBase } from '@/types';
import DocumentsTab from './tabs/DocumentsTab';
import RetrieverTestTab from './tabs/RetrieverTestTab';
import SettingsTab from './tabs/SettingsTab';
import { showError } from '@/lib/toast';

type TabType = 'documents' | 'retriever' | 'settings';

export default function KnowledgeBaseDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { currentKB, setCurrentKB } = useAppStore();
  const [activeTab, setActiveTab] = useState<TabType>('documents');
  const [kb, setKb] = useState<KnowledgeBase | null>(currentKB);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!currentKB || currentKB.id !== id) {
      fetchKB();
    } else {
      setKb(currentKB);
    }
  }, [id, currentKB]);

  const fetchKB = async () => {
    if (!id) return;

    try {
      setLoading(true);
      const response = await knowledgeBaseApi.get(id);
      setKb(response);
      setCurrentKB(response);
    } catch (error) {
      console.error('Failed to fetch knowledge base:', error);
      showError('获取知识库信息失败');
      navigate('/knowledge-base');
    } finally {
      setLoading(false);
    }
  };

  const tabs = [
    { key: 'documents' as TabType, label: '文档管理', icon: FileText },
    { key: 'retriever' as TabType, label: '召回测试', icon: TestTube },
    { key: 'settings' as TabType, label: '设置', icon: Settings },
  ];

  if (loading || !kb) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="inline-block w-8 h-8 border-4 border-primary-600 border-t-transparent rounded-full animate-spin"></div>
        <p className="ml-4 text-gray-600">加载中...</p>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="border-b bg-white px-6 py-4">
        <button
          onClick={() => navigate('/knowledge-base')}
          className="flex items-center gap-2 text-sm text-gray-600 hover:text-gray-900 mb-3"
        >
          <ArrowLeft className="w-4 h-4" />
          返回知识库列表
        </button>

        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">{kb.name}</h1>
            <p className="mt-1 text-sm text-gray-600">{kb.description}</p>
          </div>

          <div className="flex items-center gap-2">
            {kb.category && (
              <span className="px-3 py-1 text-sm font-medium text-primary-700 bg-primary-50 rounded">
                {kb.category}
              </span>
            )}
            <span
              className={`px-3 py-1 text-sm rounded ${
                kb.status === 1
                  ? 'bg-green-100 text-green-700'
                  : 'bg-gray-100 text-gray-700'
              }`}
            >
              {kb.status === 1 ? '启用' : '禁用'}
            </span>
          </div>
        </div>

        {/* Tabs */}
        <div className="flex gap-1 mt-4">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            return (
              <button
                key={tab.key}
                onClick={() => setActiveTab(tab.key)}
                className={`flex items-center gap-2 px-4 py-2 rounded-t-lg transition-colors ${
                  activeTab === tab.key
                    ? 'bg-white border-t-2 border-primary-600 text-primary-700'
                    : 'text-gray-600 hover:bg-gray-50'
                }`}
              >
                <Icon className="w-4 h-4" />
                {tab.label}
              </button>
            );
          })}
        </div>
      </div>

      {/* Tab Content */}
      <div className="flex-1 overflow-auto bg-gray-50">
        {activeTab === 'documents' && <DocumentsTab kbId={kb.id} />}
        {activeTab === 'retriever' && <RetrieverTestTab kbId={kb.id} />}
        {activeTab === 'settings' && <SettingsTab kb={kb} onUpdate={fetchKB} />}
      </div>
    </div>
  );
}