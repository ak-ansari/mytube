package elastic

import (
	"fmt"

	"github.com/ak-ansari/mytube/internal/config"
	es_v9 "github.com/elastic/go-elasticsearch/v9"
)

func NewEsClient(cnf *config.Config) (*es_v9.Client, error) {
	es_config := es_v9.Config{
		Addresses: []string{fmt.Sprintf("http://%s:%s", cnf.Es.EsHost, cnf.Es.EsPort)},
	}
	client, err := es_v9.NewClient(es_config)
	if err != nil {
		return nil, err
	}
	return client, nil
}
