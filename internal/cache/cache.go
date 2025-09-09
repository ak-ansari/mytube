package cache

import (
	"context"
	"fmt"
	"time"
)

const (
	VIDEO_INFO string = "video_info"
	URL        string = "url"
	KEY        string = "key"
)

func GetKey(key string, id string) string {
	return fmt.Sprintf("%s:%s", key, id)
}

type Cache interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string, target any) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}
