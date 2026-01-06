import { useState, useEffect } from 'react';
import { Database, Table, MessageSquare, Plus, Minus, ChevronDown, ChevronUp } from 'lucide-react';
import type { AgentConfig, KnowledgeBase, Model, MCPRegistry } from '@/types';

interface ToolConfigurationPanelProps {
  config: AgentConfig;
  onConfigChange: (config: AgentConfig) => void;
  kbList: KnowledgeBase[];
  rerankModels: Model[];
  mcpServices: MCPRegistry[];
  nl2sqlDatasources: any[];
}

interface McpServiceConfig {
  id: string;
  serviceName: string;
  selectedTools: string[];
}

export default function ToolConfigurationPanel({
  config,
  onConfigChange,
  kbList,
  rerankModels,
  mcpServices,
  nl2sqlDatasources,
}: ToolConfigurationPanelProps) {
  // 工具启用状态
  const [enableKnowledgeRetrieval, setEnableKnowledgeRetrieval] = useState(false);
  const [enableNL2SQL, setEnableNL2SQL] = useState(false);
  const [enableMCP, setEnableMCP] = useState(false);

  // 展开/折叠状态
  const [expandedTools, setExpandedTools] = useState<Record<string, boolean>>({
    knowledge: true,
    nl2sql: true,
    mcp: true,
  });

  // MCP 配置
  const [mcpConfigs, setMcpConfigs] = useState<McpServiceConfig[]>([]);

  // 初始化工具启用状态
  useEffect(() => {
    setEnableKnowledgeRetrieval(!!config.knowledge_id);
    setEnableNL2SQL(!!config.nl2sql_datasource_id);
    setEnableMCP(!!config.use_mcp);

    // 初始化 MCP 配置
    if (config.mcp_service_tools) {
      const configs: McpServiceConfig[] = Object.entries(config.mcp_service_tools).map(
        ([serviceName, tools], index) => ({
          id: `${Date.now()}-${index}`,
          serviceName,
          selectedTools: tools as string[],
        })
      );
      setMcpConfigs(configs);
    }
  }, [config.knowledge_id, config.nl2sql_datasource_id, config.use_mcp, config.mcp_service_tools]);

  // 切换工具展开/折叠
  const toggleTool = (toolKey: string) => {
    setExpandedTools((prev) => ({ ...prev, [toolKey]: !prev[toolKey] }));
  };

  // 知识库检索工具配置
  const handleKnowledgeRetrievalToggle = (enabled: boolean) => {
    setEnableKnowledgeRetrieval(enabled);
    if (!enabled) {
      onConfigChange({
        ...config,
        knowledge_id: undefined,
        enable_retriever: false,
      });
    } else {
      onConfigChange({
        ...config,
        enable_retriever: true,
      });
    }
  };

  const handleKnowledgeBaseChange = (knowledgeId: string) => {
    onConfigChange({
      ...config,
      knowledge_id: knowledgeId,
      enable_retriever: !!knowledgeId,
    });
  };

  // NL2SQL 工具配置
  const handleNL2SQLToggle = (enabled: boolean) => {
    setEnableNL2SQL(enabled);
    if (!enabled) {
      onConfigChange({
        ...config,
        nl2sql_datasource_id: undefined,
        enable_nl2sql: false,
      });
    }
  };

  const handleNL2SQLDatasourceChange = (datasourceId: string) => {
    onConfigChange({
      ...config,
      nl2sql_datasource_id: datasourceId,
      enable_nl2sql: !!datasourceId,
    });
  };

  // MCP 工具配置
  const handleMCPToggle = (enabled: boolean) => {
    setEnableMCP(enabled);
    if (!enabled) {
      onConfigChange({
        ...config,
        use_mcp: false,
        mcp_service_tools: undefined,
      });
      setMcpConfigs([]);
    } else {
      onConfigChange({
        ...config,
        use_mcp: true,
      });
    }
  };

  const addMcpConfig = () => {
    setMcpConfigs((prev) => [
      ...prev,
      {
        id: Date.now().toString(),
        serviceName: '',
        selectedTools: [],
      },
    ]);
  };

  const removeMcpConfig = (id: string) => {
    setMcpConfigs((prev) => prev.filter((c) => c.id !== id));
  };

  const updateMcpServiceName = (id: string, serviceName: string) => {
    setMcpConfigs((prev) =>
      prev.map((c) => (c.id === id ? { ...c, serviceName, selectedTools: [] } : c))
    );
  };

  const updateMcpTools = (id: string, tools: string[]) => {
    setMcpConfigs((prev) =>
      prev.map((c) => (c.id === id ? { ...c, selectedTools: tools } : c))
    );
  };

  // 同步 MCP 配置到 config
  useEffect(() => {
    if (!enableMCP) return;

    const newSelectedTools: Record<string, string[]> = {};
    mcpConfigs.forEach((mcpConfig) => {
      if (mcpConfig.serviceName && mcpConfig.selectedTools.length > 0) {
        newSelectedTools[mcpConfig.serviceName] = mcpConfig.selectedTools;
      }
    });

    onConfigChange({
      ...config,
      mcp_service_tools: Object.keys(newSelectedTools).length > 0 ? newSelectedTools : undefined,
    });
  }, [mcpConfigs, enableMCP]);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 mb-4">
        <h3 className="text-lg font-medium">工具配置</h3>
        <span className="text-xs text-gray-500">选择 Agent 可以使用的工具</span>
      </div>

      {/* 知识库检索工具 */}
      <div className="border rounded-lg overflow-hidden">
        <div
          className="flex items-center justify-between p-4 bg-gray-50 cursor-pointer hover:bg-gray-100 transition-colors"
          onClick={() => toggleTool('knowledge')}
        >
          <div className="flex items-center gap-3">
            <Database className="w-5 h-5 text-blue-500" />
            <div>
              <h4 className="font-medium">知识库检索</h4>
              <p className="text-xs text-gray-500">从知识库中检索相关文档</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <label className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
              <input
                type="checkbox"
                checked={enableKnowledgeRetrieval}
                onChange={(e) => handleKnowledgeRetrievalToggle(e.target.checked)}
                className="w-4 h-4 text-blue-500 rounded focus:ring-2 focus:ring-blue-500"
              />
              <span className="text-sm">启用</span>
            </label>
            {expandedTools.knowledge ? (
              <ChevronUp className="w-5 h-5 text-gray-400" />
            ) : (
              <ChevronDown className="w-5 h-5 text-gray-400" />
            )}
          </div>
        </div>

        {expandedTools.knowledge && enableKnowledgeRetrieval && (
          <div className="p-4 space-y-4 border-t">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">知识库 *</label>
              <select
                value={config.knowledge_id || ''}
                onChange={(e) => handleKnowledgeBaseChange(e.target.value)}
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

            {config.knowledge_id && (
              <>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">检索模式</label>
                    <select
                      value={config.retrieve_mode || 'rerank'}
                      onChange={(e) =>
                        onConfigChange({ ...config, retrieve_mode: e.target.value as any })
                      }
                      className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                    >
                      <option value="simple">普通检索</option>
                      <option value="rerank">Rerank</option>
                      <option value="rrf">RRF</option>
                    </select>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">Top K</label>
                    <input
                      type="number"
                      value={config.top_k || 5}
                      onChange={(e) =>
                        onConfigChange({ ...config, top_k: parseInt(e.target.value) })
                      }
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
                    onChange={(e) =>
                      onConfigChange({ ...config, score: parseFloat(e.target.value) })
                    }
                    className="w-full"
                    min={0}
                    max={1}
                    step={0.1}
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-2">Rerank 模型</label>
                  <select
                    value={config.rerank_model_id || ''}
                    onChange={(e) =>
                      onConfigChange({ ...config, rerank_model_id: e.target.value })
                    }
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

                {config.retrieve_mode === 'rerank' && (
                  <div className="pt-4 border-t border-gray-100">
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      Rerank 权重: {((config.rerank_weight ?? 1.0) * 100).toFixed(0)}%
                      <span className="text-xs text-gray-500 ml-2">
                        (BM25: {((1 - (config.rerank_weight ?? 1.0)) * 100).toFixed(0)}%)
                      </span>
                    </label>
                    <input
                      type="range"
                      value={config.rerank_weight ?? 1.0}
                      onChange={(e) =>
                        onConfigChange({ ...config, rerank_weight: parseFloat(e.target.value) })
                      }
                      min={0}
                      max={1}
                      step={0.05}
                      className="w-full"
                    />
                    <div className="flex justify-between text-xs text-gray-500 mt-1">
                      <span>纯BM25</span>
                      <span>混合</span>
                      <span>纯Rerank</span>
                    </div>
                    <div className="mt-2 text-xs text-gray-600 bg-gray-50 rounded p-2">
                      {(config.rerank_weight ?? 1.0) === 1.0 && '🔹 当前使用纯 Rerank 语义检索'}
                      {(config.rerank_weight ?? 1.0) === 0.0 && '🔹 当前使用纯 BM25 关键词检索'}
                      {(config.rerank_weight ?? 1.0) > 0 &&
                        (config.rerank_weight ?? 1.0) < 1 &&
                        `🔹 混合检索：${((config.rerank_weight ?? 1.0) * 100).toFixed(0)}% Rerank + ${((1 - (config.rerank_weight ?? 1.0)) * 100).toFixed(0)}% BM25`}
                    </div>
                  </div>
                )}
              </>
            )}
          </div>
        )}
      </div>

      {/* NL2SQL 工具 */}
      <div className="border rounded-lg overflow-hidden">
        <div
          className="flex items-center justify-between p-4 bg-gray-50 cursor-pointer hover:bg-gray-100 transition-colors"
          onClick={() => toggleTool('nl2sql')}
        >
          <div className="flex items-center gap-3">
            <Table className="w-5 h-5 text-green-500" />
            <div>
              <h4 className="font-medium">NL2SQL 数据库查询</h4>
              <p className="text-xs text-gray-500">通过自然语言查询数据库</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <label className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
              <input
                type="checkbox"
                checked={enableNL2SQL}
                onChange={(e) => handleNL2SQLToggle(e.target.checked)}
                className="w-4 h-4 text-blue-500 rounded focus:ring-2 focus:ring-blue-500"
              />
              <span className="text-sm">启用</span>
            </label>
            {expandedTools.nl2sql ? (
              <ChevronUp className="w-5 h-5 text-gray-400" />
            ) : (
              <ChevronDown className="w-5 h-5 text-gray-400" />
            )}
          </div>
        </div>

        {expandedTools.nl2sql && enableNL2SQL && (
          <div className="p-4 space-y-4 border-t">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">数据源 *</label>
              <select
                value={config.nl2sql_datasource_id || ''}
                onChange={(e) => handleNL2SQLDatasourceChange(e.target.value)}
                className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="">选择数据源</option>
                {nl2sqlDatasources.map((ds: any) => (
                  <option key={ds.id} value={ds.id}>
                    {ds.name} ({ds.type} - {ds.db_type || 'CSV/Excel'})
                  </option>
                ))}
              </select>
              <p className="text-xs text-gray-500 mt-1">
                选择数据源后，Agent 可以通过自然语言查询数据库
              </p>
            </div>

            {config.nl2sql_datasource_id && (
              <div className="bg-blue-50 border border-blue-200 rounded-lg p-3">
                <p className="text-sm text-blue-700">
                  <span className="font-medium">Embedding 模型：</span>
                  将自动使用数据源绑定的 Embedding 模型进行 Schema 向量化
                </p>
              </div>
            )}
          </div>
        )}
      </div>

      {/* MCP 工具 */}
      <div className="border rounded-lg overflow-hidden">
        <div
          className="flex items-center justify-between p-4 bg-gray-50 cursor-pointer hover:bg-gray-100 transition-colors"
          onClick={() => toggleTool('mcp')}
        >
          <div className="flex items-center gap-3">
            <MessageSquare className="w-5 h-5 text-purple-500" />
            <div>
              <h4 className="font-medium">MCP 外部工具</h4>
              <p className="text-xs text-gray-500">调用外部 MCP 服务提供的工具</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <label className="flex items-center gap-2" onClick={(e) => e.stopPropagation()}>
              <input
                type="checkbox"
                checked={enableMCP}
                onChange={(e) => handleMCPToggle(e.target.checked)}
                className="w-4 h-4 text-blue-500 rounded focus:ring-2 focus:ring-blue-500"
              />
              <span className="text-sm">启用</span>
            </label>
            {expandedTools.mcp ? (
              <ChevronUp className="w-5 h-5 text-gray-400" />
            ) : (
              <ChevronDown className="w-5 h-5 text-gray-400" />
            )}
          </div>
        </div>

        {expandedTools.mcp && enableMCP && (
          <div className="p-4 space-y-4 border-t">
            {mcpServices.length > 0 ? (
              <>
                <div className="flex items-center justify-between">
                  <p className="text-sm text-gray-600">配置 MCP 服务和工具</p>
                  <button
                    type="button"
                    onClick={addMcpConfig}
                    className="flex items-center gap-1 px-3 py-1.5 text-sm bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors"
                  >
                    <Plus className="w-4 h-4" />
                    添加服务
                  </button>
                </div>

                {mcpConfigs.length === 0 ? (
                  <p className="text-sm text-gray-500 text-center py-4">
                    暂未配置 MCP 服务，点击"添加服务"按钮开始配置
                  </p>
                ) : (
                  <div className="space-y-3">
                    {mcpConfigs.map((mcpConfig) => {
                      const selectedService = mcpServices.find(
                        (s) => s.name === mcpConfig.serviceName
                      );
                      const availableTools = selectedService?.tools || [];

                      return (
                        <div key={mcpConfig.id} className="border rounded-lg p-4 bg-white">
                          <div className="space-y-3">
                            <div className="flex items-start gap-3">
                              <div className="flex-1">
                                <label className="block text-sm font-medium text-gray-700 mb-2">
                                  MCP 服务
                                </label>
                                <select
                                  value={mcpConfig.serviceName}
                                  onChange={(e) =>
                                    updateMcpServiceName(mcpConfig.id, e.target.value)
                                  }
                                  className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                                >
                                  <option value="">选择服务</option>
                                  {mcpServices.map((service) => (
                                    <option key={service.id} value={service.name}>
                                      {service.name}
                                    </option>
                                  ))}
                                </select>
                              </div>
                              <button
                                type="button"
                                onClick={() => removeMcpConfig(mcpConfig.id)}
                                className="mt-7 p-2 text-red-600 hover:bg-red-50 rounded-lg transition-colors"
                                title="删除"
                              >
                                <Minus className="w-4 h-4" />
                              </button>
                            </div>

                            {mcpConfig.serviceName && availableTools.length > 0 && (
                              <div>
                                <label className="block text-sm font-medium text-gray-700 mb-2">
                                  选择工具
                                </label>
                                <select
                                  multiple
                                  value={mcpConfig.selectedTools}
                                  onChange={(e) => {
                                    const selected = Array.from(
                                      e.target.selectedOptions,
                                      (option) => option.value
                                    );
                                    updateMcpTools(mcpConfig.id, selected);
                                  }}
                                  className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 min-h-[120px]"
                                >
                                  {availableTools.map((tool) => (
                                    <option key={tool.name} value={tool.name}>
                                      {tool.name} {tool.description ? `- ${tool.description}` : ''}
                                    </option>
                                  ))}
                                </select>
                                <p className="mt-1 text-xs text-gray-500">
                                  按住 Ctrl/Cmd 可以选择多个工具
                                </p>
                              </div>
                            )}

                            {selectedService && (
                              <div className="bg-purple-50 border border-purple-200 rounded p-2">
                                <p className="text-xs text-purple-700">
                                  {selectedService.description}
                                </p>
                              </div>
                            )}
                          </div>
                        </div>
                      );
                    })}
                  </div>
                )}
              </>
            ) : (
              <p className="text-sm text-gray-500 text-center py-4">
                暂无可用的 MCP 服务，请先在 MCP 服务页面添加服务
              </p>
            )}
          </div>
        )}
      </div>
    </div>
  );
}