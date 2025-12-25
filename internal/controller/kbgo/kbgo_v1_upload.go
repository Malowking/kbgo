package kbgo

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/core/file_store"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/google/uuid"
)

// UploadFile File upload interface
func (c *ControllerV1) UploadFile(ctx context.Context, req *v1.UploadFileReq) (res *v1.UploadFileRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "UploadFile request received - URL: %s, KnowledgeId: %s", req.URL, req.KnowledgeId)

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

	fileName, fileExt, fileSha256, fileReader, err := common.HandleFileUpload(ctx, req.File, req.URL)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to process file upload pre-steps: %v", err)
		res.Status = "failed"
		res.Message = "Failed to process file upload pre-steps: " + err.Error()
		return res, errors.Newf(errors.ErrFileUploadFailed, "failed to process file upload pre-steps: %v", err)
	}
	defer func() {
		if closer, ok := fileReader.(io.Closer); ok {
			_ = closer.Close()
		}
	}()

	// Check if a file with the same SHA256 already exists
	existingDoc, err := knowledge.GetDocumentBySHA256(ctx, req.KnowledgeId, fileSha256)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to query existing document: %v", err)
		// Continue processing, don't interrupt upload process
	} else if existingDoc.ID != "" {
		// File already exists, reject upload
		g.Log().Infof(ctx, "File already exists, SHA256: %s, upload rejected", fileSha256)

		// Return error message
		res.DocumentId = ""
		res.Status = "failed"
		res.Message = "File already exists, upload rejected"
		return res, nil
	}

	localPath, rustfsKey, err := file_store.SaveFileToRustFS(ctx, rustfsConfig.Client, rustfsConfig.BucketName, req.KnowledgeId, fileName, fileReader)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to upload file to RustFS: %v", err)
		res.Status = "failed"
		res.Message = "Failed to upload file to RustFS: " + err.Error()
		// Clean up local file if it was created
		if localPath != "" {
			_ = gfile.Remove(localPath)
		}
		return res, errors.Newf(errors.ErrFileUploadFailed, "failed to upload file to RustFS: %v", err)
	}

	// Prepare document information
	documents := gorm.KnowledgeDocuments{
		ID:             strings.ReplaceAll(uuid.New().String(), "-", ""),
		KnowledgeId:    req.KnowledgeId,
		FileName:       fileName,
		FileExtension:  fileExt,
		CollectionName: req.KnowledgeId, // Use knowledge base ID as default CollectionName
		SHA256:         fileSha256,
		RustfsBucket:   rustfsConfig.BucketName,
		RustfsLocation: rustfsKey,
		LocalFilePath:  localPath, // Save local file path
		Status:         int8(v1.StatusPending),
	}

	db := dao.GetDB()
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		g.Log().Errorf(ctx, "Failed to start database transaction: %v", tx.Error)

		// Rollback: Delete the file from RustFS since transaction couldn't start
		deleteErr := file_store.DeleteObject(ctx, rustfsConfig.Client, rustfsConfig.BucketName, rustfsKey)
		if deleteErr != nil {
			g.Log().Errorf(ctx, "Failed to rollback file from RustFS after transaction start error: %v", deleteErr)
		}

		// Clean up local file
		_ = gfile.Remove(localPath)

		res.Status = "failed"
		res.Message = "Failed to start database transaction"
		return res, errors.Newf(errors.ErrFileUploadFailed, "failed to start database transaction: %v", tx.Error)
	}

	// Ensure transaction is rolled back on any error
	defer func() {
		if err != nil && tx != nil {
			tx.Rollback()
		}
	}()

	_, err = knowledge.SaveDocumentsInfoWithTx(ctx, tx, documents)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to save document information to database: %v", err)

		// Rollback: Delete the file from RustFS since database insert failed
		deleteErr := file_store.DeleteObject(ctx, rustfsConfig.Client, rustfsConfig.BucketName, rustfsKey)
		if deleteErr != nil {
			g.Log().Errorf(ctx, "Failed to rollback file from RustFS after insert error: %v", deleteErr)
		}

		// Clean up local file
		_ = gfile.Remove(localPath)

		res.Status = "failed"
		res.Message = "Failed to save document information to database: " + err.Error()
		return res, errors.Newf(errors.ErrDatabaseInsert, "failed to save document information: %v", err)
	}

	if err = tx.Commit().Error; err != nil {
		g.Log().Errorf(ctx, "Failed to commit database transaction: %v", err)

		// Rollback: Delete the file from RustFS since transaction commit failed
		deleteErr := file_store.DeleteObject(ctx, rustfsConfig.Client, rustfsConfig.BucketName, rustfsKey)
		if deleteErr != nil {
			g.Log().Errorf(ctx, "Failed to rollback file from RustFS after commit error: %v", deleteErr)
		} else {
			g.Log().Infof(ctx, "Successfully rolled back file from RustFS after commit error: %s", rustfsKey)
		}

		// Clean up local file
		_ = gfile.Remove(localPath)

		res.Status = "failed"
		res.Message = "Failed to commit database transaction: " + err.Error()
		return res, errors.Newf(errors.ErrFileUploadFailed, "failed to commit transaction: %v", err)
	}

	res.DocumentId = documents.ID
	res.Status = "success"
	res.Message = "File uploaded successfully"
	return res, nil
}

