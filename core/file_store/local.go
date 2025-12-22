package file_store

import (
	"context"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/Malowking/kbgo/core/errors"
	"github.com/gogf/gf/v2/frame/g"
)

// SaveFileToLocal 保存文件到本地存储
func SaveFileToLocal(ctx context.Context, knowledgeId string, fileName string, file multipart.File) (finalPath string, err error) {
	// 构建目标目录路径: upload/knowledge_file/知识库id
	targetDir := filepath.Join("upload", "knowledge_file", knowledgeId)

	// 确保目标目录存在
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		g.Log().Errorf(ctx, "Failed to create directory %s: %v", targetDir, err)
		return "", errors.Newf(errors.ErrFileUploadFailed, "failed to create directory %s: %v", targetDir, err)
	}

	// 构建最终文件路径: upload/knowledge_file/知识库id/文件名
	finalPath = filepath.Join(targetDir, fileName)

	// 创建目标文件
	destFile, err := os.Create(finalPath)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to create file %s: %v", finalPath, err)
		return "", errors.Newf(errors.ErrFileUploadFailed, "failed to create file %s: %v", finalPath, err)
	}
	defer destFile.Close()

	// 将上传的文件内容复制到目标文件
	_, err = io.Copy(destFile, file)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to write file %s: %v", finalPath, err)
		// 删除创建失败的文件
		_ = os.Remove(finalPath)
		return "", errors.Newf(errors.ErrFileUploadFailed, "failed to write file %s: %v", finalPath, err)
	}

	g.Log().Infof(ctx, "File saved to local storage: %s", finalPath)
	return finalPath, nil
}
