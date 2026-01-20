import { X } from 'lucide-react';
import type { AgentConfig, KnowledgeBase, Model, MCPRegistry } from '@/types';
import ToolConfigurationPanel from './ToolConfigurationPanel';

interface ToolConfigurationModalProps {
  config: AgentConfig;
  onConfigChange: (config: AgentConfig) => void;
  kbList: KnowledgeBase[];
  rerankModels: Model[];
  mcpServices: MCPRegistry[];
  nl2sqlDatasources: any[];
  onClose: () => void;
}

export default function ToolConfigurationModal({
  config,
  onConfigChange,
  kbList,
  rerankModels,
  mcpServices,
  nl2sqlDatasources,
  onClose,
}: ToolConfigurationModalProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-4xl max-h-[90vh] flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b">
          <h2 className="text-xl font-semibold">工具配置</h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600 transition-colors"
          >
            <X className="w-6 h-6" />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-auto p-6">
          <ToolConfigurationPanel
            config={config}
            onConfigChange={onConfigChange}
            kbList={kbList}
            rerankModels={rerankModels}
            mcpServices={mcpServices}
            nl2sqlDatasources={nl2sqlDatasources}
          />
        </div>

        {/* Footer */}
        <div className="flex justify-end gap-3 p-6 border-t">
          <button
            onClick={onClose}
            className="px-4 py-2 bg-blue-500 text-white rounded-lg hover:bg-blue-600 transition-colors"
          >
            完成
          </button>
        </div>
      </div>
    </div>
  );
}