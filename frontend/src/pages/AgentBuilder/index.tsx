import { useState, useEffect, useCallback } from 'react';
import { Plus, Edit2, Trash2, Save, X, Bot, Settings, ChevronDown } from 'lucide-react';
import { agentApi, knowledgeBaseApi, modelApi, mcpApi, conversationApi, nl2sqlApi } from '@/services';
import type { AgentPresetItem, AgentConfig, KnowledgeBase, Model, MCPRegistry } from '@/types';
import ModelSelectorModal from '@/components/ModelSelectorModal';
import ToolConfigurationPanel from '@/components/ToolConfigurationPanel';
import { logger } from '@/lib/logger';
import { showError, showWarning, showSuccess } from '@/lib/toast';
import { USER } from '@/config/constants';
import { getLLMModels, getRerankModels } from '@/lib/model-utils';
import { useConfirm } from '@/hooks/useConfirm';

export default function AgentBuilder() {
  const [presets, setPresets] = useState<AgentPresetItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [editingPresetId, setEditingPresetId] = useState<string>('');
  const [showModelSelector, setShowModelSelector] = useState(false);
  const { confirm, ConfirmDialog } = useConfirm();

  // Form state
  const [presetName, setPresetName] = useState('');
  const [description, setDescription] = useState('');
  const [isPublic, setIsPublic] = useState(false);

  // Config state
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
  const [models, setModels] = useState<Model[]>([]);
  const [rerankModels, setRerankModels] = useState<Model[]>([]);
  const [_, setEmbeddingModels] = useState<Model[]>([]);
  const [mcpServices, setMcpServices] = useState<MCPRegistry[]>([]);
  const [nl2sqlDatasources, setNl2sqlDatasources] = useState<any[]>([]);

  useEffect(() => {
    fetchPresets();
    fetchKBList();
    fetchModels();
    fetchMcpServices();
    fetchNL2SQLDatasources();
  }, []);

  const fetchPresets = useCallback(async () => {
    try {
      setLoading(true);
      const response = await agentApi.list({ user_id: USER.ID, page: 1, page_size: 100 });
      setPresets(response.list || []);
    } catch (error) {
      logger.error('Failed to fetch agent presets:', error);
      showError('获取Agent列表失败');
    } finally {
      setLoading(false);
    }
  }, []);

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

      // 使用工具函数获取 LLM 和多模态模型
      const llmAndMultimodalModels = getLLMModels(response.models || [], true);
      const rerankModelsList = getRerankModels(response.models || [], true);

      // 获取 embedding 模型
      const embeddingModelsList = (response.models || []).filter(
        m => m.type === 'embedding' && m.enabled
      );

      setModels(llmAndMultimodalModels);
      setRerankModels(rerankModelsList);
      setEmbeddingModels(embeddingModelsList);

      // Set default model if available
      if (llmAndMultimodalModels.length > 0 && !config.model_id) {
        setConfig(prev => ({ ...prev, model_id: llmAndMultimodalModels[0].model_id }));
      }
    } catch (error) {
      logger.error('Failed to fetch models:', error);
    }
  }, [config.model_id]);

  const fetchMcpServices = useCallback(async () => {
    try {
      const response = await mcpApi.list({ status: 1 }); // Only active services
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

  const handleCreate = () => {
    setShowForm(true);
    setEditingPresetId('');
    resetForm();
  };

  const handleEdit = useCallback(async (presetId: string) => {
    try {
      const preset = await agentApi.get(presetId);
      setEditingPresetId(presetId);
      setPresetName(preset.preset_name);
      setDescription(preset.description);
      setIsPublic(preset.is_public);

      // 从 tools 数组恢复配置到 config
      const restoredConfig = { ...preset.config };

      if (preset.tools && preset.tools.length > 0) {
        for (const tool of preset.tools) {
          if (!tool.enabled) continue;

          switch (tool.type) {
            case 'local_tools':
              // 恢复优先级
              if (tool.priority !== undefined) {
                restoredConfig.knowledge_retrieval_priority = tool.priority;
                restoredConfig.nl2sql_priority = tool.priority;
              }

              // 恢复知识库检索配置
              if (tool.config.knowledge_retrieval) {
                const kr = tool.config.knowledge_retrieval;
                restoredConfig.enable_retriever = true;
                restoredConfig.knowledge_id = kr.knowledge_id;
                restoredConfig.embedding_model_id = kr.embedding_model_id;
                restoredConfig.rerank_model_id = kr.rerank_model_id;
                restoredConfig.top_k = kr.top_k || 5;
                restoredConfig.score = kr.score || 0.3;
                restoredConfig.retrieve_mode = kr.retrieve_mode || 'rerank';
                restoredConfig.rerank_weight = kr.rerank_weight;
              }

              // 恢复 NL2SQL 配置
              if (tool.config.nl2sql) {
                const nl2sql = tool.config.nl2sql;
                restoredConfig.enable_nl2sql = true;
                restoredConfig.nl2sql_datasource_id = nl2sql.datasource;
                restoredConfig.nl2sql_embedding_model_id = nl2sql.embedding_model_id;
              }
              break;

            case 'mcp':
              // 恢复 MCP 优先级
              if (tool.priority !== undefined) {
                restoredConfig.mcp_priority = tool.priority;
              }

              // 恢复 MCP 配置
              if (tool.config.service_tools) {
                restoredConfig.use_mcp = true;
                restoredConfig.mcp_service_tools = tool.config.service_tools;
              }
              break;
          }
        }
      }

      setConfig(restoredConfig);
      setShowForm(true);
    } catch (error) {
      logger.error('Failed to fetch preset:', error);
      showError('获取Agent详情失败');
    }
  }, []);

  const handleDelete = useCallback(async (presetId: string) => {
    const confirmed = await confirm({
      message: '确定要删除这个Agent预设吗？',
      type: 'danger'
    });
    if (!confirmed) return;

    try {
      await agentApi.delete(presetId, USER.ID);
      showSuccess('删除成功');
      fetchPresets();
    } catch (error) {
      logger.error('Failed to delete preset:', error);
      showError('删除失败');
    }
  }, [fetchPresets, confirm]);

  const handleSubmit = useCallback(async (e: React.FormEvent) => {
    e.preventDefault();

    if (!presetName.trim() || !config.model_id) {
      showWarning('请填写预设名称并选择模型');
      return;
    }

    // 如果是编辑模式，提示用户会清除历史对话
    if (editingPresetId) {
      const confirmed = await confirm({
        message: '修改配置后，将会清除该 Agent 的历史对话记录。\n\n确定要继续吗？',
        type: 'warning'
      });
      if (!confirmed) {
        return;
      }
    }

    try {
      setLoading(true);

      // 构造 tools 数组
      const tools = [];

      // 1. 构造 local_tools 配置（包含知识库检索、NL2SQL等本地工具）
      const localToolsConfig: Record<string, any> = {};

      // 知识库检索工具
      if (config.enable_retriever && config.knowledge_id) {
        localToolsConfig.knowledge_retrieval = {
          knowledge_id: config.knowledge_id,
          embedding_model_id: config.embedding_model_id,
          rerank_model_id: config.rerank_model_id,
          top_k: config.top_k,
          score: config.score,
          retrieve_mode: config.retrieve_mode,
          rerank_weight: config.rerank_weight,
        };
      }

      // NL2SQL 工具
      if (config.enable_nl2sql && config.nl2sql_datasource_id) {
        localToolsConfig.nl2sql = {
          datasource: config.nl2sql_datasource_id,
          embedding_model_id: config.nl2sql_embedding_model_id,
        };
      }

      // 如果有本地工具配置，添加到 tools 数组
      if (Object.keys(localToolsConfig).length > 0) {
        const localToolPriority = config.knowledge_retrieval_priority || config.nl2sql_priority;
        tools.push({
          type: 'local_tools',
          enabled: true,
          priority: localToolPriority,
          config: localToolsConfig,
        });
      }

      // 2. MCP 工具（独立的工具类型）
      if (config.use_mcp && config.mcp_service_tools && Object.keys(config.mcp_service_tools).length > 0) {
        tools.push({
          type: 'mcp',
          enabled: true,
          priority: config.mcp_priority,
          config: {
            service_tools: config.mcp_service_tools,
          }
        });
      }

      if (editingPresetId) {
        await agentApi.update(editingPresetId, {
          user_id: USER.ID,
          preset_name: presetName,
          description,
          config,
          tools: tools.length > 0 ? tools : undefined,
          is_public: isPublic,
        });

        // 更新成功后，删除该 Agent 的所有对话记录
        try {
          await deleteAgentConversations(editingPresetId);
        } catch (deleteError) {
          logger.error('Failed to delete conversations:', deleteError);
          // 删除对话失败不阻断流程，只是警告
        }

        showSuccess('更新成功');
      } else {
        await agentApi.create({
          user_id: USER.ID,
          preset_name: presetName,
          description,
          config,
          tools: tools.length > 0 ? tools : undefined,
          is_public: isPublic,
        });
        showSuccess('创建成功');
      }

      setShowForm(false);
      fetchPresets();
    } catch (error) {
      logger.error('Failed to save preset:', error);
      showError('保存失败');
    } finally {
      setLoading(false);
    }
  }, [presetName, config, editingPresetId, description, isPublic, fetchPresets, confirm]);

  const handleCancel = () => {
    setShowForm(false);
    resetForm();
  };

  const resetForm = () => {
    setPresetName('');
    setDescription('');
    setIsPublic(false);
    setConfig({
      model_id: models.length > 0 ? models[0].model_id : '',
      enable_retriever: false,
      top_k: 5,
      score: 0.3,
      retrieve_mode: 'rerank',
      use_mcp: false,
    });
  };

  const handleModelSelect = (model: Model) => {
    setConfig(prev => ({ ...prev, model_id: model.model_id }));
  };

  const getSelectedModelName = (): string => {
    const model = models.find(m => m.model_id === config.model_id);
    return model ? model.name : '选择模型';
  };

  // 删除 Agent 的所有对话记录
  const deleteAgentConversations = async (presetId: string) => {
    try {
      logger.info('Deleting conversations for preset:', presetId);

      // 1. 获取该 Agent 的所有对话列表
      const listResponse = await conversationApi.list({
        page: 1,
        page_size: 1000, // 假设最多1000条对话
      });

      // 2. 从元数据中筛选出属于该 Agent 的对话
      const agentConvs = listResponse.conversations.filter((conv: any) => {
        // 检查元数据中是否有 agent_preset_id
        return conv.metadata?.agent_preset_id === presetId ||
               conv.agent_preset_id === presetId; // 兼容直接字段和元数据字段
      });

      if (agentConvs.length === 0) {
        logger.info(`No conversations found for preset ${presetId}`);
        return;
      }

      // 3. 批量删除这些对话
      const convIds = agentConvs.map((conv: any) => conv.conv_id);
      await conversationApi.batchDelete(convIds);

      logger.info(`Successfully deleted ${convIds.length} conversations for preset ${presetId}`);
    } catch (error) {
      logger.error('Failed to delete agent conversations:', error);
      throw error;
    }
  };

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="border-b bg-white px-6 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Bot className="w-6 h-6 text-blue-500" />
            <h1 className="text-2xl font-bold">Agent 构建器</h1>
          </div>
          <button
            onClick={handleCreate}
            className="flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors"
          >
            <Plus className="w-4 h-4" />
            创建 Agent
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto p-6">
        {showForm ? (
          /* Form */
          <form onSubmit={handleSubmit} className="max-w-4xl mx-auto bg-white rounded-lg shadow-sm border p-6">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-xl font-semibold">
                {editingPresetId ? '编辑 Agent' : '创建新 Agent'}
              </h2>
              <button type="button" onClick={handleCancel} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>

            <div className="space-y-6">
              {/* Basic Info */}
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  预设名称 *
                </label>
                <input
                  type="text"
                  value={presetName}
                  onChange={(e) => setPresetName(e.target.value)}
                  className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder="例如：技术支持助手"
                  required
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  描述
                </label>
                <textarea
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                  rows={3}
                  placeholder="描述这个Agent的用途..."
                />
              </div>

              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="is_public"
                  checked={isPublic}
                  onChange={(e) => setIsPublic(e.target.checked)}
                  className="w-4 h-4 text-blue-500 rounded focus:ring-2 focus:ring-blue-500"
                />
                <label htmlFor="is_public" className="text-sm text-gray-700">
                  公开预设（其他用户可以使用）
                </label>
              </div>

              {/* Model Selection */}
              <div className="border-t pt-6">
                <h3 className="text-lg font-medium mb-4 flex items-center gap-2">
                  <Settings className="w-5 h-5" />
                  模型配置
                </h3>

                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      LLM 模型 *
                    </label>
                    <button
                      type="button"
                      onClick={() => setShowModelSelector(true)}
                      className="w-full flex items-center justify-between px-3 py-2 border rounded-lg hover:bg-gray-50 transition-colors text-left"
                    >
                      <span className="text-sm text-gray-900">{getSelectedModelName()}</span>
                      <ChevronDown className="w-4 h-4 text-gray-500" />
                    </button>
                  </div>

                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="jsonformat"
                      checked={config.jsonformat || false}
                      onChange={(e) => setConfig(prev => ({ ...prev, jsonformat: e.target.checked }))}
                      className="w-4 h-4 text-blue-500 rounded focus:ring-2 focus:ring-blue-500"
                    />
                    <label htmlFor="jsonformat" className="text-sm text-gray-700">
                      JSON 格式输出
                    </label>
                  </div>

                  {/* System Prompt */}
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      系统提示词 (System Prompt)
                    </label>
                    <textarea
                      value={config.system_prompt || ''}
                      onChange={(e) => setConfig(prev => ({ ...prev, system_prompt: e.target.value }))}
                      placeholder="输入系统提示词，定义 Agent 的角色、行为和输出格式...&#10;&#10;例如：&#10;你是一个专业的技术支持助手，擅长解答用户的技术问题。请保持友好、专业的态度，用简洁明了的语言回答问题。"
                      className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 min-h-[120px] resize-y font-mono text-sm"
                    />
                    <p className="mt-1 text-xs text-gray-500">
                      系统提示词会在每次对话开始时发送给模型，用于定义 Agent 的行为规则、角色设定、输出格式等
                    </p>
                  </div>
                </div>
              </div>

              {/* Tool Configuration Panel */}
              <div className="border-t pt-6">
                <ToolConfigurationPanel
                  config={config}
                  onConfigChange={setConfig}
                  kbList={kbList}
                  rerankModels={rerankModels}
                  mcpServices={mcpServices}
                  nl2sqlDatasources={nl2sqlDatasources}
                />
              </div>

              {/* Submit Buttons */}
              <div className="flex justify-end gap-3 pt-6 border-t">
                <button
                  type="button"
                  onClick={handleCancel}
                  className="px-4 py-2 border rounded-lg hover:bg-gray-50 transition-colors"
                >
                  取消
                </button>
                <button
                  type="submit"
                  disabled={loading}
                  className="flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors disabled:opacity-50"
                >
                  <Save className="w-4 h-4" />
                  {loading ? '保存中...' : '保存'}
                </button>
              </div>
            </div>
          </form>
        ) : (
          /* List */
          <div className="max-w-6xl mx-auto">
            {loading ? (
              <div className="text-center py-12">
                <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
              </div>
            ) : presets.length === 0 ? (
              <div className="text-center py-12">
                <Bot className="w-16 h-16 text-gray-300 mx-auto mb-4" />
                <p className="text-gray-500 mb-4">还没有创建任何 Agent 预设</p>
                <button
                  onClick={handleCreate}
                  className="inline-flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors"
                >
                  <Plus className="w-4 h-4" />
                  创建第一个 Agent
                </button>
              </div>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {presets.map((preset) => (
                  <div key={preset.preset_id} className="bg-white rounded-lg border shadow-sm hover:shadow-md transition-shadow">
                    <div className="p-4">
                      <div className="flex items-start justify-between mb-3">
                        <h3 className="font-semibold text-lg">{preset.preset_name}</h3>
                        {preset.is_public && (
                          <span className="px-2 py-1 bg-green-100 text-green-700 text-xs rounded">
                            公开
                          </span>
                        )}
                      </div>

                      <p className="text-sm text-gray-600 mb-4 line-clamp-2">
                        {preset.description || '暂无描述'}
                      </p>

                      <div className="flex items-center justify-between text-xs text-gray-500 mb-4">
                        <span>创建于 {new Date(preset.create_time).toLocaleDateString()}</span>
                      </div>

                      <div className="flex gap-2">
                        <button
                          onClick={() => handleEdit(preset.preset_id)}
                          className="flex-1 flex items-center justify-center gap-1 px-3 py-2 border rounded-lg hover:bg-gray-50 transition-colors"
                        >
                          <Edit2 className="w-4 h-4" />
                          编辑
                        </button>
                        <button
                          onClick={() => handleDelete(preset.preset_id)}
                          className="flex-1 flex items-center justify-center gap-1 px-3 py-2 border border-red-200 text-red-600 rounded-lg hover:bg-red-50 transition-colors"
                        >
                          <Trash2 className="w-4 h-4" />
                          删除
                        </button>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Model Selector Modal */}
      {showModelSelector && (
        <ModelSelectorModal
          onClose={() => setShowModelSelector(false)}
          onSelect={handleModelSelect}
          currentModelId={config.model_id}
          modelTypes={['llm', 'multimodal']}
        />
      )}

      {/* Confirm Dialog */}
      <ConfirmDialog />
    </div>
  );
}
