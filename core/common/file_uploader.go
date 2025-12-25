package common

import (
	"context"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Malowking/kbgo/core/errors"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/google/uuid"
)

const (
	// MaxFileSize 单个文件最大大小 (5MB) - 用于文件对话中的多模态文件
	MaxFileSize = 5 * 1024 * 1024
	// MaxFilesPerRequest 每次请求最多上传文件数
	MaxFilesPerRequest = 5
)

// FileType 文件类型枚举
type FileType string

const (
	FileTypeImage FileType = "image"
	FileTypeAudio FileType = "audio"
	FileTypeVideo FileType = "video"
	FileTypeOther FileType = "file"
)

// MultimodalFile 多模态文件信息
type MultimodalFile struct {
	FileName     string   // 原始文件名
	FileType     FileType // 文件类型（image/audio/video）
	FilePath     string   // 保存后的完整路径
	RelativePath string   // 相对路径
	Size         int64    // 文件大小
}

// FileUploader 文件上传处理器（支持异步上传）
type FileUploader struct {
	baseDir    string // 基础上传目录
	taskQueue  chan *FileUploadTask
	workerPool int
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

// FileUploadTask 文件上传任务
type FileUploadTask struct {
	File   *multipart.FileHeader
	Result chan *FileUploadResult
}

// FileUploadResult 文件上传结果
type FileUploadResult struct {
	File *MultimodalFile
	Err  error
}

// NewFileUploader 创建文件上传处理器
func NewFileUploader(baseDir string, workerPool int) *FileUploader {
	if baseDir == "" {
		baseDir = "upload"
	}
	if workerPool <= 0 {
		workerPool = 10 // 默认10个worker
	}

	ctx, cancel := context.WithCancel(context.Background())
	fu := &FileUploader{
		baseDir:    baseDir,
		taskQueue:  make(chan *FileUploadTask, 100), // 缓冲队列
		workerPool: workerPool,
		ctx:        ctx,
		cancel:     cancel,
	}

	// 启动worker pool
	fu.start()

	return fu
}

// start 启动worker pool
func (fu *FileUploader) start() {
	for i := 0; i < fu.workerPool; i++ {
		fu.wg.Add(1)
		go fu.worker()
	}
}

// worker 处理文件上传任务
func (fu *FileUploader) worker() {
	defer fu.wg.Done()

	for {
		select {
		case <-fu.ctx.Done():
			return
		case task, ok := <-fu.taskQueue:
			if !ok {
				return
			}
			// 处理文件上传
			file, err := fu.saveFile(task.File)
			task.Result <- &FileUploadResult{
				File: file,
				Err:  err,
			}
			close(task.Result)
		}
	}
}

// GetFileType 根据文件扩展名判断文件类型
func (fu *FileUploader) GetFileType(filename string) FileType {
	ext := strings.ToLower(filepath.Ext(filename))

	// 图片类型
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".ico", ".tiff"}
	for _, imgExt := range imageExts {
		if ext == imgExt {
			return FileTypeImage
		}
	}

	// 音频类型
	audioExts := []string{".mp3", ".wav", ".flac", ".aac", ".ogg", ".m4a", ".wma", ".ape"}
	for _, audioExt := range audioExts {
		if ext == audioExt {
			return FileTypeAudio
		}
	}

	// 视频类型
	videoExts := []string{".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpeg", ".mpg"}
	for _, videoExt := range videoExts {
		if ext == videoExt {
			return FileTypeVideo
		}
	}

	return FileTypeOther
}

