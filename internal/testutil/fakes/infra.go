package fakes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ak-ansari/mytube/internal/cache"
	"github.com/ak-ansari/mytube/internal/index"
	"github.com/ak-ansari/mytube/internal/models"
	"github.com/ak-ansari/mytube/internal/queue"
	"github.com/ak-ansari/mytube/internal/storage"
)

// ErrCacheMiss is returned by Cache.Get for a missing key.
var ErrCacheMiss = errors.New("cache miss")

// ---------------------------------------------------------------- Cache

// Cache is an in-memory cache.Cache. Values are stored JSON-encoded, like the Redis impl. Expiry is ignored.
type Cache struct {
	Errs  Errs
	Calls Calls
	mu    sync.Mutex
	kv    map[string][]byte
	ttl   map[string]time.Duration
	hash  map[string]map[string]string
}

var _ cache.Cache = (*Cache)(nil)

func NewCache() *Cache {
	return &Cache{kv: map[string][]byte{}, ttl: map[string]time.Duration{}, hash: map[string]map[string]string{}}
}

func (c *Cache) Set(_ context.Context, key string, value interface{}, exp time.Duration) error {
	c.Calls.add("Set")
	if err := c.Errs.get("Set"); err != nil {
		return err
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.kv[key] = b
	c.ttl[key] = exp
	return nil
}

func (c *Cache) Get(_ context.Context, key string, target any) error {
	c.Calls.add("Get")
	if err := c.Errs.get("Get"); err != nil {
		return err
	}
	c.mu.Lock()
	b, ok := c.kv[key]
	c.mu.Unlock()
	if !ok {
		return ErrCacheMiss
	}
	return json.Unmarshal(b, target)
}

func (c *Cache) Delete(_ context.Context, key string) error {
	c.Calls.add("Delete")
	if err := c.Errs.get("Delete"); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.kv, key)
	return nil
}

func (c *Cache) Exists(_ context.Context, key string) (bool, error) {
	c.Calls.add("Exists")
	if err := c.Errs.get("Exists"); err != nil {
		return false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.kv[key]
	return ok, nil
}

func (c *Cache) GetAllFromHash(_ context.Context, h string) (map[string]string, error) {
	c.Calls.add("GetAllFromHash")
	if err := c.Errs.get("GetAllFromHash"); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]string{}
	for k, v := range c.hash[h] {
		out[k] = v
	}
	return out, nil
}

func (c *Cache) GetFromHash(_ context.Context, h, key string) (string, error) {
	c.Calls.add("GetFromHash")
	if err := c.Errs.get("GetFromHash"); err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.hash[h][key]
	if !ok {
		return "", ErrCacheMiss
	}
	return v, nil
}

func (c *Cache) DeleteFromHash(_ context.Context, h, key string) error {
	c.Calls.add("DeleteFromHash")
	if err := c.Errs.get("DeleteFromHash"); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.hash[h], key)
	return nil
}

func (c *Cache) SetNX(_ context.Context, key string, value any, exp time.Duration) (bool, error) {
	c.Calls.add("SetNX")
	if err := c.Errs.get("SetNX"); err != nil {
		return false, err
	}
	b, err := json.Marshal(value)
	if err != nil {
		return false, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.kv[key]; ok {
		return false, nil
	}
	c.kv[key] = b
	c.ttl[key] = exp
	return true, nil
}

func (c *Cache) DeleteLock(ctx context.Context, key string) error {
	return c.Delete(ctx, key)
}

// PutHash seeds a hash field (e.g. a MinIO bucket event).
func (c *Cache) PutHash(h, key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hash[h] == nil {
		c.hash[h] = map[string]string{}
	}
	c.hash[h][key] = value
}

// HashLen returns the number of fields in a hash.
func (c *Cache) HashLen(h string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.hash[h])
}

// Has reports whether a plain key exists.
func (c *Cache) Has(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.kv[key]
	return ok
}

// TTL returns the expiration a key was last written with.
func (c *Cache) TTL(key string) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ttl[key]
}

// ---------------------------------------------------------------- Queue

