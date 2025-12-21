package elasticsearch_index

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ak-ansari/mytube/models"
	es_v9 "github.com/elastic/go-elasticsearch/v9"
)

type VideoEsIndex struct {
	client *es_v9.Client
	index  string
}
type Mappings struct {
	Properties map[string]map[string]string `json:"properties"`
}
type videoIndexConfig struct {
	Mappings Mappings `json:"mappings"`
}

func getCreateVideoIndexCnf() videoIndexConfig {
	return videoIndexConfig{
		Mappings: Mappings{
			Properties: map[string]map[string]string{
				"video_suggest": map[string]string{
					"type": "completion",
				},
				"title": map[string]string{
					"type": "text",
				},
				"description": map[string]string{
					"type": "text",
				},
			},
		},
	}
}

func NewVideoEsIndex(c *es_v9.Client) (*VideoEsIndex, error) {
	index := string(models.IndexVideo)
	res, err := c.Indices.Exists([]string{index})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		// crete the index
		indexCnf := getCreateVideoIndexCnf()
		indexBytes, err := json.Marshal(indexCnf)
		if err != nil {
			return nil, err
		}
		createRes, err := c.Indices.Create(
			index,
			c.Indices.Create.WithBody(bytes.NewReader(indexBytes)),
		)
		if err != nil {
			return nil, err
		}
		defer createRes.Body.Close()
		if createRes.IsError() {
			return nil, fmt.Errorf("error while creating index:%s,error:%s", index, createRes.String())
		}
	}
	return &VideoEsIndex{
		client: c,
		index:  index,
	}, nil
}

func (vi *VideoEsIndex) InsertVideo(ctx context.Context, v *models.Video) error {

	b, err := json.Marshal(map[string]string{
		"title":         *v.Title,
		"description":   *v.Description,
		"video_suggest": *v.Title + " " + *v.Description,
	})
	if err != nil {
		return err
	}

	res, err := vi.client.Index(
		vi.index,
		bytes.NewReader(b),
		vi.client.Index.WithDocumentID(v.ID.String()),
		vi.client.Index.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("ES index error: %s", res.String())
	}

	return nil
}
func (vi *VideoEsIndex) UpdateVideo(ctx context.Context, v *models.Video) error {
	return nil
}
func (vi *VideoEsIndex) SearchVideo(ctx context.Context, query string, fields []string, offset, size int) ([]string, error) {
	body := map[string]interface{}{
		"from": offset,
		"size": size,
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":     query,
				"fields":    fields,
				"fuzziness": "AUTO",
			},
		},
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, err
	}

	res, err := vi.client.Search(
		vi.client.Search.WithIndex(vi.index),
		vi.client.Search.WithBody(&buf),
		vi.client.Search.WithTrackTotalHits(true),
		vi.client.Search.WithContext(ctx),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("search error: %s", res.String())
	}

	var result struct {
		Hits struct {
			Hits []struct {
				ID string `json:"_id"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, err
	}

	var ids []string
	for _, h := range result.Hits.Hits {
		ids = append(ids, h.ID)
	}

	return ids, nil
}
func (vi *VideoEsIndex) SuggestVideos(ctx context.Context, query string) ([]string, error) {
	body := map[string]interface{}{
		"suggest": map[string]interface{}{
			"video-suggest": map[string]interface{}{
				"prefix": query,
				"completion": map[string]interface{}{
					"field":           "video_suggest",
					"skip_duplicates": true,
					"fuzzy": map[string]interface{}{
						"fuzziness": 1,
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return nil, err
	}

	res, err := vi.client.Search(
		vi.client.Search.WithContext(ctx),
		vi.client.Search.WithIndex(vi.index),
		vi.client.Search.WithBody(&buf),
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("ES suggest error: %s", res.String())
	}

	var r map[string]any
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}

	suggestions := []string{}

	// Parse ES response
	if s, ok := r["suggest"].(map[string]any)["video-suggest"].([]any); ok {
		for _, entry := range s {
			opts := entry.(map[string]any)["options"].([]any)
			for _, o := range opts {
				suggestions = append(suggestions, o.(map[string]any)["text"].(string))
			}
		}
	}

	return suggestions, nil
}
