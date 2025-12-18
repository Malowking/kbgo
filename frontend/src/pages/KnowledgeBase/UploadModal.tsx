import { useState } from 'react';
import { X, Upload, Link as LinkIcon } from 'lucide-react';
import { documentApi } from '@/services';

interface UploadModalProps {
  kbId: string;
  onClose: () => void;
  onSuccess: (documentIds: string[]) => void;
}

export default function UploadModal({ kbId, onClose, onSuccess }: UploadModalProps) {
  const [uploadMode, setUploadMode] = useState<'file' | 'url'>('file');
  const [files, setFiles] = useState<File[]>([]);
  const [urls, setUrls] = useState<string>('');
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

      // 上传文档 - 后端只支持单文件上传，需要逐个上传
      const documentIds: string[] = [];

      if (uploadMode === 'file') {
        // 逐个上传文件
        for (const file of files) {
          const formData = new FormData();
          formData.append('knowledge_id', kbId);
          formData.append('file', file); // 使用 'file' (singular) 参数名

          try {
            const uploadResult = await documentApi.upload(formData);
            if (uploadResult.document_id) {
              documentIds.push(uploadResult.document_id);
            }
          } catch (error) {
            console.error(`Failed to upload ${file.name}:`, error);
            alert(`上传文件 ${file.name} 失败`);
            // 继续上传其他文件
          }
        }
      } else {
        // 逐个上传URL
        const urlList = urls.split('\n').filter(u => u.trim());
        for (const url of urlList) {
          const formData = new FormData();
          formData.append('knowledge_id', kbId);
          formData.append('url', url.trim()); // 使用 'url' (singular) 参数名

          try {
            const uploadResult = await documentApi.upload(formData);
            if (uploadResult.document_id) {
              documentIds.push(uploadResult.document_id);
            }
          } catch (error) {
            console.error(`Failed to upload URL ${url}:`, error);
            alert(`上传 URL ${url} 失败`);
            // 继续上传其他URL
          }
        }
      }

      if (documentIds.length > 0) {
        alert(`成功上传 ${documentIds.length} 个文档，请继续进行索引`);
        onSuccess(documentIds); // 返回上传成功的文档 ID
        onClose();
      } else {
        alert('上传失败，请重试');
      }
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

          {/* 提示信息 */}
          <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
            <p className="text-sm text-yellow-800">
              注意：上传文档后，需要进行索引才能在检索中使用
            </p>
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
              {uploading ? '上传中...' : '上传文档'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}