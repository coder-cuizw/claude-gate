// Package s3 用 minio-go 实现 observ.BodyStore（请求/响应 body 落 S3/MinIO）。
//
// 路径规则（任务书 §5.6）：requests/{YYYY-MM-DD}/{trace_id}/{kind}.json
package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/claude-gate/claude-gate/internal/observ"
)

// Store 是基于 S3/MinIO 的 body 落盘实现。
type Store struct {
	client *minio.Client
	bucket string
}

var _ observ.BodyStore = (*Store)(nil)

// Options 配置 S3/MinIO 连接。
type Options struct {
	Endpoint  string // host:9100
	AccessKey string
	SecretKey string
	Bucket    string
	UseSSL    bool
}

// New 连接 MinIO/S3，必要时自动创建 bucket。
func New(ctx context.Context, o Options) (*Store, error) {
	client, err := minio.New(o.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(o.AccessKey, o.SecretKey, ""),
		Secure: o.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 S3 客户端失败: %w", err)
	}
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	exists, err := client.BucketExists(checkCtx, o.Bucket)
	if err != nil {
		return nil, fmt.Errorf("检查 bucket 失败: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(checkCtx, o.Bucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("创建 bucket 失败: %w", err)
		}
	}
	return &Store{client: client, bucket: o.Bucket}, nil
}

// Put 写入 body 并返回 S3 key。
func (s *Store) Put(ctx context.Context, traceID, kind string, body []byte) (string, error) {
	key := fmt.Sprintf("requests/%s/%s/%s.json", time.Now().UTC().Format("2006-01-02"), traceID, kind)
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(body), int64(len(body)),
		minio.PutObjectOptions{ContentType: "application/json"})
	if err != nil {
		return "", fmt.Errorf("写 S3 失败: %w", err)
	}
	return key, nil
}

// Get 读取 body。
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

// Ping 供 readyz 探活（列 bucket 触发一次连接）。
func (s *Store) Ping(ctx context.Context) error {
	_, err := s.client.BucketExists(ctx, s.bucket)
	return err
}
