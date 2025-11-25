package kbgo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/indexer/file_store"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/model/entity"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/google/uuid"
)

// UploadFile File upload interface
func (c *ControllerV1) UploadFile(ctx context.Context, req *v1.UploadFileReq) (res *v1.UploadFileRes, err error) {
	res = &v1.UploadFileRes{}

	// Get storage type
	storageType := file_store.GetStorageType()

	if storageType == file_store.StorageTypeRustFS {
		// Use RustFS storage
		return c.uploadToRustFS(ctx, req)
	} else {
		// Use local storage
		return c.uploadToLocal(ctx, req)
	}
}

// uploadToRustFS Upload file to RustFS
func (c *ControllerV1) uploadToRustFS(ctx context.Context, req *v1.UploadFileReq) (res *v1.UploadFileRes, err error) {
	res = &v1.UploadFileRes{}

	rustfsConfig := file_store.GetRustfsConfig()

	fileName, fileExt, fileSha256, tempFilePath, err := common.HandleFileUpload(ctx, req.File, req.URL)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to process file upload pre-steps: %v", err)
		res.Status = "failed"
		res.Message = "Failed to process file upload pre-steps: " + err.Error()
		// Clean up temporary file
		_ = gfile.Remove(tempFilePath)
		return res, err
	}

	// Check if a file with the same SHA256 already exists
	existingDoc, err := knowledge.GetDocumentBySHA256(ctx, req.KnowledgeId, fileSha256)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to query existing document: %v", err)
		// Continue processing, don't interrupt upload process
	} else if existingDoc.Id != "" {
		// File already exists, reject upload
		g.Log().Infof(ctx, "File already exists, SHA256: %s, upload rejected", fileSha256)

		// Clean up temporary file
		_ = gfile.Remove(tempFilePath)

		// Return error message
		res.DocumentId = ""
		res.Status = "failed"
		res.Message = "File already exists, upload rejected"
		return res, nil
	}
	//TODO 全部修改为本地upload文件上传
	info, err := file_store.UploadToRustFS(ctx, rustfsConfig.Client, rustfsConfig.BucketName, req.KnowledgeId, tempFilePath)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to upload file to RustFS: %v", err)
		res.Status = "failed"
		res.Message = "Failed to upload file to RustFS: " + err.Error()
		// Clean up temporary file
		_ = gfile.Remove(tempFilePath)
		return res, err
	}

	// Save document information to database
	documents := entity.KnowledgeDocuments{
		Id:             strings.ReplaceAll(uuid.New().String(), "-", ""),
		KnowledgeId:    req.KnowledgeId,
		FileName:       fileName,
		FileExtension:  fileExt,
		CollectionName: req.KnowledgeId, // Use knowledge base ID as default CollectionName
		SHA256:         fileSha256,
		RustfsBucket:   rustfsConfig.BucketName,
		RustfsLocation: info.Key,
		LocalFilePath:  "", // Save local file path
		Status:         int(v1.StatusPending),
	}

	// Save to database
	_, err = knowledge.SaveDocumentsInfo(ctx, documents)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to save document information to database: %v", err)
		res.Status = "failed"
		res.Message = "Failed to save document information to database: " + err.Error()
		// Clean up file
		_ = gfile.Remove(tempFilePath)
		return res, err
	}
	res.DocumentId = documents.Id
	res.Status = "success"
	res.Message = "File uploaded successfully"
	return res, nil
}

// uploadToLocal Upload file to local
func (c *ControllerV1) uploadToLocal(ctx context.Context, req *v1.UploadFileReq) (res *v1.UploadFileRes, err error) {
	res = &v1.UploadFileRes{}

	fileName, fileExt, fileSha256, tempFilePath, err := common.HandleFileUpload(ctx, req.File, req.URL)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to process file: %v", err)
		res.Status = "failed"
		res.Message = "Failed to process file: " + err.Error()
		// Clean up temporary file
		_ = gfile.Remove(tempFilePath)
		return res, err
	}

	// Check if a file with the same SHA256 already exists
	existingDoc, err := knowledge.GetDocumentBySHA256(ctx, req.KnowledgeId, fileSha256)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to query existing document: %v", err)
		// Continue processing, don't interrupt upload process
	} else if existingDoc.Id != "" {
		// File already exists, reject upload
		g.Log().Infof(ctx, "File already exists, SHA256: %s, upload rejected", fileSha256)

		// Clean up temporary file
		_ = gfile.Remove(tempFilePath)

		// Return error message
		res.DocumentId = ""
		res.Status = "failed"
		res.Message = "File duplicated, upload rejected"
		return res, nil
	}

	uploadDir := filepath.Join("knowledge_file", req.KnowledgeId)

	// Check if directory exists, report error if not
	if !gfile.Exists(uploadDir) {
		err = fmt.Errorf("upload directory does not exist: %s", uploadDir)
		g.Log().Errorf(ctx, "Upload directory does not exist: %v", err)
		res.Status = "failed"
		res.Message = "Upload directory does not exist: " + err.Error()
		// Clean up temporary file
		_ = gfile.Remove(tempFilePath)
		return res, err
	}

	finalFilePath := filepath.Join(uploadDir, fileName)

	// Move file to final location
	err = os.Rename(tempFilePath, finalFilePath)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to move file to final location: %v", err)
		res.Status = "failed"
		res.Message = "Failed to move file to final location: " + err.Error()
		// Clean up temporary file
		_ = gfile.Remove(tempFilePath)
		return res, err
	}

	// Save document information to database
	documents := entity.KnowledgeDocuments{
		Id:             strings.ReplaceAll(uuid.New().String(), "-", ""),
		KnowledgeId:    req.KnowledgeId,
		FileName:       fileName,
		FileExtension:  fileExt,
		CollectionName: req.KnowledgeId, // Use knowledge base ID as default CollectionName
		SHA256:         fileSha256,
		LocalFilePath:  finalFilePath,
		Status:         int(v1.StatusPending),
	}

	// Save to database
	_, err = knowledge.SaveDocumentsInfo(ctx, documents)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to save document information to database: %v", err)
		res.Status = "failed"
		res.Message = "Failed to save document information to database: " + err.Error()
		// Clean up file
		_ = gfile.Remove(finalFilePath)
		return res, err
	}
	res.DocumentId = documents.Id
	res.Status = "success"
	res.Message = "File uploaded successfully"
	return res, nil
}
