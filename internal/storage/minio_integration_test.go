//go:build integration

package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ak-ansari/mytube/internal/testutil/integration"
	"github.com/minio/minio-go/v7"
)

// newTestStore builds an S3Store on the throwaway bucket (default mytube-test, never the dev
// bucket) through the nginx S3 proxy on :9002. NewS3Store reads ./configs/config.yaml via
// config.GetConfig, so the store is assembled directly from the same client settings.
func newTestStore(t *testing.T) (*S3Store, *minio.Client, string) {
	t.Helper()
	client, s, prefix := integration.Minio(t)
	return &S3Store{client: client, bucket: s.Bucket, log: integration.Logger()}, client, prefix
}

func readAll(t *testing.T, rc io.ReadCloser) []byte {
	t.Helper()
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

var _ ObjectStore = (*S3Store)(nil)

func TestS3Store_PutGetDelete(t *testing.T) {
	st, client, p := newTestStore(t)
	ctx := context.Background()
	key := p + "originals/v1/original.mp4"
	body := []byte("fake video bytes")

	got, err := st.Put(ctx, "v1", key, bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if got != key {
		t.Fatalf("Put returned key %q, want %q", got, key)
	}
	info, err := client.StatObject(ctx, st.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if info.Size != int64(len(body)) || info.UserMetadata["Fileid"] != "v1" {
		t.Fatalf("stat = size %d meta %v", info.Size, info.UserMetadata)
	}

	rc, size, err := st.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len(body)) || string(readAll(t, rc)) != string(body) {
		t.Fatalf("Get size=%d", size)
	}

	if err := st.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.Get(ctx, key); err == nil {
		t.Fatal("Get after Delete should fail")
	}
	// S3 delete of a missing key is not an error
	if err := st.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
}

func TestS3Store_GetMissing(t *testing.T) {
	st, _, p := newTestStore(t)
	if _, _, err := st.Get(context.Background(), p+"nope"); err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestS3Store_DeletePrefix(t *testing.T) {
	st, client, p := newTestStore(t)
	ctx := context.Background()
	keys := []string{
		p + "segments/v1/master.m3u8",
		p + "segments/v1/360p/index.m3u8",
		p + "segments/v1/360p/seg0.ts",
		p + "segments/v10/master.m3u8", // shares the string prefix "segments/v1" but not "segments/v1/"
		p + "thumbnails/v1/v1.jpeg",
	}
	for _, k := range keys {
		if _, err := st.Put(ctx, "v1", k, strings.NewReader("x"), 1); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.DeletePrefix(ctx, p+"segments/v1/"); err != nil {
		t.Fatal(err)
	}
	var left []string
	for o := range client.ListObjects(ctx, st.bucket, minio.ListObjectsOptions{Prefix: p, Recursive: true}) {
		if o.Err != nil {
			t.Fatal(o.Err)
		}
		left = append(left, strings.TrimPrefix(o.Key, p))
	}
	if len(left) != 2 || left[0] != "segments/v10/master.m3u8" || left[1] != "thumbnails/v1/v1.jpeg" {
		t.Fatalf("remaining = %v", left)
	}
	// empty prefix match is fine
	if err := st.DeletePrefix(ctx, p+"nothing/here/"); err != nil {
		t.Fatal(err)
	}
}

func TestS3Store_PresignedPutThenHTTPPut(t *testing.T) {
	st, _, p := newTestStore(t)
	ctx := context.Background()
	key := p + "thumbnails/v1/custom.png"
	body := []byte("png bytes from the browser")

	raw, err := st.GerPreSignedPutUrl(ctx, key, int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != integration.MinioFromEnv().Endpoint || u.Query().Get("X-Amz-Signature") == "" {
		t.Fatalf("unexpected presigned url %s", raw)
	}

	req, _ := http.NewRequest(http.MethodPut, raw, bytes.NewReader(body))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("presigned PUT status = %d", res.StatusCode)
	}

	rc, size, err := st.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len(body)) || string(readAll(t, rc)) != string(body) {
		t.Fatal("uploaded content mismatch")
	}

	// a tampered signature is rejected
	q := u.Query()
	q.Set("X-Amz-Signature", strings.Repeat("0", 64))
	u.RawQuery = q.Encode()
	req, _ = http.NewRequest(http.MethodPut, u.String(), bytes.NewReader(body))
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("tampered PUT status = %d, want 403", res.StatusCode)
	}
}

func TestS3Store_PresignedGetAndPrivateBucket(t *testing.T) {
	st, _, p := newTestStore(t)
	ctx := context.Background()
	key := p + "avatars/u1/a.png"
	if _, err := st.Put(ctx, "u1", key, strings.NewReader("avatar"), 6); err != nil {
		t.Fatal(err)
	}

	raw, err := st.GetUrl(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Get(raw)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusOK || string(b) != "avatar" {
		t.Fatalf("presigned GET = %d %q", res.StatusCode, b)
	}

	// nginx on :9002 rejects unsigned requests
	u, _ := url.Parse(raw)
	u.RawQuery = ""
	res, err = http.Get(u.String())
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("unsigned GET status = %d, want 403", res.StatusCode)
	}
}

func TestS3Store_SaveLocallyAndUploadLocalFile(t *testing.T) {
	st, client, p := newTestStore(t)
	ctx := context.Background()
	dir := t.TempDir()
	src := filepath.Join(dir, "in.m3u8")
	if err := os.WriteFile(src, []byte("#EXTM3U\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	key := p + "segments/v1/master.m3u8"
	got, err := st.UploadLocalFile(ctx, key, src, "application/vnd.apple.mpegurl")
	if err != nil || got != key {
		t.Fatalf("UploadLocalFile = %q, %v", got, err)
	}
	info, err := client.StatObject(ctx, st.bucket, key, minio.StatObjectOptions{})
	if err != nil || info.ContentType != "application/vnd.apple.mpegurl" {
		t.Fatalf("content type = %q, %v", info.ContentType, err)
	}

	dst := filepath.Join(dir, "out.m3u8")
	if err := st.SaveLocally(ctx, key, dst); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(dst)
	if string(b) != "#EXTM3U\n" {
		t.Fatalf("saved = %q", b)
	}
	if err := st.SaveLocally(ctx, p+"missing", filepath.Join(dir, "x")); err == nil {
		t.Fatal("expected error for missing key")
	}
}
