import { useState, useEffect } from 'react';
import { Plus, Edit2, Trash2, Save, X, Bot, Settings, Database, MessageSquare, ChevronDown } from 'lucide-react';
import { agentApi, knowledgeBaseApi, modelApi, mcpApi } from '@/services';
import type { AgentPresetItem, AgentConfig, KnowledgeBase, Model, MCPRegistry } from '@/types';
import ModelSelectorModal from '@/components/ModelSelectorModal';

export default function AgentBuilder() {
  const [presets, setPresets] = useState<AgentPresetItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [showForm, setShowForm] = useState(false);
  const [editingPresetId, setEditingPresetId] = useState<string>('');
  const [showModelSelector, setShowModelSelector] = useState(false);

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
  const [mcpServices, setMcpServices] = useState<MCPRegistry[]>([]);
  const [selectedMcpTools, setSelectedMcpTools] = useState<Record<string, string[]>>({});

  const USER_ID = 'user_001'; // TODO: Get from auth context

  useEffect(() => {
    fetchPresets();
    fetchKBList();
    fetchModels();
    fetchMcpServices();
  }, []);

  const fetchPresets = async () => {
    try {
      setLoading(true);
      const response = await agentApi.list({ user_id: USER_ID, page: 1, page_size: 100 });
      setPresets(response.list || []);
    } catch (error) {
      console.error('Failed to fetch agent presets:', error);
      alert('获取Agent列表失败');
    } finally {
      setLoading(false);
    }
  };

  const fetchKBList = async () => {
    try {
      const response = await knowledgeBaseApi.list();
      setKbList(response.list || []);
    } catch (error) {
      console.error('Failed to fetch knowledge bases:', error);
    }
  };

  const fetchModels = async () => {
    try {
      const response = await modelApi.list();
      const allModels = response.models || [];

      // 包含 LLM 和多模态模型
      const llmAndMultimodalModels = allModels.filter(m => m.type === 'llm' || m.type === 'multimodal');
      const rerankModels = allModels.filter(m => m.type === 'rerank' || m.type === 'reranker');

      setModels(llmAndMultimodalModels);
      setRerankModels(rerankModels);

      // Set default model if available
      if (llmAndMultimodalModels.length > 0 && !config.model_id) {
        setConfig(prev => ({ ...prev, model_id: llmAndMultimodalModels[0].model_id }));
      }
    } catch (error) {
      console.error('Failed to fetch models:', error);
    }
  };

  const fetchMcpServices = async () => {
    try {
      const response = await mcpApi.list({ status: 1 }); // Only active services
      setMcpServices(response.list || []);
    } catch (error) {
      console.error('Failed to fetch MCP services:', error);
    }
  };

  const handleCreate = () => {
    setShowForm(true);
    setEditingPresetId('');
    resetForm();
  };

  const handleEdit = async (presetId: string) => {
    try {
      const preset = await agentApi.get(presetId);
      setEditingPresetId(presetId);
      setPresetName(preset.preset_name);
      setDescription(preset.description);
      setIsPublic(preset.is_public);
      setConfig(preset.config);

      // Set selected MCP tools if any
      if (preset.config.mcp_service_tools) {
        setSelectedMcpTools(preset.config.mcp_service_tools);
      }

      setShowForm(true);
    } catch (error) {
      console.error('Failed to fetch preset:', error);
      alert('获取Agent详情失败');
    }
  };

  const handleDelete = async (presetId: string) => {
    if (!confirm('确定要删除这个Agent预设吗？')) return;

    try {
      await agentApi.delete(presetId, USER_ID);
      alert('删除成功');
      fetchPresets();
    } catch (error) {
      console.error('Failed to delete preset:', error);
      alert('删除失败');
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!presetName.trim() || !config.model_id) {
      alert('请填写预设名称并选择模型');
      return;
    }

    // 如果是编辑模式，提示用户会清除历史对话
    if (editingPresetId) {
      const confirmed = confirm(
        '修改配置后，将会清除该 Agent 的历史对话记录。\n\n确定要继续吗？'
      );
      if (!confirmed) {
        return;
      }
    }

    try {
      setLoading(true);

      const configData: AgentConfig = {
        ...config,
        mcp_service_tools: config.use_mcp ? selectedMcpTools : undefined,
      };

      if (editingPresetId) {
        await agentApi.update(editingPresetId, {
          user_id: USER_ID,
          preset_name: presetName,
          description,
          config: configData,
          is_public: isPublic,
        });

        // 更新成功后，删除该 Agent 的所有对话记录
        try {
          await deleteAgentConversations(editingPresetId);
        } catch (deleteError) {
          console.error('Failed to delete conversations:', deleteError);
          // 删除对话失败不阻断流程，只是警告
        }

        alert('更新成功');
      } else {
        await agentApi.create({
          user_id: USER_ID,
          preset_name: presetName,
          description,
          config: configData,
          is_public: isPublic,
        });
        alert('创建成功');
      }

      setShowForm(false);
      fetchPresets();
    } catch (error) {
      console.error('Failed to save preset:', error);
      alert('保存失败');
    } finally {
      setLoading(false);
    }
  };

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
    setSelectedMcpTools({});
  };

  const toggleMcpTool = (serviceName: string, toolName: string) => {
    setSelectedMcpTools(prev => {
      const serviceTools = prev[serviceName] || [];
      const newServiceTools = serviceTools.includes(toolName)
        ? serviceTools.filter(t => t !== toolName)
        : [...serviceTools, toolName];

      if (newServiceTools.length === 0) {
        const { [serviceName]: _, ...rest} = prev;
        return rest;
      }

      return { ...prev, [serviceName]: newServiceTools };
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
    // TODO: 实现删除该 Agent 的所有对话
    // 这里可以调用批量删除 API，根据 agent_preset_id 筛选
    console.log('Deleting conversations for preset:', presetId);
    // 暂时不实现，因为后端可能需要添加按 preset_id 删除的接口
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

              {/* Retriever Configuration */}
              <div className="border-t pt-6">
                <h3 className="text-lg font-medium mb-4 flex items-center gap-2">
                  <Database className="w-5 h-5" />
                  知识检索配置
                </h3>

                <div className="space-y-4">
                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="enable_retriever"
                      checked={config.enable_retriever || false}
                      onChange={(e) => setConfig(prev => ({ ...prev, enable_retriever: e.target.checked }))}
                      className="w-4 h-4 text-blue-500 rounded focus:ring-2 focus:ring-blue-500"
                    />
                    <label htmlFor="enable_retriever" className="text-sm text-gray-700">
                      启用知识检索
                    </label>
                  </div>

                  {config.enable_retriever && (
                    <>
                      <div>
                        <label className="block text-sm font-medium text-gray-700 mb-2">
                          知识库
                        </label>
                        <select
                          value={config.knowledge_id || ''}
                          onChange={(e) => setConfig(prev => ({ ...prev, knowledge_id: e.target.value }))}
                          className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                        >
                          <option value="">选择知识库</option>
                          {kbList.map((kb) => (
                            <option key={kb.id} value={kb.id}>
                              {kb.name}
                            </option>
                          ))}
                        </select>
                      </div>

                      <div className="grid grid-cols-2 gap-4">
                        <div>
                          <label className="block text-sm font-medium text-gray-700 mb-2">
                            检索模式
                          </label>
                          <select
                            value={config.retrieve_mode || 'rerank'}
                            onChange={(e) => setConfig(prev => ({ ...prev, retrieve_mode: e.target.value as any }))}
                            className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                          >
                            <option value="milvus">Milvus</option>
                            <option value="rerank">Rerank</option>
                            <option value="rrf">RRF</option>
                          </select>
                        </div>

                        <div>
                          <label className="block text-sm font-medium text-gray-700 mb-2">
                            Top K
                          </label>
                          <input
                            type="number"
                            value={config.top_k || 5}
                            onChange={(e) => setConfig(prev => ({ ...prev, top_k: parseInt(e.target.value) }))}
                            className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                            min={1}
                            max={20}
                          />
                        </div>
                      </div>

                      <div>
                        <label className="block text-sm font-medium text-gray-700 mb-2">
                          相似度阈值: {config.score || 0.3}
                        </label>
                        <input
                          type="range"
                          value={config.score || 0.3}
                          onChange={(e) => setConfig(prev => ({ ...prev, score: parseFloat(e.target.value) }))}
                          className="w-full"
                          min={0}
                          max={1}
                          step={0.1}
                        />
                      </div>

                      <div>
                        <label className="block text-sm font-medium text-gray-700 mb-2">
                          Rerank 模型
                        </label>
                        <select
                          value={config.rerank_model_id || ''}
                          onChange={(e) => setConfig(prev => ({ ...prev, rerank_model_id: e.target.value }))}
                          className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                        >
                          <option value="">选择模型</option>
                          {rerankModels.map((model) => (
                            <option key={model.model_id} value={model.model_id}>
                              {model.name}
                            </option>
                          ))}
                        </select>
                        <p className="mt-1 text-xs text-gray-500">
                          Embedding 模型将自动使用知识库绑定的模型
                        </p>
                      </div>
                    </>
                  )}
                </div>
              </div>

              {/* MCP Configuration */}
              <div className="border-t pt-6">
                <h3 className="text-lg font-medium mb-4 flex items-center gap-2">
                  <MessageSquare className="w-5 h-5" />
                  MCP 工具配置
                </h3>

                <div className="space-y-4">
                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      id="use_mcp"
                      checked={config.use_mcp || false}
                      onChange={(e) => setConfig(prev => ({ ...prev, use_mcp: e.target.checked }))}
                      className="w-4 h-4 text-blue-500 rounded focus:ring-2 focus:ring-blue-500"
                    />
                    <label htmlFor="use_mcp" className="text-sm text-gray-700">
                      启用 MCP 工具
                    </label>
                  </div>

                  {config.use_mcp && mcpServices.length > 0 && (
                    <div className="space-y-4">
                      {mcpServices.map((service) => (
                        <div key={service.id} className="border rounded-lg p-4">
                          <h4 className="font-medium mb-2">{service.name}</h4>
                          <p className="text-sm text-gray-600 mb-3">{service.description}</p>
                          {service.tools && service.tools.length > 0 && (
                            <div className="space-y-2">
                              <p className="text-sm font-medium text-gray-700">可用工具：</p>
                              <div className="flex flex-wrap gap-2">
                                {service.tools.map((tool) => (
                                  <label
                                    key={tool.name}
                                    className="flex items-center gap-2 px-3 py-1 border rounded-full cursor-pointer hover:bg-gray-50"
                                  >
                                    <input
                                      type="checkbox"
                                      checked={selectedMcpTools[service.name]?.includes(tool.name) || false}
                                      onChange={() => toggleMcpTool(service.name, tool.name)}
                                      className="w-3 h-3 text-blue-500 rounded"
                                    />
                                    <span className="text-sm">{tool.name}</span>
                                  </label>
                                ))}
                              </div>
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  )}

                  {config.use_mcp && mcpServices.length === 0 && (
                    <p className="text-sm text-gray-500">暂无可用的 MCP 服务</p>
                  )}
                </div>
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
    </div>
  );
}