// saveFile 保存单个文件
func (fu *FileUploader) saveFile(file *multipart.FileHeader) (*MultimodalFile, error) {
	if file == nil {
		return nil, errors.New(errors.ErrInvalidParameter, "file is nil")
	}

	// 检查文件名是否为空
	if file.Filename == "" {
		return nil, errors.New(errors.ErrInvalidParameter, "file filename is empty")
	}

	// 检查文件大小
	if file.Size > MaxFileSize {
		return nil, errors.Newf(errors.ErrFileUploadFailed, "file size %d exceeds maximum allowed size %d", file.Size, MaxFileSize)
	}

	// 获取文件类型
	fileType := fu.GetFileType(file.Filename)

	// 构建目标目录
	targetDir := filepath.Join(fu.baseDir, string(fileType))

	// 确保目录存在
	if !gfile.Exists(targetDir) {
		if err := gfile.Mkdir(targetDir); err != nil {
			return nil, errors.Newf(errors.ErrFileUploadFailed, "failed to create directory %s: %v", targetDir, err)
		}
	}

	// 获取文件扩展名
	ext := filepath.Ext(file.Filename)

	// 为所有文件生成UUID文件名
	uuidFileName := strings.ReplaceAll(uuid.New().String(), "-", "") + ext

	// 构建目标文件路径
	targetPath := filepath.Join(targetDir, uuidFileName)

	// 打开上传的文件
	src, err := file.Open()
	if err != nil {
		return nil, errors.Newf(errors.ErrFileUploadFailed, "failed to open uploaded file: %v", err)
	}
	defer src.Close()

	// 创建目标文件
	dst, err := os.Create(targetPath)
	if err != nil {
		return nil, errors.Newf(errors.ErrFileUploadFailed, "failed to create target file %s: %v", targetPath, err)
	}
	defer dst.Close()

	// 复制文件内容
	size, err := io.Copy(dst, src)
	if err != nil {
		return nil, errors.Newf(errors.ErrFileUploadFailed, "failed to copy file content: %v", err)
	}

	multiFile := &MultimodalFile{
		FileName:     file.Filename, // 保留原始文件名用于显示
		FileType:     fileType,
		FilePath:     targetPath,                                    // UUID重命名后的完整路径
		RelativePath: filepath.Join(string(fileType), uuidFileName), // UUID重命名后的相对路径
		Size:         size,
	}

	return multiFile, nil
}

// UploadFiles 异步上传多个文件
func (fu *FileUploader) UploadFiles(ctx context.Context, files []*multipart.FileHeader) ([]*MultimodalFile, error) {
	if len(files) == 0 {
		return nil, nil
	}

	// 检查文件数量限制
	if len(files) > MaxFilesPerRequest {
		return nil, errors.Newf(errors.ErrFileUploadFailed, "too many files: %d, maximum allowed: %d", len(files), MaxFilesPerRequest)
	}

	// 创建任务和结果通道
	results := make([]*FileUploadResult, len(files))
	tasks := make([]*FileUploadTask, len(files))

	// 提交所有任务
	for i, file := range files {
		task := &FileUploadTask{
			File:   file,
			Result: make(chan *FileUploadResult, 1),
		}
		tasks[i] = task

		select {
		case <-ctx.Done():
			return nil, errors.New(errors.ErrFileUploadFailed, "context cancelled during file upload")
		case fu.taskQueue <- task:
			// 任务提交成功
		default:
			// 队列满了，同步处理
			g.Log().Warning(ctx, "File upload queue is full, processing synchronously")
			file, err := fu.saveFile(file)
			results[i] = &FileUploadResult{File: file, Err: err}
			tasks[i] = nil // 标记为已处理
		}
	}

	// 收集所有结果
	for i, task := range tasks {
		if task == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return nil, errors.New(errors.ErrFileUploadFailed, "context cancelled while waiting for upload results")
		case result := <-task.Result:
			results[i] = result
		}
	}

	// 整理结果
	var uploadedFiles []*MultimodalFile
	var lastErr error

	for i, result := range results {
		if result == nil {
			continue
		}
		if result.Err != nil {
			g.Log().Errorf(ctx, "Failed to upload file %s: %v", files[i].Filename, result.Err)
			lastErr = result.Err
			continue
		}
		if result.File != nil {
			uploadedFiles = append(uploadedFiles, result.File)
		}
	}

	return uploadedFiles, lastErr
}

// GetFileURL 根据相对路径获取文件的访问URL
func (fu *FileUploader) GetFileURL(relativePath string) string {
	// 这里可以根据实际需求返回完整的URL
	// 例如：http://localhost:8080/upload/image/test.jpg
	return filepath.Join("/", fu.baseDir, relativePath)
}

// DeleteFile 删除文件
func (fu *FileUploader) DeleteFile(relativePath string) error {
	fullPath := filepath.Join(fu.baseDir, relativePath)
	if !gfile.Exists(fullPath) {
		return errors.Newf(errors.ErrFileReadFailed, "file not found: %s", relativePath)
	}
	return os.Remove(fullPath)
}

// Shutdown 关闭文件上传器
func (fu *FileUploader) Shutdown() {
	fu.cancel()
	close(fu.taskQueue)
	fu.wg.Wait()
}

// 全局文件上传器实例
var globalFileUploader *FileUploader
var uploaderOnce sync.Once

// GetGlobalFileUploader 获取全局文件上传器
func GetGlobalFileUploader() *FileUploader {
	uploaderOnce.Do(func() {
		globalFileUploader = NewFileUploader("upload", 10)
	})
	return globalFileUploader
}
