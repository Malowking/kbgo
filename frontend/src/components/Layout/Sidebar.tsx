import { Link, useLocation } from 'react-router-dom';
import {
  Database,
  MessageSquare,
  Settings,
  Menu,
  X,
  Plug,
  Bot,
  HardDrive,
  Table,
  ChevronDown,
  ChevronRight
} from 'lucide-react';
import { useAppStore } from '@/store';
import { useState } from 'react';

interface NavItem {
  name: string;
  href?: string;
  icon: any;
  children?: NavItem[];
}

const navigation: NavItem[] = [
  {
    name: '数据源',
    icon: HardDrive,
    children: [
      { name: '知识库', href: '/knowledge-base', icon: Database },
      { name: 'NL2SQL', href: '/nl2sql-datasource', icon: Table },
    ],
  },
  { name: '对话', href: '/chat', icon: MessageSquare },
  { name: 'Agent构建', href: '/agent-builder', icon: Bot },
  { name: 'Agent对话', href: '/agent-chat', icon: Bot },
  { name: 'MCP服务', href: '/mcp', icon: Plug },
  { name: '模型管理', href: '/models', icon: Settings },
];

export default function Sidebar() {
  const location = useLocation();
  const { sidebarOpen, toggleSidebar } = useAppStore();
  const [expandedItems, setExpandedItems] = useState<string[]>(['数据源']);

  const toggleExpand = (name: string) => {
    setExpandedItems(prev =>
      prev.includes(name) ? prev.filter(item => item !== name) : [...prev, name]
    );
  };

  const renderNavItem = (item: NavItem) => {
    const Icon = item.icon;
    const hasChildren = item.children && item.children.length > 0;
    const isExpanded = expandedItems.includes(item.name);
    const isActive = item.href && location.pathname.startsWith(item.href);

    if (hasChildren) {
      return (
        <div key={item.name}>
          <button
            onClick={() => toggleExpand(item.name)}
            className="w-full flex items-center justify-between px-4 py-3 text-sm font-medium rounded-lg text-gray-700 hover:bg-gray-100 transition-colors duration-200"
          >
            <div className="flex items-center">
              <Icon className="w-5 h-5 mr-3" />
              {item.name}
            </div>
            {isExpanded ? (
              <ChevronDown className="w-4 h-4" />
            ) : (
              <ChevronRight className="w-4 h-4" />
            )}
          </button>
          {isExpanded && item.children && (
            <div className="ml-4 mt-1 space-y-1">
              {item.children.map((child) => {
                const ChildIcon = child.icon;
                const childActive = child.href && location.pathname.startsWith(child.href);
                return (
                  <Link
                    key={child.name}
                    to={child.href!}
                    className={`
                      flex items-center px-4 py-2 text-sm font-medium rounded-lg
                      transition-colors duration-200
                      ${
                        childActive
                          ? 'bg-primary-50 text-primary-700'
                          : 'text-gray-600 hover:bg-gray-100'
                      }
                    `}
                  >
                    <ChildIcon className="w-4 h-4 mr-3" />
                    {child.name}
                  </Link>
                );
              })}
            </div>
          )}
        </div>
      );
    }

    return (
      <Link
        key={item.name}
        to={item.href!}
        className={`
          flex items-center px-4 py-3 text-sm font-medium rounded-lg
          transition-colors duration-200
          ${
            isActive
              ? 'bg-primary-50 text-primary-700'
              : 'text-gray-700 hover:bg-gray-100'
          }
        `}
      >
        <Icon className="w-5 h-5 mr-3" />
        {item.name}
      </Link>
    );
  };

  return (
    <>
      {/* Mobile menu button */}
      <button
        onClick={toggleSidebar}
        className="fixed top-4 left-4 z-50 md:hidden p-2 rounded-lg bg-white shadow-lg"
      >
        {sidebarOpen ? <X size={24} /> : <Menu size={24} />}
      </button>

      {/* Sidebar */}
      <aside
        className={`
          fixed inset-y-0 left-0 z-40 w-64 bg-white border-r border-gray-200
          transform transition-transform duration-200 ease-in-out
          ${sidebarOpen ? 'translate-x-0' : '-translate-x-full'}
          md:translate-x-0
        `}
      >
        <div className="flex flex-col h-full">
          {/* Logo */}
          <div className="flex items-center h-16 px-6 border-b border-gray-200">
            <Database className="w-8 h-8 text-primary-600" />
            <h1 className="ml-3 text-xl font-bold text-gray-900">KBGO</h1>
          </div>

          {/* Navigation */}
          <nav className="flex-1 px-4 py-6 space-y-2 overflow-y-auto">
            {navigation.map(renderNavItem)}
          </nav>

          {/* Footer */}
          <div className="p-4 border-t border-gray-200">
            <p className="text-xs text-gray-500 text-center">
              KBGO v1.0.0
            </p>
          </div>
        </div>
      </aside>

      {/* Overlay for mobile */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-30 bg-black bg-opacity-50 md:hidden"
          onClick={toggleSidebar}
        />
      )}
    </>
  );
}