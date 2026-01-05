import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Database, Table, Settings } from 'lucide-react';
import { nl2sqlApi } from '@/services';
import { showError } from '@/lib/toast';
import TablesTab from './tabs/TablesTab';
import SettingsTab from './tabs/SettingsTab';

type TabType = 'tables' | 'settings';

interface DataSource {
  id: string;
  name: string;
  type: string;
  db_type?: string;
  status: string;
  create_time: string;
  embedding_model_id: string;
}

export default function NL2SQLDataSourceDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<TabType>('tables');
  const [datasource, setDatasource] = useState<DataSource | null>(null);
  const [displayName, setDisplayName] = useState<string>('');
  const [loading, setLoading] = useState(false);
  const [parseStatus, setParseStatus] = useState<{ parsed: number; total: number } | null>(null);

  useEffect(() => {
    if (id) {
      fetchDatasource();
      fetchParseStatus();
    }
  }, [id]);

  const fetchDatasource = async () => {
    if (!id) return;

    try {
      setLoading(true);
      const response = await nl2sqlApi.listDatasources();
      const ds = response.list?.find((d: DataSource) => d.id === id);
      if (ds) {
        setDatasource(ds);
      } else {
        showError('数据源不存在');
        navigate('/nl2sql-datasource');
      }
    } catch (error) {
      console.error('Failed to fetch datasource:', error);
      showError('获取数据源信息失败');
      navigate('/nl2sql-datasource');
    } finally {
      setLoading(false);
    }
  };

  const fetchParseStatus = async () => {
    if (!id) return;
    try {
      const schema = await nl2sqlApi.getSchema(id);
      const parsed = schema.tables?.filter((t: any) => t.parsed).length || 0;
      const total = schema.tables?.length || 0;
      setParseStatus({ parsed, total });

      // 从第一个表获取 display_name
      if (schema.tables && schema.tables.length > 0) {
        const firstTable = schema.tables[0];
        setDisplayName(firstTable.display_name || firstTable.table_name);
      }
    } catch (error) {
      // 如果获取失败，不影响主流程
      console.error('Failed to fetch parse status:', error);
    }
  };

  const tabs = [
    { key: 'tables' as TabType, label: '表管理', icon: Table },
    { key: 'settings' as TabType, label: '设置', icon: Settings },
  ];

  if (loading || !datasource) {
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
          onClick={() => navigate('/nl2sql-datasource')}
          className="flex items-center gap-2 text-sm text-gray-600 hover:text-gray-900 mb-3"
        >
          <ArrowLeft className="w-4 h-4" />
          返回数据源列表
        </button>

        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Database className="w-6 h-6 text-blue-500" />
            <div>
              <h1 className="text-2xl font-bold text-gray-900">{displayName}</h1>
              <p className="mt-1 text-sm text-gray-600">
                {datasource.type === 'jdbc'
                  ? `JDBC数据源 (${datasource.db_type})`
                  : `文件数据源 (${datasource.type.toUpperCase()})`}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            {parseStatus && (
              <span
                className={`px-3 py-1 text-sm rounded ${
                  parseStatus.total > 0 && parseStatus.parsed === parseStatus.total
                    ? 'bg-green-100 text-green-700'
                    : parseStatus.parsed > 0
                    ? 'bg-blue-100 text-blue-700'
                    : 'bg-yellow-100 text-yellow-700'
                }`}
              >
                {parseStatus.total === 0
                  ? '无表'
                  : parseStatus.parsed === parseStatus.total
                  ? '已解析'
                  : `${parseStatus.parsed}/${parseStatus.total} 已解析`}
              </span>
            )}
            <span
              className={`px-3 py-1 text-sm rounded ${
                datasource.status === 'active'
                  ? 'bg-green-100 text-green-700'
                  : 'bg-gray-100 text-gray-700'
              }`}
            >
              {datasource.status === 'active' ? '活跃' : datasource.status}
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
        {activeTab === 'tables' && <TablesTab datasource={datasource} onUpdate={fetchDatasource} />}
        {activeTab === 'settings' && <SettingsTab datasource={datasource} onUpdate={fetchDatasource} />}
      </div>
    </div>
  );
}