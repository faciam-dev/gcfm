package snapshot

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/faciam-dev/gcfm/internal/customfield/registry"
	"github.com/faciam-dev/gcfm/internal/customfield/registry/codec"
)

type Dest interface {
	Write(ctx context.Context, name string, data []byte) error
}

type LocalDir struct{ Path string }

func (l LocalDir) Write(_ context.Context, name string, data []byte) error {
	if err := os.MkdirAll(l.Path, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(l.Path, name), data, 0o644)
}

type S3 struct {
	Bucket string
	Prefix string
	client *s3.Client
}

func NewS3(ctx context.Context, bucket, prefix string) (S3, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return S3{}, err
	}
	return S3{Bucket: bucket, Prefix: prefix, client: s3.NewFromConfig(cfg)}, nil
}

func (s S3) Write(ctx context.Context, name string, data []byte) error {
	key := filepath.Join(s.Prefix, name)
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	return err
}

func Export(ctx context.Context, db *sql.DB, schema, driver, prefix string, dest Dest) error {
	metas, err := registry.LoadSQL(ctx, db, registry.DBConfig{Schema: schema, Driver: driver, TablePrefix: prefix})
	if err != nil {
		return err
	}
	data, err := codec.EncodeYAML(metas)
	if err != nil {
		return err
	}
	fname := fmt.Sprintf("registry_%s.yaml", time.Now().Format("2006-01-02T15-04-05"))
	return dest.Write(ctx, fname, data)
}