// uploadToLocal Upload file to local
func (c *ControllerV1) uploadToLocal(ctx context.Context, req *v1.UploadFileReq) (res *v1.UploadFileRes, err error) {
	res = &v1.UploadFileRes{}

	fileName, fileExt, fileSha256, fileReader, err := common.HandleFileUpload(ctx, req.File, req.URL)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to process file: %v", err)
		res.Status = "failed"
		res.Message = "Failed to process file: " + err.Error()
		return res, errors.Newf(errors.ErrFileUploadFailed, "failed to process file: %v", err)
	}
	defer func() {
		if closer, ok := fileReader.(io.Closer); ok {
			_ = closer.Close()
		}
	}()

	// Check if a file with the same SHA256 already exists
	existingDoc, err := knowledge.GetDocumentBySHA256(ctx, req.KnowledgeId, fileSha256)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to query existing document: %v", err)
		// Continue processing, don't interrupt upload process
	} else if existingDoc.ID != "" {
		// File already exists, reject upload
		g.Log().Infof(ctx, "File already exists, SHA256: %s, upload rejected", fileSha256)

		// Return error message
		res.DocumentId = ""
		res.Status = "failed"
		res.Message = "File duplicated, upload rejected"
		return res, errors.New(errors.ErrFileAlreadyExists, "file already exists")
	}

	// Convert fileReader to multipart.File if it's from an uploaded file
	var finalPath string
	if req.File != nil {
		// For uploaded files, open it directly
		multipartFile, err := req.File.Open()
		if err != nil {
			g.Log().Errorf(ctx, "Failed to open file: %v", err)
			res.Status = "failed"
			res.Message = "Failed to open file: " + err.Error()
			return res, errors.Newf(errors.ErrFileReadFailed, "failed to open file: %v", err)
		}
		defer multipartFile.Close()

		// Save to local storage
		finalPath, err = file_store.SaveFileToLocal(ctx, req.KnowledgeId, fileName, multipartFile)
		if err != nil {
			g.Log().Errorf(ctx, "Failed to save file to local storage: %v", err)
			res.Status = "failed"
			res.Message = "Failed to save file to local storage: " + err.Error()
			return res, errors.Newf(errors.ErrFileUploadFailed, "failed to save file to local storage: %v", err)
		}
	} else {
		// For URL files, the fileReader is an os.File, we need to save it
		if osFile, ok := fileReader.(*os.File); ok {
			tempPath := osFile.Name()
			defer func() {
				_ = osFile.Close()
				_ = os.Remove(tempPath) // 清理临时文件
			}()

			// Create target directory
			targetDir := filepath.Join("upload", "knowledge_file", req.KnowledgeId)
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				g.Log().Errorf(ctx, "Failed to create directory: %v", err)
				res.Status = "failed"
				res.Message = "Failed to create directory: " + err.Error()
				return res, errors.Newf(errors.ErrFileUploadFailed, "failed to create directory: %v", err)
			}

			// Move file to final location
			finalPath = filepath.Join(targetDir, fileName)
			if err := os.Rename(tempPath, finalPath); err != nil {
				g.Log().Errorf(ctx, "Failed to move file: %v", err)
				res.Status = "failed"
				res.Message = "Failed to move file: " + err.Error()
				return res, errors.Newf(errors.ErrFileUploadFailed, "failed to move file: %v", err)
			}
		}
	}

	// Prepare document information
	documents := gorm.KnowledgeDocuments{
		ID:             strings.ReplaceAll(uuid.New().String(), "-", ""),
		KnowledgeId:    req.KnowledgeId,
		FileName:       fileName,
		FileExtension:  fileExt,
		CollectionName: req.KnowledgeId, // Use knowledge base ID as default CollectionName
		SHA256:         fileSha256,
		LocalFilePath:  finalPath,
		Status:         int8(v1.StatusPending),
	}

	// Start database transaction AFTER file operations
	db := dao.GetDB()
	tx := db.WithContext(ctx).Begin()
	if tx.Error != nil {
		g.Log().Errorf(ctx, "Failed to start database transaction: %v", tx.Error)

		// Rollback: Delete the local file since transaction couldn't start
		_ = gfile.Remove(finalPath)

		res.Status = "failed"
		res.Message = "Failed to start database transaction"
		return res, errors.Newf(errors.ErrFileUploadFailed, "failed to start database transaction: %v", tx.Error)
	}

	// Ensure transaction is rolled back on any error
	defer func() {
		if err != nil && tx != nil {
			tx.Rollback()
		}
	}()

	// Save to database using transaction
	_, err = knowledge.SaveDocumentsInfoWithTx(ctx, tx, documents)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to save document information to database: %v", err)

		// Rollback: Delete the local file since database insert failed
		_ = gfile.Remove(finalPath)

		res.Status = "failed"
		res.Message = "Failed to save document information to database: " + err.Error()
		return res, errors.Newf(errors.ErrDatabaseInsert, "failed to save document information: %v", err)
	}

	// Commit the transaction
	if err = tx.Commit().Error; err != nil {
		g.Log().Errorf(ctx, "Failed to commit database transaction: %v", err)

		// Rollback: Delete the local file since transaction commit failed
		_ = gfile.Remove(finalPath)

		res.Status = "failed"
		res.Message = "Failed to commit database transaction: " + err.Error()
		return res, errors.Newf(errors.ErrFileUploadFailed, "failed to commit transaction: %v", err)
	}

	res.DocumentId = documents.ID
	res.Status = "success"
	res.Message = "File uploaded successfully"
	return res, nil
}
