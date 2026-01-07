import { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Save, Loader2 } from 'lucide-react';
import { agentApi, knowledgeBaseApi, modelApi, mcpApi, nl2sqlApi } from '@/services';
import type { AgentPreset, AgentConfig, KnowledgeBase, Model, MCPRegistry } from '@/types';
import ToolConfigurationPanel from '@/components/ToolConfigurationPanel';
import { logger } from '@/lib/logger';
import { showError, showSuccess } from '@/lib/toast';
import { USER } from '@/config/constants';
import { getRerankModels } from '@/lib/model-utils';

export default function AgentTools() {
  const { presetId } = useParams<{ presetId: string }>();
  const navigate = useNavigate();

  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [preset, setPreset] = useState<AgentPreset | null>(null);
  const [config, setConfig] = useState<AgentConfig>({
    model_id: '',
    enable_retriever: false,
    top_k: 5,
    score: 0.3,
    retrieve_mode: 'rerank',
    use_mcp: false,
  });

  // Available options
  const [kbList, setKbList] = useState<KnowledgeBase[]>([]);
  const [rerankModels, setRerankModels] = useState<Model[]>([]);
  const [mcpServices, setMcpServices] = useState<MCPRegistry[]>([]);
  const [nl2sqlDatasources, setNl2sqlDatasources] = useState<any[]>([]);

  useEffect(() => {
    if (presetId) {
      fetchPreset();
      fetchKBList();
      fetchModels();
      fetchMcpServices();
      fetchNL2SQLDatasources();
    }
  }, [presetId]);

  const fetchPreset = useCallback(async () => {
    if (!presetId) return;

    try {
      setLoading(true);
      const response = await agentApi.get(presetId);
      setPreset(response);

      // 设置配置
      if (response.config) {
        setConfig(response.config);
      }
    } catch (error) {
      logger.error('Failed to fetch agent preset:', error);
      showError('获取 Agent 配置失败');
    } finally {
      setLoading(false);
    }
  }, [presetId]);

  const fetchKBList = useCallback(async () => {
    try {
      const response = await knowledgeBaseApi.list();
      setKbList(response.list || []);
    } catch (error) {
      logger.error('Failed to fetch knowledge bases:', error);
    }
  }, []);

  const fetchModels = useCallback(async () => {
    try {
      const response = await modelApi.list();
      const rerankModelsList = getRerankModels(response.models || [], true);
      setRerankModels(rerankModelsList);
    } catch (error) {
      logger.error('Failed to fetch models:', error);
    }
  }, []);

  const fetchMcpServices = useCallback(async () => {
    try {
      const response = await mcpApi.list({ status: 1 });
      setMcpServices(response.list || []);
    } catch (error) {
      logger.error('Failed to fetch MCP services:', error);
    }
  }, []);

  const fetchNL2SQLDatasources = useCallback(async () => {
    try {
      const response = await nl2sqlApi.listDatasources();
      setNl2sqlDatasources(response.list || []);
    } catch (error) {
      logger.error('Failed to fetch NL2SQL datasources:', error);
    }
  }, []);

  const handleSave = async () => {
    if (!presetId || !preset) return;

    try {
      setSaving(true);
      await agentApi.update(presetId, {
        preset_name: preset.preset_name,
        description: preset.description,
        is_public: preset.is_public,
        config: config,
      });
      showSuccess('工具配置保存成功');
    } catch (error) {
      logger.error('Failed to save tool configuration:', error);
      showError('保存失败');
    } finally {
      setSaving(false);
    }
  };

  if (loading) {
    return (
      <div className="h-screen flex items-center justify-center">
        <Loader2 className="w-8 h-8 animate-spin text-blue-500" />
      </div>
    );
  }

  if (!preset) {
    return (
      <div className="h-screen flex items-center justify-center">
        <div className="text-center">
          <p className="text-gray-500 mb-4">Agent 不存在</p>
          <button
            onClick={() => navigate('/agent-builder')}
            className="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600"
          >
            返回 Agent 列表
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <div className="bg-white border-b sticky top-0 z-10">
        <div className="max-w-7xl mx-auto px-6 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-4">
              <button
                onClick={() => navigate('/agent-builder')}
                className="flex items-center gap-2 text-gray-600 hover:text-gray-900 transition-colors"
              >
                <ArrowLeft className="w-5 h-5" />
                返回
              </button>
              <div>
                <h1 className="text-2xl font-bold text-gray-900">工具配置</h1>
                <p className="text-sm text-gray-500 mt-1">
                  为 <span className="font-medium text-gray-700">{preset.preset_name}</span> 配置可用工具
                </p>
              </div>
            </div>
            <button
              onClick={handleSave}
              disabled={saving}
              className="flex items-center gap-2 px-6 py-2.5 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {saving ? (
                <>
                  <Loader2 className="w-5 h-5 animate-spin" />
                  保存中...
                </>
              ) : (
                <>
                  <Save className="w-5 h-5" />
                  保存配置
                </>
              )}
            </button>
          </div>
        </div>
      </div>

      {/* Content */}
      <div className="max-w-7xl mx-auto px-6 py-8">
        <div className="bg-white rounded-lg shadow-sm p-6">
          <ToolConfigurationPanel
            config={config}
            onConfigChange={setConfig}
            kbList={kbList}
            rerankModels={rerankModels}
            mcpServices={mcpServices}
            nl2sqlDatasources={nl2sqlDatasources}
          />
        </div>
      </div>
    </div>
  );
}