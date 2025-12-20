import { Outlet } from 'react-router-dom';
import Sidebar from './Sidebar';
import SystemMessages from '../SystemMessages';

export default function Layout() {
  return (
    <div className="flex h-screen bg-gray-50">
      <Sidebar />
      <main className="flex-1 overflow-y-auto md:ml-64">
        {/* Header with System Messages */}
        <div className="sticky top-0 z-30 bg-white border-b border-gray-200 px-6 py-3">
          <div className="flex items-center justify-end">
            <SystemMessages />
          </div>
        </div>
        <div className="container mx-auto p-6">
          <Outlet />
        </div>
      </main>
    </div>
  );
}