// Queue is an in-memory FIFO queue.Queue. Dequeue on an empty queue returns (nil, nil).
type Queue struct {
	Errs  Errs
	Calls Calls
	mu    sync.Mutex
	q     map[string][][]byte
}

var _ queue.Queue = (*Queue)(nil)

func NewQueue() *Queue { return &Queue{q: map[string][][]byte{}} }

func (q *Queue) Enqueue(_ context.Context, name string, payload []byte) error {
	q.Calls.add("Enqueue")
	if err := q.Errs.get("Enqueue"); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.q[name] = append(q.q[name], payload)
	return nil
}

func (q *Queue) Dequeue(_ context.Context, name string) ([]byte, error) {
	q.Calls.add("Dequeue")
	if err := q.Errs.get("Dequeue"); err != nil {
		return nil, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.q[name]) == 0 {
		return nil, nil
	}
	head := q.q[name][0]
	q.q[name] = q.q[name][1:]
	return head, nil
}

// Messages returns a copy of the pending messages of a queue.
func (q *Queue) Messages(name string) [][]byte {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([][]byte(nil), q.q[name]...)
}

// ---------------------------------------------------------------- ObjectStore

// ObjectStore is an in-memory storage.ObjectStore. URLs are deterministic: GetUrl gives
// "https://get.test/<key>", presigned PUTs "https://put.test/<key>?size=<n>".
type ObjectStore struct {
	Errs  Errs
	Calls Calls
	mu    sync.Mutex
	// Objects holds stored bytes by key.
	Objects map[string][]byte
	// DeletedPrefixes records DeletePrefix calls in order.
	DeletedPrefixes []string
	// ParseFunc overrides ParseBucketEvent. By default the raw data is JSON-decoded into a storage.BucketEvent.
	ParseFunc func(raw string) (*storage.BucketEvent, error)
}

var _ storage.ObjectStore = (*ObjectStore)(nil)

func NewObjectStore() *ObjectStore { return &ObjectStore{Objects: map[string][]byte{}} }

func (s *ObjectStore) Put(_ context.Context, _ string, key string, file io.Reader, _ int64) (string, error) {
	s.Calls.add("Put")
	if err := s.Errs.get("Put"); err != nil {
		return "", err
	}
	b, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Objects[key] = b
	return key, nil
}

func (s *ObjectStore) GerPreSignedPutUrl(_ context.Context, key string, size int64) (string, error) {
	s.Calls.add("GerPreSignedPutUrl")
	if err := s.Errs.get("GerPreSignedPutUrl"); err != nil {
		return "", err
	}
	return fmt.Sprintf("https://put.test/%s?size=%d", key, size), nil
}

func (s *ObjectStore) Get(_ context.Context, key string) (io.ReadCloser, int64, error) {
	s.Calls.add("Get")
	if err := s.Errs.get("Get"); err != nil {
		return nil, 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.Objects[key]
	if !ok {
		return nil, 0, fmt.Errorf("object %q not found", key)
	}
	return io.NopCloser(strings.NewReader(string(b))), int64(len(b)), nil
}

func (s *ObjectStore) Delete(_ context.Context, key string) error {
	s.Calls.add("Delete")
	if err := s.Errs.get("Delete"); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.Objects, key)
	return nil
}

func (s *ObjectStore) GetUrl(_ context.Context, key string) (string, error) {
	s.Calls.add("GetUrl")
	if err := s.Errs.get("GetUrl"); err != nil {
		return "", err
	}
	return "https://get.test/" + key, nil
}

func (s *ObjectStore) SaveLocally(context.Context, string, string) error {
	s.Calls.add("SaveLocally")
	return s.Errs.get("SaveLocally")
}

func (s *ObjectStore) UploadLocalFile(_ context.Context, key, _ string, _ string) (string, error) {
	s.Calls.add("UploadLocalFile")
	if err := s.Errs.get("UploadLocalFile"); err != nil {
		return "", err
	}
	return key, nil
}

