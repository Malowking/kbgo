import { useState, useEffect } from 'react';
import { Plus, Upload, RefreshCw, Columns, Link as LinkIcon, Trash2 } from 'lucide-react';
import { nl2sqlApi, modelApi } from '@/services';
import { showError, showSuccess, showWarning } from '@/lib/toast';
import { logger } from '@/lib/logger';

interface DataSource {
  id: string;
  name: string;
  type: string;
  db_type?: string;
  status: string;
  create_time: string;
  embedding_model_id: string;
}

interface TablesTabProps {
  datasource: DataSource;
  onUpdate: () => void;
}

interface SchemaInfo {
  tables: Array<{
    id: string;
    table_name: string;
    display_name?: string; // 用户自定义的显示名称
    row_count: number;
    parsed: boolean; // 新增：表解析状态
    columns: Array<{
      id: string;
      column_name: string;
      data_type: string;
      nullable: boolean;
      description?: string;
    }>;
    primary_keys?: string[];
  }>;
  relations: Array<{
    id: string;
    source_table: string;
    target_table: string;
    relation_type: string;
  }>;
}

export default function TablesTab({ datasource, onUpdate }: TablesTabProps) {
  const [schemaInfo, setSchemaInfo] = useState<SchemaInfo | null>(null);
  const [loading, setLoading] = useState(false);
  const [showAddTableModal, setShowAddTableModal] = useState(false);
  const [showParseModal, setShowParseModal] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [tableToDelete, setTableToDelete] = useState<{ id: string; name: string } | null>(null);
  const [deletingTable, setDeletingTable] = useState(false);
  const [fileToAdd, setFileToAdd] = useState<File | null>(null);
  const [tableDisplayName, setTableDisplayName] = useState(''); // 用户输入的表显示名称
  const [addingTable, setAddingTable] = useState(false);
  const [parseLoading, setParseLoading] = useState(false);

  // 模型选择
  const [models, setModels] = useState<any[]>([]);
  const [selectedLLMModel, setSelectedLLMModel] = useState('');

  useEffect(() => {
    // 总是尝试获取 Schema（即使未解析，也可能有手动添加的表）
    fetchSchema();
    fetchModels();
  }, [datasource.id]);

  const fetchModels = async () => {
    try {
      const response = await modelApi.list();
      const llmList = (response.models || [])
        .filter((m: any) => (m.type === 'llm' || m.type === 'multimodal') && m.enabled)
        .sort((a: any, b: any) => a.name.localeCompare(b.name));
      setModels(llmList);
      if (llmList.length > 0) setSelectedLLMModel(llmList[0].model_id);
    } catch (error) {
      logger.error('Failed to fetch models:', error);
    }
  };

  const fetchSchema = async () => {
    try {
      setLoading(true);
      const response = await nl2sqlApi.getSchema(datasource.id);
      setSchemaInfo(response);
    } catch (error) {
      logger.error('Failed to fetch schema:', error);
      showError('获取Schema信息失败');
    } finally {
      setLoading(false);
    }
  };

  const handleAddTableFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) {
      setFileToAdd(file);
    }
  };

  const handleAddTable = async () => {
    if (!fileToAdd) {
      showWarning('请选择要上传的文件');
      return;
    }

    try {
      setAddingTable(true);
      await nl2sqlApi.addTable(datasource.id, fileToAdd, tableDisplayName);
      showSuccess('添加成功');
      setShowAddTableModal(false);
      setFileToAdd(null);
      setTableDisplayName(''); // 重置显示名称
      fetchSchema();
      onUpdate();
    } catch (error) {
      logger.error('Failed to add table:', error);
      showError('添加表失败');
    } finally {
      setAddingTable(false);
    }
  };

  const handleParseSchema = () => {
    setShowParseModal(true);
  };

  const confirmParseSchema = async () => {
    if (!selectedLLMModel) {
      showWarning('请选择LLM模型');
      return;
    }

    try {
      setParseLoading(true);
      await nl2sqlApi.parseSchema(datasource.id, {
        llm_model_id: selectedLLMModel,
      });
      showSuccess(`"${datasource.name}" 开始解析`);
      setShowParseModal(false);

      setTimeout(() => {
        onUpdate();
        fetchSchema();
      }, 3000);
    } catch (error) {
      logger.error('Failed to parse schema:', error);
      showError('解析Schema失败');
    } finally {
      setParseLoading(false);
    }
  };

  // 删除表处理函数
  const handleDeleteTable = (tableId: string, tableName: string) => {
    setTableToDelete({ id: tableId, name: tableName });
    setShowDeleteModal(true);
  };

  const confirmDeleteTable = async () => {
    if (!tableToDelete) return;

    try {
      setDeletingTable(true);
      const response = await nl2sqlApi.deleteTable(datasource.id, tableToDelete.id);
      showSuccess(response.message || `表 "${tableToDelete.name}" 已成功删除`);
      setShowDeleteModal(false);
      setTableToDelete(null);
      fetchSchema();
      onUpdate();
    } catch (error) {
      logger.error('Failed to delete table:', error);
      showError('删除表失败');
    } finally {
      setDeletingTable(false);
    }
  };

  // 计算解析状态：当前解析的表数和总表数
  const parsedTableCount = schemaInfo?.tables?.filter(t => t.parsed).length || 0;
  const totalTableCount = schemaInfo?.tables?.length || 0;
  const allTablesParsed = totalTableCount > 0 && parsedTableCount === totalTableCount;

  return (
    <div className="p-6 space-y-6">
      {/* Actions Bar */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold text-gray-900">Schema 管理</h2>
          <p className="text-sm text-gray-600 mt-1">
            {totalTableCount > 0
              ? `共 ${totalTableCount} 个表（已解析 ${parsedTableCount} 个），${schemaInfo?.relations?.length || 0} 个关系`
              : '暂无表信息'}
          </p>
        </div>
        <div className="flex gap-3">
          {(datasource.type === 'csv' || datasource.type === 'excel') && (
            <button
              onClick={() => setShowAddTableModal(true)}
              className="flex items-center gap-2 px-4 py-2 bg-green-500 text-white rounded-lg hover:bg-green-600 transition-colors"
            >
              <Plus className="w-4 h-4" />
              添加表
            </button>
          )}
          {!allTablesParsed ? (
            <button
              onClick={handleParseSchema}
              disabled={parseLoading || totalTableCount === 0}
              className="flex items-center gap-2 px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors disabled:opacity-50"
            >
              <RefreshCw className={`w-4 h-4 ${parseLoading ? 'animate-spin' : ''}`} />
              {parsedTableCount > 0 ? '继续解析' : '解析Schema'}
            </button>
          ) : (
            <button
              onClick={handleParseSchema}
              disabled={parseLoading}
              className="flex items-center gap-2 px-4 py-2 border border-blue-500 text-blue-600 rounded-lg hover:bg-blue-50 transition-colors disabled:opacity-50"
            >
              <RefreshCw className={`w-4 h-4 ${parseLoading ? 'animate-spin' : ''}`} />
              重新解析
            </button>
          )}
        </div>
      </div>

      {/* Schema Content */}
      {loading ? (
        <div className="text-center py-12">
          <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
        </div>
      ) : schemaInfo && schemaInfo.tables.length > 0 ? (
        <div className="space-y-6">
          {/* Tables */}
          <div className="space-y-4">
            <h3 className="text-base font-semibold text-gray-900 flex items-center gap-2">
              <Columns className="w-5 h-5" />
              数据表
            </h3>
            {schemaInfo.tables.map((table) => (
              <div key={table.id} className="bg-white rounded-lg border overflow-hidden">
                <div className="bg-gray-50 px-4 py-3 border-b">
                  <div className="flex items-center justify-between">
                    <div className="flex-1">
                      <div className="flex items-center gap-3">
                        <h4 className="font-semibold text-gray-900">
                          {table.display_name || table.table_name}
                        </h4>
                        {(datasource.type === 'csv' || datasource.type === 'excel') && (
                          <button
                            onClick={() => handleDeleteTable(table.id, table.display_name || table.table_name)}
                            className="p-1.5 text-red-600 hover:bg-red-50 rounded transition-colors"
                            title="删除表"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        )}
                      </div>
                      {table.display_name && table.display_name !== table.table_name}
                    </div>
                    <span className="text-sm text-gray-500">{table.row_count} 行</span>
                  </div>
                  {table.primary_keys && table.primary_keys.length > 0 && (
                    <p className="text-base text-gray-600 mt-1">
                      主键: {table.primary_keys.join(', ')}
                    </p>
                  )}
                </div>
                <div className="p-4">
                  <div className="flex items-center gap-2 mb-3">
                    <Columns className="w-4 h-4 text-gray-500" />
                    <span className="text-sm font-medium text-gray-700">
                      列信息 ({table.columns.length})
                    </span>
                  </div>
                  <div className="space-y-2">
                    {table.columns.map((column) => (
                      <div
                        key={column.id}
                        className="flex items-center justify-between p-2 bg-gray-50 rounded"
                      >
                        <div className="flex items-center gap-3">
                          <span className="font-mono text-sm font-medium text-gray-900">
                            {column.column_name}
                          </span>
                          <span className="text-xs text-gray-500 bg-white px-2 py-1 rounded">
                            {column.data_type}
                          </span>
                          {!column.nullable && (
                            <span className="text-xs text-red-600 bg-red-50 px-2 py-1 rounded">
                              NOT NULL
                            </span>
                          )}
                        </div>
                        {column.description && (
                          <span className="text-sm text-gray-600">{column.description}</span>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            ))}
          </div>

          {/* Relations */}
          {schemaInfo.relations.length > 0 && (
            <div className="space-y-4">
              <h3 className="text-base font-semibold text-gray-900 flex items-center gap-2">
                <LinkIcon className="w-5 h-5" />
                表关系
              </h3>
              <div className="space-y-2">
                {schemaInfo.relations.map((relation) => (
                  <div
                    key={relation.id}
                    className="bg-white flex items-center gap-3 p-3 border rounded-lg"
                  >
                    <span className="font-medium text-gray-900">{relation.source_table}</span>
                    <span className="text-gray-500">→</span>
                    <span className="font-medium text-gray-900">{relation.target_table}</span>
                    <span className="ml-auto text-sm text-blue-700 bg-blue-100 px-2 py-1 rounded">
                      {relation.relation_type}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      ) : (
        <div className="bg-white rounded-lg border p-12 text-center">
          <p className="text-gray-500">暂无表信息</p>
        </div>
      )}

      {/* Add Table Modal */}
      {showAddTableModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-lg shadow-xl max-w-md w-full">
            <div className="px-6 py-4 border-b">
              <h2 className="text-xl font-semibold">添加表 - {datasource.name}</h2>
              <p className="text-sm text-gray-500 mt-1">上传CSV或Excel文件添加新表到数据源</p>
            </div>

            <div className="p-6 space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">表显示名称</label>
                <input
                  type="text"
                  value={tableDisplayName}
                  onChange={(e) => setTableDisplayName(e.target.value)}
                  placeholder="例如：用户信息表（可选，留空则使用文件名）"
                  className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <p className="text-xs text-gray-500 mt-1">
                  用于展示的表名称，如果不填写则使用文件名
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">选择文件 *</label>
                <div className="border-2 border-dashed border-gray-300 rounded-lg p-8 text-center">
                  <input
                    type="file"
                    accept={datasource.type === 'csv' ? '.csv' : '.xlsx,.xls'}
                    onChange={handleAddTableFileChange}
                    className="hidden"
                    id="table-file-upload"
                  />
                  <label htmlFor="table-file-upload" className="cursor-pointer flex flex-col items-center">
                    <Upload className="w-12 h-12 text-gray-400 mb-3" />
                    {fileToAdd ? (
                      <div className="text-sm">
                        <p className="font-medium text-gray-700">{fileToAdd.name}</p>
                        <p className="text-gray-500 mt-1">
                          {(fileToAdd.size / 1024 / 1024).toFixed(2)} MB
                        </p>
                      </div>
                    ) : (
                      <>
                        <p className="text-sm font-medium text-gray-700">点击选择文件或拖拽文件到此处</p>
                        <p className="text-xs text-gray-500 mt-1">
                          支持 {datasource.type === 'csv' ? 'CSV' : 'Excel'} 格式，最大 100MB
                        </p>
                      </>
                    )}
                  </label>
                </div>
              </div>

              <div className="bg-blue-50 border border-blue-200 rounded-lg p-3">
                <p className="text-sm text-blue-800">
                  新表将自动添加到数据源的schema中。记得添加后需要重新解析Schema以更新向量索引。
                </p>
              </div>
            </div>

            <div className="px-6 py-4 border-t flex justify-end gap-3">
              <button
                onClick={() => {
                  setShowAddTableModal(false);
                  setFileToAdd(null);
                  setTableDisplayName('');
                }}
                disabled={addingTable}
                className="px-4 py-2 border rounded-lg hover:bg-gray-50 transition-colors disabled:opacity-50"
              >
                取消
              </button>
              <button
                onClick={handleAddTable}
                disabled={addingTable || !fileToAdd}
                className="px-4 py-2 bg-green-500 text-white rounded-lg hover:bg-green-600 transition-colors disabled:opacity-50"
              >
                {addingTable ? '上传中...' : '添加表'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Parse Schema Modal */}
      {showParseModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-lg shadow-xl max-w-md w-full">
            <div className="px-6 py-4 border-b">
              <h2 className="text-xl font-semibold">解析 Schema - {datasource.name}</h2>
              <p className="text-sm text-gray-500 mt-1">选择用于Schema解析和增强的模型</p>
            </div>

            <div className="p-6 space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">LLM 模型 *</label>
                <select
                  value={selectedLLMModel}
                  onChange={(e) => setSelectedLLMModel(e.target.value)}
                  className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  {models.map((model) => (
                    <option key={model.model_id} value={model.model_id}>
                      {model.name}
                    </option>
                  ))}
                </select>
                <p className="text-xs text-gray-500 mt-1">用于增强表和列的描述信息</p>
              </div>

              <div className="bg-blue-50 border border-blue-200 rounded-lg p-3">
                <p className="text-sm text-blue-800">
                  Embedding模型将使用数据源创建时绑定的模型
                </p>
              </div>

              {models.length === 0 && (
                <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-3">
                  <p className="text-sm text-yellow-800">请先配置LLM模型</p>
                </div>
              )}
            </div>

            <div className="px-6 py-4 border-t flex justify-end gap-3">
              <button
                onClick={() => setShowParseModal(false)}
                disabled={parseLoading}
                className="px-4 py-2 border rounded-lg hover:bg-gray-50 transition-colors disabled:opacity-50"
              >
                取消
              </button>
              <button
                onClick={confirmParseSchema}
                disabled={parseLoading || models.length === 0}
                className="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors disabled:opacity-50"
              >
                {parseLoading ? '解析中...' : '开始解析'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Table Confirmation Modal */}
      {showDeleteModal && tableToDelete && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
          <div className="bg-white rounded-lg shadow-xl max-w-md w-full">
            <div className="px-6 py-4 border-b">
              <h2 className="text-xl font-semibold text-red-600">确认删除表</h2>
              <p className="text-sm text-gray-500 mt-1">此操作无法撤销</p>
            </div>

            <div className="p-6 space-y-4">
              <div className="bg-red-50 border border-red-200 rounded-lg p-4">
                <p className="text-sm text-red-800 font-medium mb-2">
                  您确定要删除表 "{tableToDelete.name}" 吗？
                </p>
                <p className="text-sm text-red-700">
                  删除表将会同时删除：
                </p>
                <ul className="text-sm text-red-700 list-disc list-inside mt-2 space-y-1">
                  <li>所有列信息 (nl2sql_columns)</li>
                  <li>相关的表关系 (nl2sql_relations)</li>
                  <li>向量文档 (nl2sql_vector_docs)</li>
                  <li>相关指标 (nl2sql_metrics)</li>
                  <li>数据库中的实际数据表</li>
                  <li>文件系统中的源文件</li>
                </ul>
              </div>

              <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-3">
                <p className="text-sm text-yellow-800">
                  建议删除表后重新解析Schema以更新向量索引
                </p>
              </div>
            </div>

            <div className="px-6 py-4 border-t flex justify-end gap-3">
              <button
                onClick={() => {
                  setShowDeleteModal(false);
                  setTableToDelete(null);
                }}
                disabled={deletingTable}
                className="px-4 py-2 border rounded-lg hover:bg-gray-50 transition-colors disabled:opacity-50"
              >
                取消
              </button>
              <button
                onClick={confirmDeleteTable}
                disabled={deletingTable}
                className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors disabled:opacity-50"
              >
                {deletingTable ? '删除中...' : '确认删除'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
