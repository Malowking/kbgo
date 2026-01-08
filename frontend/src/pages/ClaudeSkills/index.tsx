import { useState, useEffect } from 'react';
import { Plus, Edit, Trash2, Code2, Loader2, Search, ArrowLeft } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { skillsApi } from '@/services';
import type { SkillItem } from '@/types';
import { logger } from '@/lib/logger';
import { showSuccess, showError } from '@/lib/toast';

export default function ClaudeSkills() {
  const navigate = useNavigate();
  const [skills, setSkills] = useState<SkillItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [filterStatus, setFilterStatus] = useState<0 | 1 | undefined>(undefined);

  useEffect(() => {
    fetchSkills();
  }, [searchKeyword, filterStatus]);

  const fetchSkills = async () => {
    try {
      setLoading(true);
      const response = await skillsApi.list({
        keyword: searchKeyword || undefined,
        status: filterStatus,
        page: 1,
        page_size: 100,
      });
      setSkills(response.list || []);
    } catch (error) {
      logger.error('Failed to fetch skills:', error);
      showError('获取 Skills 列表失败');
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: string, name: string) => {
    if (!confirm(`确定要删除 Skill "${name}" 吗？`)) return;

    try {
      await skillsApi.delete(id);
      showSuccess('删除成功');
      fetchSkills();
    } catch (error) {
      logger.error('Failed to delete skill:', error);
      showError('删除失败');
    }
  };

  const handleToggleStatus = async (skill: SkillItem) => {
    try {
      const newStatus = skill.status === 1 ? 0 : 1;
      await skillsApi.update(skill.id, { status: newStatus });
      showSuccess(newStatus === 1 ? '已启用' : '已禁用');
      fetchSkills();
    } catch (error) {
      logger.error('Failed to toggle status:', error);
      showError('操作失败');
    }
  };

  const getRuntimeColor = (runtime: string) => {
    switch (runtime) {
      case 'python': return 'bg-blue-100 text-blue-800';
      case 'node': return 'bg-green-100 text-green-800';
      case 'shell': return 'bg-gray-100 text-gray-800';
      default: return 'bg-gray-100 text-gray-800';
    }
  };

  return (
    <div className="h-screen flex flex-col bg-gray-50">
      {/* Header */}
      <div className="bg-white border-b px-6 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <button
              onClick={() => navigate('/agent-builder')}
              className="flex items-center gap-2 text-gray-600 hover:text-gray-900"
            >
              <ArrowLeft className="w-5 h-5" />
              返回
            </button>
            <div>
              <h1 className="text-2xl font-bold text-gray-900">Claude Skills</h1>
              <p className="text-sm text-gray-500 mt-1">管理自定义 Python/Node.js Skills</p>
            </div>
          </div>
          <button
            onClick={() => navigate('/claude-skills/create')}
            className="flex items-center gap-2 px-4 py-2 bg-indigo-500 text-white rounded-lg hover:bg-indigo-600"
          >
            <Plus className="w-5 h-5" />
            创建 Skill
          </button>
        </div>
      </div>

      {/* Filters */}
      <div className="bg-white border-b px-6 py-3">
        <div className="flex items-center gap-3">
          <div className="flex-1 relative">
            <Search className="w-5 h-5 absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400" />
            <input
              type="text"
              placeholder="搜索 Skills..."
              value={searchKeyword}
              onChange={(e) => setSearchKeyword(e.target.value)}
              className="w-full pl-10 pr-4 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
            />
          </div>
          <select
            value={filterStatus ?? ''}
            onChange={(e) => setFilterStatus(e.target.value === '' ? undefined : parseInt(e.target.value) as 0 | 1)}
            className="px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
          >
            <option value="">全部状态</option>
            <option value="1">已启用</option>
            <option value="0">已禁用</option>
          </select>
        </div>
      </div>

      {/* Skills Grid */}
      <div className="flex-1 overflow-auto p-6">
        {loading ? (
          <div className="flex items-center justify-center h-64">
            <Loader2 className="w-8 h-8 animate-spin text-indigo-500" />
          </div>
        ) : skills.length === 0 ? (
          <div className="text-center py-12">
            <Code2 className="w-16 h-16 text-gray-300 mx-auto mb-4" />
            <p className="text-gray-500 mb-2">暂无 Skills</p>
            <p className="text-sm text-gray-400 mb-4">创建您的第一个 Skill 开始使用</p>
            <button
              onClick={() => navigate('/claude-skills/create')}
              className="inline-flex items-center gap-2 px-4 py-2 bg-indigo-500 text-white rounded-lg hover:bg-indigo-600"
            >
              <Plus className="w-5 h-5" />
              创建 Skill
            </button>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {skills.map((skill) => (
              <div key={skill.id} className="bg-white rounded-lg border p-4 hover:shadow-md transition-shadow">
                <div className="flex items-start justify-between mb-3">
                  <div className="flex-1">
                    <h3 className="font-medium text-gray-900 mb-1">{skill.name}</h3>
                    <p className="text-sm text-gray-500 line-clamp-2">{skill.description}</p>
                  </div>
                  <button
                    onClick={() => handleToggleStatus(skill)}
                    className={`ml-2 text-xs px-2 py-1 rounded ${
                      skill.status === 1
                        ? 'bg-green-100 text-green-800'
                        : 'bg-gray-100 text-gray-800'
                    }`}
                  >
                    {skill.status === 1 ? '启用' : '禁用'}
                  </button>
                </div>

                <div className="flex items-center gap-2 mb-3">
                  <span className={`text-xs px-2 py-0.5 rounded ${getRuntimeColor(skill.runtime_type)}`}>
                    {skill.runtime_type}
                  </span>
                  {skill.category && (
                    <span className="text-xs bg-gray-100 text-gray-700 px-2 py-0.5 rounded">
                      {skill.category}
                    </span>
                  )}
                </div>

                {skill.call_count > 0 && (
                  <div className="text-xs text-gray-500 mb-3 space-y-1">
                    <div>调用次数: {skill.call_count}</div>
                    <div>成功率: {((skill.success_count / skill.call_count) * 100).toFixed(1)}%</div>
                    {skill.avg_duration > 0 && (
                      <div>平均耗时: {skill.avg_duration}ms</div>
                    )}
                  </div>
                )}

                <div className="flex items-center gap-2 pt-3 border-t">
                  <button
                    onClick={() => navigate(`/claude-skills/edit/${skill.id}`)}
                    className="flex-1 flex items-center justify-center gap-1 px-3 py-1.5 text-sm text-indigo-600 hover:bg-indigo-50 rounded transition-colors"
                  >
                    <Edit className="w-4 h-4" />
                    编辑
                  </button>
                  <button
                    onClick={() => handleDelete(skill.id, skill.name)}
                    className="flex-1 flex items-center justify-center gap-1 px-3 py-1.5 text-sm text-red-600 hover:bg-red-50 rounded transition-colors"
                  >
                    <Trash2 className="w-4 h-4" />
                    删除
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
