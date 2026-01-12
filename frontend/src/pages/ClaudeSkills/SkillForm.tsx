import { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { ArrowLeft, Save, Loader2, Plus, X } from 'lucide-react';
import { skillsApi } from '@/services';
import type { CreateSkillRequest, UpdateSkillRequest, SkillToolParameterDef } from '@/types';
import { logger } from '@/lib/logger';
import { showSuccess, showError } from '@/lib/toast';

export default function SkillForm() {
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const isEdit = !!id;

  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [formData, setFormData] = useState<CreateSkillRequest>({
    name: '',
    description: '',
    version: '1.0.0',
    author: '',
    category: '',
    tags: '',
    runtime_type: 'python',
    runtime_version: '3.9+',
    requirements: [],
    tool_name: '',
    tool_description: '',
    tool_parameters: {},
    script: '',
    is_public: false,
  });

  // 参数编辑状态
  const [paramName, setParamName] = useState('');
  const [paramDef, setParamDef] = useState<SkillToolParameterDef>({
    type: 'string',
    required: false,
    description: '',
  });

  // 依赖编辑状态
  const [newRequirement, setNewRequirement] = useState('');

  useEffect(() => {
    if (isEdit && id) {
      fetchSkill(id);
    }
  }, [id, isEdit]);

  const fetchSkill = async (skillId: string) => {
    try {
      setLoading(true);
      const skill = await skillsApi.get(skillId);
      setFormData({
        name: skill.name,
        description: skill.description,
        version: skill.version,
        author: skill.author,
        category: skill.category || '',
        tags: skill.tags || '',
        runtime_type: skill.runtime_type,
        runtime_version: skill.runtime_version,
        requirements: skill.requirements || [],
        tool_name: skill.tool_name,
        tool_description: skill.tool_description,
        tool_parameters: skill.tool_parameters || {},
        script: skill.script,
        is_public: skill.is_public,
      });
    } catch (error) {
      logger.error('Failed to fetch skill:', error);
      showError('获取 Skill 信息失败');
      navigate('/claude-skills');
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    // 验证必填字段
    if (!formData.name || !formData.tool_name || !formData.script) {
      showError('请填写所有必填字段');
      return;
    }

    try {
      setSaving(true);
      if (isEdit && id) {
        await skillsApi.update(id, formData as UpdateSkillRequest);
        showSuccess('更新成功');
      } else {
        await skillsApi.create(formData);
        showSuccess('创建成功');
      }
      navigate('/claude-skills');
    } catch (error) {
      logger.error('Failed to save skill:', error);
      showError(isEdit ? '更新失败' : '创建失败');
    } finally {
      setSaving(false);
    }
  };

  const handleAddParameter = () => {
    if (!paramName.trim()) {
      showError('请输入参数名称');
      return;
    }
    if (formData.tool_parameters && formData.tool_parameters[paramName]) {
      showError('参数名称已存在');
      return;
    }

    setFormData({
      ...formData,
      tool_parameters: {
        ...(formData.tool_parameters || {}),
        [paramName]: paramDef,
      },
    });

    // 重置表单
    setParamName('');
    setParamDef({
      type: 'string',
      required: false,
      description: '',
    });
  };

  const handleRemoveParameter = (name: string) => {
    const newParams = { ...(formData.tool_parameters || {}) };
    delete newParams[name];
    setFormData({
      ...formData,
      tool_parameters: newParams,
    });
  };

  const handleAddRequirement = () => {
    if (!newRequirement.trim()) return;
    if ((formData.requirements || []).includes(newRequirement)) {
      showError('依赖已存在');
      return;
    }

    setFormData({
      ...formData,
      requirements: [...(formData.requirements || []), newRequirement],
    });
    setNewRequirement('');
  };

  const handleRemoveRequirement = (req: string) => {
    setFormData({
      ...formData,
      requirements: (formData.requirements || []).filter(r => r !== req),
    });
  };

  if (loading) {
    return (
      <div className="h-screen flex items-center justify-center">
        <Loader2 className="w-8 h-8 animate-spin text-indigo-500" />
      </div>
    );
  }

  return (
    <div className="h-screen flex flex-col bg-gray-50">
      {/* Header */}
      <div className="bg-white border-b px-6 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <button
              onClick={() => navigate('/claude-skills')}
              className="flex items-center gap-2 text-gray-600 hover:text-gray-900"
            >
              <ArrowLeft className="w-5 h-5" />
              返回
            </button>
            <div>
              <h1 className="text-2xl font-bold text-gray-900">
                {isEdit ? '编辑 Skill' : '创建 Skill'}
              </h1>
              <p className="text-sm text-gray-500 mt-1">
                {isEdit ? '修改 Skill 配置和代码' : '创建一个新的自定义 Skill'}
              </p>
            </div>
          </div>
          <button
            onClick={handleSubmit}
            disabled={saving}
            className="flex items-center gap-2 px-4 py-2 bg-indigo-500 text-white rounded-lg hover:bg-indigo-600 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {saving ? (
              <>
                <Loader2 className="w-5 h-5 animate-spin" />
                保存中...
              </>
            ) : (
              <>
                <Save className="w-5 h-5" />
                保存
              </>
            )}
          </button>
        </div>
      </div>

      {/* Form */}
      <div className="flex-1 overflow-auto p-6">
        <form onSubmit={handleSubmit} className="max-w-4xl mx-auto space-y-6">
          {/* 基本信息 */}
          <div className="bg-white rounded-lg border p-6">
            <h2 className="text-lg font-semibold text-gray-900 mb-4">基本信息</h2>
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Skill 名称 <span className="text-red-500">*</span>
                  </label>
                  <input
                    type="text"
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
                    placeholder="例如: Data Analysis Helper"
                    required
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    版本号
                  </label>
                  <input
                    type="text"
                    value={formData.version}
                    onChange={(e) => setFormData({ ...formData, version: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
                    placeholder="1.0.0"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  描述
                </label>
                <textarea
                  value={formData.description}
                  onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  rows={3}
                  placeholder="简要描述这个 Skill 的功能..."
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    作者
                  </label>
                  <input
                    type="text"
                    value={formData.author}
                    onChange={(e) => setFormData({ ...formData, author: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
                    placeholder="您的名字"
                  />
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    分类
                  </label>
                  <input
                    type="text"
                    value={formData.category}
                    onChange={(e) => setFormData({ ...formData, category: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
                    placeholder="例如: data_analysis"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  标签
                </label>
                <input
                  type="text"
                  value={formData.tags}
                  onChange={(e) => setFormData({ ...formData, tags: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  placeholder="用逗号分隔，例如: python,pandas,data"
                />
              </div>

              <div className="flex items-center">
                <input
                  type="checkbox"
                  id="is_public"
                  checked={formData.is_public}
                  onChange={(e) => setFormData({ ...formData, is_public: e.target.checked })}
                  className="rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
                />
                <label htmlFor="is_public" className="ml-2 text-sm text-gray-700">
                  公开 Skill（允许其他用户使用）
                </label>
              </div>
            </div>
          </div>

          {/* 运行时配置 */}
          <div className="bg-white rounded-lg border p-6">
            <h2 className="text-lg font-semibold text-gray-900 mb-4">运行时配置</h2>
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    运行时类型 <span className="text-red-500">*</span>
                  </label>
                  <select
                    value={formData.runtime_type}
                    onChange={(e) => setFormData({ ...formData, runtime_type: e.target.value as 'python' | 'node' | 'shell' })}
                    className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  >
                    <option value="python">Python</option>
                    <option value="node">Node.js</option>
                    <option value="shell">Shell</option>
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    运行时版本
                  </label>
                  <input
                    type="text"
                    value={formData.runtime_version}
                    onChange={(e) => setFormData({ ...formData, runtime_version: e.target.value })}
                    className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
                    placeholder="例如: 3.9+"
                  />
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  依赖包
                </label>
                <div className="space-y-2">
                  <div className="flex gap-2">
                    <input
                      type="text"
                      value={newRequirement}
                      onChange={(e) => setNewRequirement(e.target.value)}
                      onKeyPress={(e) => e.key === 'Enter' && (e.preventDefault(), handleAddRequirement())}
                      className="flex-1 px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
                      placeholder="例如: pandas==2.0.0"
                    />
                    <button
                      type="button"
                      onClick={handleAddRequirement}
                      className="px-4 py-2 bg-indigo-500 text-white rounded-lg hover:bg-indigo-600"
                    >
                      <Plus className="w-5 h-5" />
                    </button>
                  </div>
                  {(formData.requirements || []).length > 0 && (
                    <div className="border rounded-lg p-3 space-y-2">
                      {(formData.requirements || []).map((req, index) => (
                        <div key={index} className="flex items-center justify-between bg-gray-50 px-3 py-2 rounded">
                          <span className="text-sm text-gray-700">{req}</span>
                          <button
                            type="button"
                            onClick={() => handleRemoveRequirement(req)}
                            className="text-red-600 hover:text-red-700"
                          >
                            <X className="w-4 h-4" />
                          </button>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>

          {/* 工具定义 */}
          <div className="bg-white rounded-lg border p-6">
            <h2 className="text-lg font-semibold text-gray-900 mb-4">工具定义</h2>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  工具名称 <span className="text-red-500">*</span>
                </label>
                <input
                  type="text"
                  value={formData.tool_name}
                  onChange={(e) => setFormData({ ...formData, tool_name: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  placeholder="例如: analyze_data"
                  required
                />
                <p className="text-xs text-gray-500 mt-1">
                  这是 LLM 调用工具时使用的名称，建议使用小写字母和下划线
                </p>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  工具描述
                </label>
                <textarea
                  value={formData.tool_description}
                  onChange={(e) => setFormData({ ...formData, tool_description: e.target.value })}
                  className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500"
                  rows={2}
                  placeholder="描述工具的功能，帮助 LLM 理解何时使用这个工具..."
                />
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  工具参数
                </label>
                <div className="space-y-3">
                  {/* 参数添加表单 */}
                  <div className="border rounded-lg p-4 bg-gray-50">
                    <div className="grid grid-cols-2 gap-3 mb-3">
                      <div>
                        <label className="block text-xs font-medium text-gray-600 mb-1">
                          参数名称
                        </label>
                        <input
                          type="text"
                          value={paramName}
                          onChange={(e) => setParamName(e.target.value)}
                          className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 text-sm"
                          placeholder="例如: file_path"
                        />
                      </div>
                      <div>
                        <label className="block text-xs font-medium text-gray-600 mb-1">
                          参数类型
                        </label>
                        <select
                          value={paramDef.type}
                          onChange={(e) => setParamDef({ ...paramDef, type: e.target.value as any })}
                          className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 text-sm"
                        >
                          <option value="string">string</option>
                          <option value="number">number</option>
                          <option value="boolean">boolean</option>
                          <option value="array">array</option>
                          <option value="object">object</option>
                        </select>
                      </div>
                    </div>
                    <div className="mb-3">
                      <label className="block text-xs font-medium text-gray-600 mb-1">
                        参数描述
                      </label>
                      <input
                        type="text"
                        value={paramDef.description}
                        onChange={(e) => setParamDef({ ...paramDef, description: e.target.value })}
                        className="w-full px-3 py-2 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 text-sm"
                        placeholder="描述这个参数的用途..."
                      />
                    </div>
                    <div className="flex items-center justify-between">
                      <label className="flex items-center">
                        <input
                          type="checkbox"
                          checked={paramDef.required}
                          onChange={(e) => setParamDef({ ...paramDef, required: e.target.checked })}
                          className="rounded border-gray-300 text-indigo-600 focus:ring-indigo-500"
                        />
                        <span className="ml-2 text-sm text-gray-700">必填参数</span>
                      </label>
                      <button
                        type="button"
                        onClick={handleAddParameter}
                        className="px-3 py-1.5 bg-indigo-500 text-white text-sm rounded hover:bg-indigo-600"
                      >
                        添加参数
                      </button>
                    </div>
                  </div>

                  {/* 已添加的参数列表 */}
                  {Object.keys(formData.tool_parameters || {}).length > 0 && (
                    <div className="border rounded-lg divide-y">
                      {Object.entries(formData.tool_parameters || {}).map(([name, def]) => (
                        <div key={name} className="p-3 hover:bg-gray-50">
                          <div className="flex items-start justify-between">
                            <div className="flex-1">
                              <div className="flex items-center gap-2">
                                <span className="font-medium text-sm text-gray-900">{name}</span>
                                <span className="text-xs bg-blue-100 text-blue-800 px-2 py-0.5 rounded">
                                  {def.type}
                                </span>
                                {def.required && (
                                  <span className="text-xs bg-red-100 text-red-800 px-2 py-0.5 rounded">
                                    必填
                                  </span>
                                )}
                              </div>
                              {def.description && (
                                <p className="text-xs text-gray-500 mt-1">{def.description}</p>
                              )}
                            </div>
                            <button
                              type="button"
                              onClick={() => handleRemoveParameter(name)}
                              className="text-red-600 hover:text-red-700 ml-2"
                            >
                              <X className="w-4 h-4" />
                            </button>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>

          {/* 代码编辑 */}
          <div className="bg-white rounded-lg border p-6">
            <h2 className="text-lg font-semibold text-gray-900 mb-4">
              代码 <span className="text-red-500">*</span>
            </h2>
            <div>
              <textarea
                value={formData.script}
                onChange={(e) => setFormData({ ...formData, script: e.target.value })}
                className="w-full px-4 py-3 border rounded-lg focus:outline-none focus:ring-2 focus:ring-indigo-500 font-mono text-sm"
                rows={20}
                placeholder={
                  formData.runtime_type === 'python'
                    ? '# Python 代码示例\nimport pandas as pd\n\ndef execute(**kwargs):\n    # 你的代码逻辑\n    return {"result": "success"}'
                    : formData.runtime_type === 'node'
                    ? '// Node.js 代码示例\nmodule.exports = async function execute(args) {\n    // 你的代码逻辑\n    return { result: "success" };\n};'
                    : '#!/bin/bash\n# Shell 脚本示例\necho "Hello World"'
                }
                required
              />
              <p className="text-xs text-gray-500 mt-2">
                {formData.runtime_type === 'python' && '代码必须包含一个 execute 函数，接收参数并返回结果'}
                {formData.runtime_type === 'node' && '代码必须导出一个 execute 函数，接收参数并返回结果'}
                {formData.runtime_type === 'shell' && 'Shell 脚本将直接执行'}
              </p>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
}