func (s *ObjectStore) ParseBucketEvent(raw string) (*storage.BucketEvent, error) {
	s.Calls.add("ParseBucketEvent")
	if err := s.Errs.get("ParseBucketEvent"); err != nil {
		return nil, err
	}
	if s.ParseFunc != nil {
		return s.ParseFunc(raw)
	}
	var e storage.BucketEvent
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *ObjectStore) DeletePrefix(_ context.Context, prefix string) error {
	s.Calls.add("DeletePrefix")
	s.mu.Lock()
	s.DeletedPrefixes = append(s.DeletedPrefixes, prefix)
	for k := range s.Objects {
		if strings.HasPrefix(k, prefix) {
			delete(s.Objects, k)
		}
	}
	s.mu.Unlock()
	return s.Errs.get("DeletePrefix")
}

// ---------------------------------------------------------------- VideoIndex

// VideoIndex is an in-memory index.VideoIndex. SearchVideo does a case-insensitive substring match
// on title/description and returns ids in insertion order. Suggestions are returned as seeded.
type VideoIndex struct {
	Errs        Errs
	Calls       Calls
	mu          sync.Mutex
	Docs        map[string]*models.Video
	order       []string
	Suggestions []index.Suggestion
	// ExtraIDs are appended to every search result (e.g. ids that are not uuids or not in Postgres).
	ExtraIDs []string
	// LastSearch records the offset/size of the last SearchVideo call.
	LastOffset, LastSize int
}

var _ index.VideoIndex = (*VideoIndex)(nil)

func NewVideoIndex() *VideoIndex { return &VideoIndex{Docs: map[string]*models.Video{}} }

func (x *VideoIndex) InsertVideo(_ context.Context, v *models.Video) error {
	x.Calls.add("InsertVideo")
	if err := x.Errs.get("InsertVideo"); err != nil {
		return err
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	id := v.ID.String()
	if _, ok := x.Docs[id]; !ok {
		x.order = append(x.order, id)
	}
	cp := *v
	x.Docs[id] = &cp
	return nil
}

func (x *VideoIndex) UpdateVideo(_ context.Context, v *models.Video) error {
	x.Calls.add("UpdateVideo")
	if err := x.Errs.get("UpdateVideo"); err != nil {
		return err
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	id := v.ID.String()
	if _, ok := x.Docs[id]; !ok {
		x.order = append(x.order, id)
	}
	cp := *v
	x.Docs[id] = &cp
	return nil
}

func (x *VideoIndex) SearchVideo(_ context.Context, query string, _ []string, offset, size int) ([]string, error) {
	x.Calls.add("SearchVideo")
	if err := x.Errs.get("SearchVideo"); err != nil {
		return nil, err
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	x.LastOffset, x.LastSize = offset, size
	q := strings.ToLower(query)
	var hits []string
	for _, id := range x.order {
		d := x.Docs[id]
		text := ""
		if d.Title != nil {
			text += strings.ToLower(*d.Title) + " "
		}
		if d.Description != nil {
			text += strings.ToLower(*d.Description)
		}
		if strings.Contains(text, q) {
			hits = append(hits, id)
		}
	}
	hits = append(hits, x.ExtraIDs...)
	if offset >= len(hits) {
		return []string{}, nil
	}
	hits = hits[offset:]
	if size < len(hits) {
		hits = hits[:size]
	}
	return hits, nil
}

func (x *VideoIndex) SuggestVideos(context.Context, string) ([]index.Suggestion, error) {
	x.Calls.add("SuggestVideos")
	if err := x.Errs.get("SuggestVideos"); err != nil {
		return nil, err
	}
	return x.Suggestions, nil
}

func (x *VideoIndex) DeleteVideo(_ context.Context, id string) error {
	x.Calls.add("DeleteVideo")
	if err := x.Errs.get("DeleteVideo"); err != nil {
		return err
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	delete(x.Docs, id)
	for i, o := range x.order {
		if o == id {
			x.order = append(x.order[:i], x.order[i+1:]...)
			break
		}
	}
	return nil
}

// IDs returns the indexed ids, sorted.
func (x *VideoIndex) IDs() []string {
	x.mu.Lock()
	defer x.mu.Unlock()
	ids := make([]string, 0, len(x.Docs))
	for id := range x.Docs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
