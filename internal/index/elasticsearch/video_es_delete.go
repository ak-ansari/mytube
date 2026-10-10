package elasticsearch_index

import (
	"context"
	"fmt"
	"net/http"
)

func (vi *VideoEsIndex) DeleteVideo(ctx context.Context, id string) error {
	res, err := vi.client.Delete(vi.index, id, vi.client.Delete.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// a video that was never confirmed was never indexed
	if res.IsError() && res.StatusCode != http.StatusNotFound {
		return fmt.Errorf("ES delete error: %s", res.String())
	}
	return nil
}
