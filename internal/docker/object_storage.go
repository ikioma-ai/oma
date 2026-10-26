package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/encrypt"
)

const (
	DefaultObjectStorageConfigPath = "/etc/oma/object-storage.json"
	ObjectStorageConfigEnv         = "OMA_OBJECT_STORAGE_CONFIG"
)

type ObjectStorageConfig struct {
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	Prefix    string `json:"prefix"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

func LoadObjectStorageConfig() (ObjectStorageConfig, error) {
	configPath := os.Getenv(ObjectStorageConfigEnv)
	if configPath == "" {
		configPath = DefaultObjectStorageConfigPath
	}

	info, err := os.Stat(configPath)
	if err != nil {
		return ObjectStorageConfig{}, fmt.Errorf("reading object storage config: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return ObjectStorageConfig{}, fmt.Errorf("object storage config must not be accessible by other users")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return ObjectStorageConfig{}, fmt.Errorf("reading object storage config: %w", err)
	}
	var config ObjectStorageConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return ObjectStorageConfig{}, fmt.Errorf("parsing object storage config: %w", err)
	}
	if err := config.Validate(); err != nil {
		return ObjectStorageConfig{}, err
	}
	return config, nil
}

func (c ObjectStorageConfig) Validate() error {
	if c.Endpoint == "" || c.Bucket == "" || c.AccessKey == "" || c.SecretKey == "" {
		return fmt.Errorf("object storage requires endpoint, bucket, access key, and secret key")
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("object storage endpoint must be an absolute http(s) URL")
	}
	return nil
}

func (c ObjectStorageConfig) ObjectKey(appName string, t time.Time) string {
	name := fmt.Sprintf("%s-%s.tar.gz", appName, t.Format(backupTimeFormat))
	if c.Prefix == "" {
		return name
	}
	return path.Join(c.Prefix, name)
}

func (c ObjectStorageConfig) Store() (*ObjectStore, error) {
	u, err := url.Parse(c.Endpoint)
	if err != nil {
		return nil, err
	}
	client, err := minio.New(u.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(c.AccessKey, c.SecretKey, ""),
		Secure: u.Scheme == "https",
		Region: c.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("creating object storage client: %w", err)
	}
	return &ObjectStore{client: client, config: c}, nil
}

type ObjectStore struct {
	client *minio.Client
	config ObjectStorageConfig
}

func (s *ObjectStore) Upload(ctx context.Context, key string, r io.Reader, size int64) error {
	if _, err := s.client.StatObject(ctx, s.config.Bucket, key, minio.StatObjectOptions{}); err == nil {
		return fmt.Errorf("object %q already exists", key)
	} else if minio.ToErrorResponse(err).Code != "NoSuchKey" && minio.ToErrorResponse(err).Code != "NotFound" {
		return fmt.Errorf("checking object %q: %w", key, err)
	}

	_, err := s.client.PutObject(ctx, s.config.Bucket, key, r, size, minio.PutObjectOptions{
		ContentType:          "application/gzip",
		ServerSideEncryption: encrypt.NewSSE(),
	})
	if err != nil {
		return fmt.Errorf("uploading object %q: %w", key, err)
	}
	return nil
}

type ObjectArchive struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

func (s *ObjectStore) List(ctx context.Context) ([]ObjectArchive, error) {
	prefix := ""
	if s.config.Prefix != "" {
		prefix = strings.TrimSuffix(s.config.Prefix, "/") + "/"
	}

	var archives []ObjectArchive
	for object := range s.client.ListObjects(ctx, s.config.Bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if object.Err != nil {
			return nil, fmt.Errorf("listing object backups: %w", object.Err)
		}
		name := strings.TrimPrefix(object.Key, prefix)
		if name == object.Key || !isObjectArchiveName(name) {
			continue
		}
		archives = append(archives, ObjectArchive{Name: name, Size: object.Size, Modified: object.LastModified})
	}
	return archives, nil
}

func (s *ObjectStore) Download(ctx context.Context, name string, w io.Writer) error {
	if !isObjectArchiveName(name) {
		return fmt.Errorf("invalid object archive name %q", name)
	}
	key := name
	if s.config.Prefix != "" {
		key = path.Join(s.config.Prefix, name)
	}

	object, err := s.client.GetObject(ctx, s.config.Bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("opening object %q: %w", key, err)
	}
	defer object.Close()

	if _, err := io.Copy(w, object); err != nil {
		return fmt.Errorf("downloading object %q: %w", key, err)
	}
	return nil
}

func isObjectArchiveName(name string) bool {
	if name == "" || name == "." || name == ".." || name != path.Base(name) {
		return false
	}
	if strings.ContainsAny(name, "/\\\x00\n\r") {
		return false
	}
	return strings.HasSuffix(name, ".tar.gz")
}

func (s *ObjectStore) Trim(ctx context.Context, appName string, cutoff time.Time) error {
	prefix := appName + "-"
	if s.config.Prefix != "" {
		prefix = path.Join(s.config.Prefix, prefix)
	}

	for object := range s.client.ListObjects(ctx, s.config.Bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if object.Err != nil {
			return fmt.Errorf("listing object backups: %w", object.Err)
		}
		if object.LastModified.After(cutoff) {
			continue
		}
		if err := s.client.RemoveObject(ctx, s.config.Bucket, object.Key, minio.RemoveObjectOptions{}); err != nil {
			return fmt.Errorf("removing expired object %q: %w", object.Key, err)
		}
	}
	return nil
}
