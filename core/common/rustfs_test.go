package common

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestRustFSUploadDownload(t *testing.T) {
	// 配置
	endpoint := "www.sgxlllm.cn:9000"
	accessKeyID := "gOlxd2a38wUyAeI1rtKD"
	secretAccessKey := "7CbIT6yV9mJReQYzkZhdE83Sq2LonGpD051PAa4X"
	useSSL := false

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()
	bucketName := "kbfiles"
	//region := ""

	//// 创建 bucket
	//err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: region})
	//if err != nil {
	//	exists, errBucketExists := minioClient.BucketExists(ctx, bucketName)
	//	if errBucketExists == nil && exists {
	//		t.Logf("Bucket %s already exists", bucketName)
	//	} else {
	//		t.Fatalf("failed to create bucket: %v", err)
	//	}
	//} else {
	//	t.Logf("Created bucket %s", bucketName)
	//}

	// 上传文件
	objectName := "test/TODO.txt"
	filePath := "/Users/wing/code/go_code/kbgo/TODO.txt"
	contentType := "text/plain"

	info, err := minioClient.FPutObject(ctx, bucketName, objectName, filePath,
		minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	t.Logf("Uploaded %s of size %d", objectName, info.Size)
	fmt.Println(info)

	// 列举对象
	found := false
	objectCh := minioClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})
	for object := range objectCh {
		if object.Err != nil {
			t.Fatalf("list error: %v", object.Err)
		}
		t.Logf("Found object: %s", object.Key)
		if object.Key == objectName {
			found = true
		}
	}
	if !found {
		t.Fatalf("object %s not found after upload", objectName)
	}

	// 下载文件
	destFile := "./downloaded_TODO.txt"
	err = minioClient.FGetObject(ctx, bucketName, objectName, destFile, minio.GetObjectOptions{})
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	defer os.Remove(destFile)
	t.Logf("Downloaded %s to %s", objectName, destFile)

	//删除对象
	err = minioClient.RemoveObject(ctx, bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		t.Fatalf("delete object failed: %v", err)
	}
	t.Logf("Deleted object %s", objectName)

	////删除 bucket
	//err = minioClient.RemoveBucket(ctx, bucketName)
	//if err != nil {
	//	t.Fatalf("delete bucket failed: %v", err)
	//}
	//t.Logf("Deleted bucket %s", bucketName)
	//
	//fmt.Println("✅ RustFS upload/download/delete test passed.")
}
