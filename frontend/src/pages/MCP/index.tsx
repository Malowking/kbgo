import { useState, useEffect, useCallback, useMemo } from 'react';
import {
  Plus,
  Search,
  Edit2,
  Trash2,
  Power,
  PowerOff,
  TestTube,
  Wrench,
  Activity,
  Clock,
  CheckCircle,
  XCircle
} from 'lucide-react';
import { mcpApi } from '@/services';
import type { MCPRegistry, MCPTool, MCPStats } from '@/types';
import { formatDate } from '@/lib/utils';
import { logger } from '@/lib/logger';
import { showSuccess, showError } from '@/lib/toast';
import CreateMCPModal from './CreateMCPModal';

export default function MCPPage() {
  const [mcpList, setMcpList] = useState<MCPRegistry[]>([]);
  const [loading, setLoading] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [editingMCP, setEditingMCP] = useState<MCPRegistry | null>(null);
  const [expandedMCP, setExpandedMCP] = useState<string | null>(null);
  const [mcpTools, setMcpTools] = useState<Record<string, MCPTool[]>>({});
  const [mcpStats, setMcpStats] = useState<Record<string, MCPStats>>({});

  const fetchMCPList = useCallback(async () => {
    try {
      setLoading(true);
      const response = await mcpApi.list({ page: 1, page_size: 100 });
      setMcpList(response.list || []);
    } catch (error) {
      logger.error('Failed to fetch MCP services:', error);
      showError('加载MCP服务列表失败');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchMCPList();
  }, [fetchMCPList]);

  const handleDelete = useCallback(async (id: string) => {
    if (!window.confirm('确定要删除这个MCP服务吗？')) return;

    try {
      await mcpApi.delete(id);
      showSuccess('删除成功');
      fetchMCPList();
    } catch (error) {
      logger.error('Failed to delete MCP service:', error);
      showError('删除失败');
    }
  }, [fetchMCPList]);

  const handleToggleStatus = useCallback(async (mcp: MCPRegistry) => {
    try {
      const newStatus = mcp.status === 1 ? 0 : 1;
      await mcpApi.updateStatus(mcp.id, newStatus);
      showSuccess(newStatus === 1 ? '已启用' : '已禁用');
      fetchMCPList();
    } catch (error) {
      logger.error('Failed to update status:', error);
      showError('状态更新失败');
    }
  }, [fetchMCPList]);

  const handleTest = useCallback(async (id: string) => {
    try {
      const result = await mcpApi.test(id);
      if (result.success) {
        showSuccess('连接测试成功！');
      } else {
        showError(`连接测试失败：${result.message}`);
      }
    } catch (error) {
      logger.error('Failed to test MCP service:', error);
      showError('连接测试失败');
    }
  }, []);

  const handleEdit = (mcp: MCPRegistry) => {
    setEditingMCP(mcp);
    setShowCreateModal(true);
  };

  const handleModalClose = () => {
    setShowCreateModal(false);
    setEditingMCP(null);
  };

  const handleToggleExpand = useCallback(async (mcpId: string) => {
    if (expandedMCP === mcpId) {
      setExpandedMCP(null);
    } else {
      setExpandedMCP(mcpId);
      // Load tools and stats
      if (!mcpTools[mcpId]) {
        try {
          const toolsResponse = await mcpApi.listTools(mcpId);
          setMcpTools(prev => ({ ...prev, [mcpId]: toolsResponse.tools || [] }));
        } catch (error) {
          logger.error('Failed to load tools:', error);
        }
      }
      if (!mcpStats[mcpId]) {
        try {
          const statsResponse = await mcpApi.stats(mcpId);
          setMcpStats(prev => ({ ...prev, [mcpId]: statsResponse }));
        } catch (error) {
          logger.error('Failed to load stats:', error);
        }
      }
    }
  }, [expandedMCP, mcpTools, mcpStats]);

  const filteredMCPList = useMemo(() =>
    mcpList.filter((mcp) =>
      mcp.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      mcp.description.toLowerCase().includes(searchQuery.toLowerCase())
    ),
    [mcpList, searchQuery]
  );

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">MCP服务管理</h1>
          <p className="mt-2 text-gray-600">
            管理Model Context Protocol服务和工具
          </p>
        </div>
        <button
          onClick={() => setShowCreateModal(true)}
          className="btn btn-primary flex items-center"
        >
          <Plus className="w-5 h-5 mr-2" />
          添加MCP服务
        </button>
      </div>

      {/* Search */}
      <div className="card">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400 w-5 h-5" />
          <input
            type="text"
            placeholder="搜索MCP服务..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="input pl-10"
          />
        </div>
      </div>

      {/* MCP Service List */}
      {loading ? (
        <div className="text-center py-12">
          <div className="inline-block w-8 h-8 border-4 border-primary-600 border-t-transparent rounded-full animate-spin"></div>
          <p className="mt-4 text-gray-600">加载中...</p>
        </div>
      ) : filteredMCPList.length === 0 ? (
        <div className="card text-center py-12">
          <p className="text-gray-500">
            {searchQuery ? '没有找到匹配的MCP服务' : '还没有MCP服务，点击上方按钮添加一个'}
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {filteredMCPList.map((mcp) => {
            const isExpanded = expandedMCP === mcp.id;
            const tools = mcpTools[mcp.id] || [];
            const stats = mcpStats[mcp.id];

            return (
              <div key={mcp.id} className="card">
                {/* MCP Header */}
                <div className="flex items-start justify-between">
                  <div className="flex-1">
                    <div className="flex items-center space-x-3 mb-2">
                      <h3 className="text-lg font-semibold text-gray-900">
                        {mcp.name}
                      </h3>
                      <span className={`px-2 py-1 text-xs font-medium rounded ${
                        mcp.status === 1
                          ? 'bg-green-100 text-green-700'
                          : 'bg-gray-100 text-gray-700'
                      }`}>
                        {mcp.status === 1 ? '启用' : '禁用'}
                      </span>
                    </div>
                    <p className="text-gray-600 text-sm mb-2">{mcp.description}</p>
                    <p className="text-gray-500 text-xs font-mono">{mcp.endpoint}</p>
                  </div>

                  {/* Actions */}
                  <div className="flex space-x-2">
                    <button
                      onClick={() => handleTest(mcp.id)}
                      className="p-2 rounded hover:bg-gray-100 text-purple-600"
                      title="测试连接"
                    >
                      <TestTube className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleToggleExpand(mcp.id)}
                      className="p-2 rounded hover:bg-gray-100 text-cyan-600"
                      title="查看工具"
                    >
                      <Wrench className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleToggleStatus(mcp)}
                      className={`p-2 rounded hover:bg-gray-100 ${
                        mcp.status === 1 ? 'text-green-600' : 'text-gray-400'
                      }`}
                      title={mcp.status === 1 ? '禁用' : '启用'}
                    >
                      {mcp.status === 1 ? <Power className="w-4 h-4" /> : <PowerOff className="w-4 h-4" />}
                    </button>
                    <button
                      onClick={() => handleEdit(mcp)}
                      className="p-2 rounded hover:bg-gray-100 text-blue-600"
                      title="编辑"
                    >
                      <Edit2 className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleDelete(mcp.id)}
                      className="p-2 rounded hover:bg-gray-100 text-red-600"
                      title="删除"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>

                {/* Stats */}
                {stats && (
                  <div className="mt-4 pt-4 border-t border-gray-200 grid grid-cols-4 gap-4">
                    <div className="flex items-center space-x-2">
                      <Activity className="w-4 h-4 text-gray-400" />
                      <div>
                        <p className="text-xs text-gray-500">总调用</p>
                        <p className="text-sm font-semibold">{stats.total_calls}</p>
                      </div>
                    </div>
                    <div className="flex items-center space-x-2">
                      <CheckCircle className="w-4 h-4 text-green-500" />
                      <div>
                        <p className="text-xs text-gray-500">成功率</p>
                        <p className="text-sm font-semibold">{(stats.success_rate * 100).toFixed(1)}%</p>
                      </div>
                    </div>
                    <div className="flex items-center space-x-2">
                      <XCircle className="w-4 h-4 text-red-500" />
                      <div>
                        <p className="text-xs text-gray-500">失败次数</p>
                        <p className="text-sm font-semibold">{stats.failed_calls}</p>
                      </div>
                    </div>
                    <div className="flex items-center space-x-2">
                      <Clock className="w-4 h-4 text-blue-500" />
                      <div>
                        <p className="text-xs text-gray-500">平均耗时</p>
                        <p className="text-sm font-semibold">{stats.avg_duration.toFixed(0)}ms</p>
                      </div>
                    </div>
                  </div>
                )}

                {/* Expanded: Tools */}
                {isExpanded && (
                  <div className="mt-4 pt-4 border-t border-gray-200">
                    <h4 className="text-sm font-semibold text-gray-700 mb-3">可用工具</h4>
                    {tools.length === 0 ? (
                      <p className="text-sm text-gray-500">没有可用工具</p>
                    ) : (
                      <div className="space-y-2">
                        {tools.map((tool, index) => (
                          <div key={index} className="bg-gray-50 rounded p-3">
                            <div className="flex items-start justify-between">
                              <div>
                                <h5 className="text-sm font-medium text-gray-900">{tool.name}</h5>
                                <p className="text-xs text-gray-600 mt-1">{tool.description}</p>
                              </div>
                            </div>
                            {tool.inputSchema && (
                              <div className="mt-2 text-xs">
                                <span className="text-gray-500">参数：</span>
                                <code className="ml-2 text-gray-700">
                                  {tool.inputSchema.required?.join(', ') || '无'}
                                </code>
                              </div>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}

                {/* Footer */}
                <div className="mt-4 pt-4 border-t border-gray-100 flex items-center justify-between text-xs text-gray-500">
                  <span>创建于 {formatDate(mcp.create_time)}</span>
                  <span>超时时间：{mcp.timeout}秒</span>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Create/Edit Modal */}
      {showCreateModal && (
        <CreateMCPModal
          mcp={editingMCP}
          onClose={handleModalClose}
          onSuccess={fetchMCPList}
        />
      )}
    </div>
  );
}
