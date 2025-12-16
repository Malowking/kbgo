import { useState } from 'react';
import { X, Upload, Link as LinkIcon } from 'lucide-react';
import { documentApi } from '@/services';

interface UploadModalProps {
  kbId: string;
  onClose: () => void;
  onSuccess: () => void;
}

export default function UploadModal({ kbId, onClose, onSuccess }: UploadModalProps) {
  const [uploadMode, setUploadMode] = useState<'file' | 'url'>('file');
  const [files, setFiles] = useState<File[]>([]);
  const [urls, setUrls] = useState<string>('');
  const [chunkSize, setChunkSize] = useState(500);
  const [chunkOverlap, setChunkOverlap] = useState(50);
  const [uploading, setUploading] = useState(false);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files) {
      setFiles(Array.from(e.target.files));
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (uploadMode === 'file' && files.length === 0) {
      alert('请选择文件');
      return;
    }

    if (uploadMode === 'url' && !urls.trim()) {
      alert('请输入 URL');
      return;
    }

    try {
      setUploading(true);

      // 上传文档
      const formData = new FormData();
      formData.append('kb_id', kbId);

      if (uploadMode === 'file') {
        files.forEach((file) => {
          formData.append('files', file);
        });
      } else {
        const urlList = urls.split('\n').filter(u => u.trim());
        urlList.forEach((url) => {
          formData.append('urls', url.trim());
        });
      }

      const uploadResult = await documentApi.upload(formData);

      // 索引文档
      if (uploadResult.document_ids && uploadResult.document_ids.length > 0) {
        await documentApi.index({
          document_ids: uploadResult.document_ids,
          chunk_size: chunkSize,
          chunk_overlap: chunkOverlap,
        });
      }

      alert('文档上传并索引成功');
      onSuccess(); // 先刷新列表
      onClose(); // 再关闭模态框
    } catch (error) {
      console.error('Failed to upload documents:', error);
      alert('上传失败');
    } finally {
      setUploading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-2xl mx-4 max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b border-gray-200">
          <h2 className="text-xl font-semibold text-gray-900">
            上传文档
          </h2>
          <button
            onClick={onClose}
            className="p-1 rounded hover:bg-gray-100"
          >
            <X className="w-5 h-5 text-gray-500" />
          </button>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="p-6 space-y-6">
          {/* Upload Mode */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              上传方式
            </label>
            <div className="flex space-x-4">
              <button
                type="button"
                onClick={() => setUploadMode('file')}
                className={`flex-1 py-2 px-4 rounded-lg border-2 transition-colors ${
                  uploadMode === 'file'
                    ? 'border-primary-600 bg-primary-50 text-primary-700'
                    : 'border-gray-200 text-gray-600 hover:border-gray-300'
                }`}
              >
                <Upload className="w-5 h-5 mx-auto mb-1" />
                文件上传
              </button>
              <button
                type="button"
                onClick={() => setUploadMode('url')}
                className={`flex-1 py-2 px-4 rounded-lg border-2 transition-colors ${
                  uploadMode === 'url'
                    ? 'border-primary-600 bg-primary-50 text-primary-700'
                    : 'border-gray-200 text-gray-600 hover:border-gray-300'
                }`}
              >
                <LinkIcon className="w-5 h-5 mx-auto mb-1" />
                URL 导入
              </button>
            </div>
          </div>

          {/* File Upload */}
          {uploadMode === 'file' && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                选择文件 <span className="text-red-500">*</span>
              </label>
              <input
                type="file"
                multiple
                onChange={handleFileChange}
                className="input"
                accept=".pdf,.doc,.docx,.txt,.md"
              />
              {files.length > 0 && (
                <div className="mt-2 space-y-1">
                  {files.map((file, idx) => (
                    <div key={idx} className="text-sm text-gray-600">
                      {file.name} ({(file.size / 1024).toFixed(2)} KB)
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* URL Input */}
          {uploadMode === 'url' && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                输入 URL <span className="text-red-500">*</span>
              </label>
              <textarea
                value={urls}
                onChange={(e) => setUrls(e.target.value)}
                className="input"
                placeholder="每行一个 URL&#10;https://example.com/doc1.pdf&#10;https://example.com/doc2.pdf"
                rows={6}
              />
            </div>
          )}

          {/* Chunk Settings */}
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                分块大小
              </label>
              <input
                type="number"
                value={chunkSize}
                onChange={(e) => setChunkSize(Number(e.target.value))}
                className="input"
                min="100"
                max="2000"
              />
              <p className="mt-1 text-xs text-gray-500">
                默认 500 个字符
              </p>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                分块重叠
              </label>
              <input
                type="number"
                value={chunkOverlap}
                onChange={(e) => setChunkOverlap(Number(e.target.value))}
                className="input"
                min="0"
                max="500"
              />
              <p className="mt-1 text-xs text-gray-500">
                默认 50 个字符
              </p>
            </div>
          </div>

          {/* Actions */}
          <div className="flex justify-end space-x-3 pt-4 border-t border-gray-200">
            <button
              type="button"
              onClick={onClose}
              className="btn btn-secondary"
              disabled={uploading}
            >
              取消
            </button>
            <button
              type="submit"
              className="btn btn-primary"
              disabled={uploading}
            >
              {uploading ? '上传中...' : '上传并索引'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}