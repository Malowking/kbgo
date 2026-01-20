import { useState, useEffect, useCallback } from 'react';
import { Plus, Upload, Database, Trash2, Eye, X } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { nl2sqlApi, modelApi } from '@/services';
import { logger } from '@/lib/logger';
import { showError, showSuccess, showWarning } from '@/lib/toast';
import { useConfirm } from '@/hooks/useConfirm';

interface DataSource {
  id: string;
  name: string;
  type: string;
  db_type?: string;
  status: string;
  create_time: string;
}

export default function NL2SQLDataSource() {
  const navigate = useNavigate();
  const [datasources, setDatasources] = useState<DataSource[]>([]);
  const [loading, setLoading] = useState(false);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const { confirm, ConfirmDialog } = useConfirm();

  // Form state
  const [datasourceType, setDatasourceType] = useState<'jdbc' | 'file'>('jdbc');
  const [name, setName] = useState('');
  const [dbType, setDbType] = useState('postgresql');
  const [host, setHost] = useState('');
  const [port, setPort] = useState(5432);
  const [database, setDatabase] = useState('');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');

  // 模型选择
  const [embeddingModels, setEmbeddingModels] = useState<any[]>([]);
  const [selectedEmbeddingModel, setSelectedEmbeddingModel] = useState('');

  useEffect(() => {
    fetchDatasources();
    fetchModels();
  }, []);

  const fetchModels = useCallback(async () => {
    try {
      const response = await modelApi.list();
      const embList = (response.models || []).filter(
        (m: any) => m.type === 'embedding' && m.enabled
      );
      setEmbeddingModels(embList);
      if (embList.length > 0) setSelectedEmbeddingModel(embList[0].model_id);
    } catch (error) {
      logger.error('Failed to fetch models:', error);
    }
  }, []);

  const fetchDatasources = useCallback(async () => {
    try {
      setLoading(true);
      const response = await nl2sqlApi.listDatasources();
      setDatasources(response.list || []);
    } catch (error) {
      logger.error('Failed to fetch datasources:', error);
      showError('获取数据源列表失败');
    } finally {
      setLoading(false);
    }
  }, []);

  const handleCreate = () => {
    setShowCreateModal(true);
    resetForm();
  };

  const resetForm = () => {
    setName('');
    setDatasourceType('jdbc');
    setDbType('postgresql');
    setHost('');
    setPort(5432);
    setDatabase('');
    setUsername('');
    setPassword('');
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!name.trim()) {
      showWarning('请输入数据源名称');
      return;
    }

    if (!selectedEmbeddingModel) {
      showWarning('请选择Embedding模型');
      return;
    }

    try {
      setLoading(true);

      let config: any = {};

      if (datasourceType === 'jdbc') {
        if (!host || !database || !username) {
          showWarning('请填写完整的数据库连接信息');
          return;
        }
        config = {
          host,
          port,
          database,
          username,
          password,
          ssl_mode: 'disable',
        };

        await nl2sqlApi.createDatasource({
          name,
          type: datasourceType,
          db_type: dbType,
          config,
          created_by: 'user-001',
          embedding_model_id: selectedEmbeddingModel,
        });

        showSuccess('JDBC数据源创建成功');
      } else if (datasourceType === 'file') {
        config = {};

        await nl2sqlApi.createDatasource({
          name,
          type: 'csv',
          config,
          created_by: 'user-001',
          embedding_model_id: selectedEmbeddingModel,
        });

        showSuccess('文件数据源创建成功，请进入详情页添加表');
      }

      setShowCreateModal(false);
      fetchDatasources();
    } catch (error) {
      logger.error('Failed to create datasource:', error);
      showError('创建数据源失败');
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    const confirmed = await confirm({
      message: '确定要删除这个数据源吗？相关的 Schema 信息也会被删除。',
      type: 'danger',
    });
    if (!confirmed) return;

    try {
      await nl2sqlApi.deleteDatasource(id);
      showSuccess('删除成功');
      fetchDatasources();
    } catch (error) {
      logger.error('Failed to delete datasource:', error);
      showError('删除失败');
    }
  };

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="border-b bg-white px-6 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Database className="w-6 h-6 text-blue-500" />
            <div>
              <h1 className="text-2xl font-bold">NL2SQL 数据源</h1>
              <p className="text-sm text-gray-500 mt-1">
                配置数据库连接或上传数据文件，支持自然语言查询
              </p>
            </div>
          </div>
          <button
            onClick={handleCreate}
            className="flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors"
          >
            <Plus className="w-4 h-4" />
            添加数据源
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto p-6">
        {loading && datasources.length === 0 ? (
          <div className="text-center py-12">
            <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
          </div>
        ) : datasources.length === 0 ? (
          <div className="text-center py-12">
            <Database className="w-16 h-16 text-gray-300 mx-auto mb-4" />
            <p className="text-gray-500 mb-4">还没有添加任何数据源</p>
            <button
              onClick={handleCreate}
              className="inline-flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors"
            >
              <Plus className="w-4 h-4" />
              添加第一个数据源
            </button>
          </div>
        ) : (
          <div className="max-w-6xl mx-auto">
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              {datasources.map((ds) => (
                <div
                  key={ds.id}
                  onClick={() => navigate(`/nl2sql-datasource/${ds.id}`)}
                  className="bg-white rounded-lg border shadow-sm hover:shadow-md transition-shadow cursor-pointer"
                >
                  <div className="p-4">
                    <div className="flex items-start justify-between mb-3">
                      <div className="flex items-center gap-2">
                        {ds.type === 'jdbc' ? (
                          <Database className="w-5 h-5 text-blue-500" />
                        ) : (
                          <Upload className="w-5 h-5 text-green-500" />
                        )}
                        <h3 className="font-semibold text-lg">{ds.name}</h3>
                      </div>
                    </div>

                    <div className="space-y-2 mb-4">
                      <div className="flex items-center text-sm text-gray-600">
                        <span className="font-medium mr-2">类型:</span>
                        <span>
                          {ds.type === 'jdbc' ? `JDBC (${ds.db_type})` : ds.type.toUpperCase()}
                        </span>
                      </div>
                      <div className="flex items-center text-sm text-gray-600">
                        <span className="font-medium mr-2">创建时间:</span>
                        <span>{new Date(ds.create_time).toLocaleString()}</span>
                      </div>
                    </div>

                    <div className="flex gap-2" onClick={(e) => e.stopPropagation()}>
                      <button
                        onClick={() => navigate(`/nl2sql-datasource/${ds.id}`)}
                        className="flex-1 flex items-center justify-center gap-1 px-3 py-2 border rounded-lg hover:bg-gray-50 transition-colors text-sm"
                      >
                        <Eye className="w-4 h-4" />
                        查看详情
                      </button>
                      <button
                        onClick={(e) => handleDelete(ds.id, e)}
                        className="px-3 py-2 border border-red-200 text-red-600 rounded-lg hover:bg-red-50 transition-colors"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Create Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-auto">
            <div className="sticky top-0 bg-white border-b px-6 py-4 flex items-center justify-between">
              <h2 className="text-xl font-semibold">添加数据源</h2>
              <button onClick={() => setShowCreateModal(false)} className="text-gray-400 hover:text-gray-600">
                <X className="w-5 h-5" />
              </button>
            </div>

            <form onSubmit={handleSubmit} className="p-6 space-y-6">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">数据源名称 *</label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder="例如：生产环境数据库"
                  required
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">数据源类型 *</label>
                <div className="grid grid-cols-2 gap-4">
                  <button
                    type="button"
                    onClick={() => setDatasourceType('jdbc')}
                    className={`p-4 border-2 rounded-lg transition-all ${
                      datasourceType === 'jdbc' ? 'border-blue-500 bg-blue-50' : 'border-gray-200 hover:border-gray-300'
                    }`}
                  >
                    <Database className="w-8 h-8 mx-auto mb-2 text-blue-500" />
                    <div className="text-sm font-medium">JDBC 数据库</div>
                    <div className="text-xs text-gray-500 mt-1">PostgreSQL</div>
                  </button>
                  <button
                    type="button"
                    onClick={() => setDatasourceType('file')}
                    className={`p-4 border-2 rounded-lg transition-all ${
                      datasourceType === 'file' ? 'border-blue-500 bg-blue-50' : 'border-gray-200 hover:border-gray-300'
                    }`}
                  >
                    <Upload className="w-8 h-8 mx-auto mb-2 text-green-500" />
                    <div className="text-sm font-medium">文件数据源</div>
                    <div className="text-xs text-gray-500 mt-1">CSV / Excel</div>
                  </button>
                </div>
              </div>

              {datasourceType === 'jdbc' && (
                <>
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">数据库类型 *</label>
                    <select
                      value={dbType}
                      onChange={(e) => {
                        setDbType(e.target.value);
                        setPort(5432);
                      }}
                      className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                    >
                      <option value="postgresql">PostgreSQL</option>
                    </select>
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">主机地址 *</label>
                      <input
                        type="text"
                        value={host}
                        onChange={(e) => setHost(e.target.value)}
                        className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                        placeholder="localhost"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">端口 *</label>
                      <input
                        type="number"
                        value={port}
                        onChange={(e) => setPort(parseInt(e.target.value))}
                        className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                      />
                    </div>
                  </div>

                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">数据库名称 *</label>
                    <input
                      type="text"
                      value={database}
                      onChange={(e) => setDatabase(e.target.value)}
                      className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                      placeholder="mydb"
                    />
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">用户名 *</label>
                      <input
                        type="text"
                        value={username}
                        onChange={(e) => setUsername(e.target.value)}
                        className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                        placeholder="readonly_user"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">密码 *</label>
                      <input
                        type="password"
                        value={password}
                        onChange={(e) => setPassword(e.target.value)}
                        className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                        placeholder="••••••••"
                      />
                    </div>
                  </div>

                  <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-3">
                    <p className="text-sm text-yellow-800">⚠️ 建议使用只读账号以确保数据安全</p>
                  </div>
                </>
              )}

              {datasourceType === 'file' && (
                <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
                  <p className="text-sm text-blue-800">
                    ℹ️ 文件数据源创建后，请进入详情页使用"添加表"功能上传 CSV 或 Excel 文件。
                  </p>
                </div>
              )}

              {/* Embedding模型选择 */}
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Embedding 模型 <span className="text-red-500">*</span>
                </label>
                {embeddingModels.length === 0 ? (
                  <div className="px-3 py-2 border rounded-lg text-red-500 bg-red-50">
                    暂无可用的 Embedding 模型，请先在模型管理中配置
                  </div>
                ) : (
                  <select
                    value={selectedEmbeddingModel}
                    onChange={(e) => setSelectedEmbeddingModel(e.target.value)}
                    className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                    required
                  >
                    {embeddingModels.map((model) => (
                      <option key={model.model_id} value={model.model_id}>
                        {model.name}
                      </option>
                    ))}
                  </select>
                )}
                <p className="text-xs text-gray-500 mt-1">用于Schema向量化和智能检索，创建后不可修改</p>
              </div>

              <div className="flex justify-end gap-3 pt-4 border-t">
                <button
                  type="button"
                  onClick={() => setShowCreateModal(false)}
                  className="px-4 py-2 border rounded-lg hover:bg-gray-50 transition-colors"
                >
                  取消
                </button>
                <button
                  type="submit"
                  disabled={loading}
                  className="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors disabled:opacity-50"
                >
                  {loading ? '创建中...' : '创建数据源'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      <ConfirmDialog />
    </div>
  );
}
