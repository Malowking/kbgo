import { useState, useEffect } from 'react';
import { X } from 'lucide-react';
import { mcpApi } from '@/services';
import type { MCPRegistry } from '@/types';
import { showError } from '@/lib/toast';

interface CreateMCPModalProps {
  mcp?: MCPRegistry | null;
  onClose: () => void;
  onSuccess: () => void;
}

export default function CreateMCPModal({ mcp, onClose, onSuccess }: CreateMCPModalProps) {
  const [formData, setFormData] = useState({
    name: '',
    description: '',
    endpoint: '',
    api_key: '',
    timeout: 30,
    headers: '',
  });
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (mcp) {
      setFormData({
        name: mcp.name,
        description: mcp.description,
        endpoint: mcp.endpoint,
        api_key: mcp.api_key || '',
        timeout: mcp.timeout,
        headers: mcp.headers ? JSON.stringify(mcp.headers, null, 2) : '',
      });
    }
  }, [mcp]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!formData.name.trim()) {
      showError('请输入服务名称');
      return;
    }

    if (!formData.endpoint.trim()) {
      showError('请输入服务端点');
      return;
    }

    try {
      setLoading(true);

      let headers: Record<string, string> | undefined;
      if (formData.headers.trim()) {
        try {
          headers = JSON.parse(formData.headers);
        } catch {
          showError('Headers格式错误，请输入有效的JSON');
          setLoading(false);
          return;
        }
      }

      const data = {
        name: formData.name.trim(),
        description: formData.description.trim(),
        endpoint: formData.endpoint.trim(),
        api_key: formData.api_key.trim() || undefined,
        timeout: formData.timeout,
        headers,
      };

      if (mcp) {
        await mcpApi.update(mcp.id, { ...data, id: mcp.id });
      } else {
        await mcpApi.create(data);
      }

      onSuccess(); // 先刷新数据
      onClose(); // 再关闭模态框
    } catch (error) {
      console.error('Failed to save MCP service:', error);
      showError('保存失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-2xl max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between p-6 border-b border-gray-200">
          <h2 className="text-xl font-semibold text-gray-900">
            {mcp ? '编辑MCP服务' : '创建MCP服务'}
          </h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              服务名称 *
            </label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              className="input"
              placeholder="例如：weather-service"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              服务描述
            </label>
            <textarea
              value={formData.description}
              onChange={(e) => setFormData({ ...formData, description: e.target.value })}
              className="input min-h-[80px]"
              placeholder="简要描述此MCP服务的功能"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              服务端点 *
            </label>
            <input
              type="url"
              value={formData.endpoint}
              onChange={(e) => setFormData({ ...formData, endpoint: e.target.value })}
              className="input"
              placeholder="https://api.example.com/mcp"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              API Key
            </label>
            <input
              type="password"
              value={formData.api_key}
              onChange={(e) => setFormData({ ...formData, api_key: e.target.value })}
              className="input"
              placeholder="可选，用于认证"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              超时时间（秒）
            </label>
            <input
              type="number"
              value={formData.timeout}
              onChange={(e) => setFormData({ ...formData, timeout: parseInt(e.target.value) || 30 })}
              className="input"
              min="1"
              max="300"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              自定义Headers（JSON格式）
            </label>
            <textarea
              value={formData.headers}
              onChange={(e) => setFormData({ ...formData, headers: e.target.value })}
              className="input font-mono text-sm min-h-[100px]"
              placeholder={'{\n  "X-Custom-Header": "value"\n}'}
            />
          </div>

          <div className="flex justify-end space-x-3 pt-4">
            <button
              type="button"
              onClick={onClose}
              className="btn btn-secondary"
              disabled={loading}
            >
              取消
            </button>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={loading}
            >
              {loading ? '保存中...' : mcp ? '更新' : '创建'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
