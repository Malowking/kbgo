package kbgo

import (
	"context"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/model/entity"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/google/uuid"
)

// UploadFile 文件上传接口
func (c *ControllerV1) UploadFile(ctx context.Context, req *v1.UploadFileReq) (res *v1.UploadFileRes, err error) {
	res = &v1.UploadFileRes{}

	// 获取存储类型
	storageType := common.GetStorageType()

	if storageType == common.StorageTypeRustFS {
		// 使用 RustFS 存储
		return c.uploadToRustFS(ctx, req)
	} else {
		// 使用本地存储
		return c.uploadToLocal(ctx, req)
	}
}

// uploadToRustFS 上传文件到 RustFS
func (c *ControllerV1) uploadToRustFS(ctx context.Context, req *v1.UploadFileReq) (res *v1.UploadFileRes, err error) {
	res = &v1.UploadFileRes{}

	rustfsConfig := common.GetRustfsConfig()

	fileName, fileExt, fileSha256, tempFilePath, err := common.HandleFileUpload(ctx, req.File, req.URL)
	if err != nil {
		g.Log().Errorf(ctx, "处理文件上传前置步骤失败: %v", err)
		res.Status = "failed"
		res.Message = "处理文件上传前置步骤失败: " + err.Error()
		// 清理临时文件
		_ = gfile.Remove(tempFilePath)
		return res, err
	}

	// 检查是否已存在相同 SHA256 的文件
	existingDoc, err := knowledge.GetDocumentBySHA256(ctx, req.KnowledgeId, fileSha256)
	if err != nil {
		g.Log().Errorf(ctx, "查询已存在文档失败: %v", err)
		// 继续处理，不中断上传流程
	} else if existingDoc.Id != "" {
		// 文件已存在，拒绝上传
		g.Log().Infof(ctx, "文件已存在，SHA256: %s, 拒绝上传", fileSha256)

		// 清理临时文件
		_ = gfile.Remove(tempFilePath)

		// 返回错误信息
		res.DocumentId = ""
		res.Status = "failed"
		res.Message = "文件重复，拒绝上传"
		return res, nil
	}

	// 上传到 RustFS
	uploadFile := &ghttp.UploadFile{
		FileHeader: &multipart.FileHeader{
			Filename: fileName,
			Size:     gfile.Size(tempFilePath),
		},
	}

	info, err := common.UploadToRustFS(ctx, rustfsConfig.Client, rustfsConfig.BucketName, req.KnowledgeId, uploadFile, "")
	if err != nil {
		g.Log().Errorf(ctx, "上传文件到RustFS失败: %v", err)
		res.Status = "failed"
		res.Message = "上传文件到RustFS失败: " + err.Error()
		// 删除临时文件
		_ = gfile.Remove(tempFilePath)
		return res, err
	}

	// 保存文档信息到数据库
	documents := entity.KnowledgeDocuments{
		Id:             strings.ReplaceAll(uuid.New().String(), "-", ""),
		KnowledgeId:    req.KnowledgeId,
		FileName:       fileName,
		FileExtension:  fileExt,
		CollectionName: req.KnowledgeId, // 使用知识库ID作为默认的CollectionName
		SHA256:         fileSha256,
		RustfsBucket:   rustfsConfig.BucketName,
		RustfsLocation: info.Key,
		LocalFilePath:  "", // 保存本地文件路径
		Status:         int(v1.StatusPending),
	}

	// 保存到数据库
	_, err = knowledge.SaveDocumentsInfo(ctx, documents)
	if err != nil {
		g.Log().Errorf(ctx, "保存文档信息到数据库失败: %v", err)
		res.Status = "failed"
		res.Message = "保存文档信息到数据库失败: " + err.Error()
		// 清理文件
		_ = gfile.Remove(tempFilePath)
		return res, err
	}
	res.DocumentId = documents.Id
	res.Status = "success"
	res.Message = "文件上传成功"
	return res, nil
}

// uploadToLocal 上传文件到本地
func (c *ControllerV1) uploadToLocal(ctx context.Context, req *v1.UploadFileReq) (res *v1.UploadFileRes, err error) {
	res = &v1.UploadFileRes{}

	fileName, fileExt, fileSha256, tempFilePath, err := common.HandleFileUpload(ctx, req.File, req.URL)
	if err != nil {
		g.Log().Errorf(ctx, "处理文件失败: %v", err)
		res.Status = "failed"
		res.Message = "处理文件失败: " + err.Error()
		// 清理临时文件
		_ = gfile.Remove(tempFilePath)
		return res, err
	}

	// 检查是否已存在相同 SHA256 的文件
	existingDoc, err := knowledge.GetDocumentBySHA256(ctx, req.KnowledgeId, fileSha256)
	if err != nil {
		g.Log().Errorf(ctx, "查询已存在文档失败: %v", err)
		// 继续处理，不中断上传流程
	} else if existingDoc.Id != "" {
		// 文件已存在，拒绝上传
		g.Log().Infof(ctx, "文件已存在，SHA256: %s, 拒绝上传", fileSha256)

		// 清理临时文件
		_ = gfile.Remove(tempFilePath)

		// 返回错误信息
		res.DocumentId = ""
		res.Status = "failed"
		res.Message = "文件重复，拒绝上传"
		return res, nil
	}

	uploadDir := filepath.Join("knowledge_file", req.KnowledgeId)

	// 检查目录是否存在，不存在则报错
	if !gfile.Exists(uploadDir) {
		err = fmt.Errorf("upload directory does not exist: %s", uploadDir)
		g.Log().Errorf(ctx, "上传目录不存在: %v", err)
		res.Status = "failed"
		res.Message = "上传目录不存在: " + err.Error()
		// 清理临时文件
		_ = gfile.Remove(tempFilePath)
		return res, err
	}

	finalFilePath := filepath.Join(uploadDir, fileName)

	// 移动文件到最终位置
	err = os.Rename(tempFilePath, finalFilePath)
	if err != nil {
		g.Log().Errorf(ctx, "移动文件到最终位置失败: %v", err)
		res.Status = "failed"
		res.Message = "移动文件到最终位置失败: " + err.Error()
		// 清理临时文件
		_ = gfile.Remove(tempFilePath)
		return res, err
	}

	// 保存文档信息到数据库
	documents := entity.KnowledgeDocuments{
		Id:             strings.ReplaceAll(uuid.New().String(), "-", ""),
		KnowledgeId:    req.KnowledgeId,
		FileName:       fileName,
		FileExtension:  fileExt,
		CollectionName: req.KnowledgeId, // 使用知识库ID作为默认的CollectionName
		SHA256:         fileSha256,
		LocalFilePath:  finalFilePath,
		Status:         int(v1.StatusPending),
	}

	// 保存到数据库
	_, err = knowledge.SaveDocumentsInfo(ctx, documents)
	if err != nil {
		g.Log().Errorf(ctx, "保存文档信息到数据库失败: %v", err)
		res.Status = "failed"
		res.Message = "保存文档信息到数据库失败: " + err.Error()
		// 清理文件
		_ = gfile.Remove(finalFilePath)
		return res, err
	}
	res.DocumentId = documents.Id
	res.Status = "success"
	res.Message = "文件上传成功"
	return res, nil
}
