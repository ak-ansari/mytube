package observability

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// dbCollector reads pool stats and a few business gauges from Postgres on every scrape.
type dbCollector struct {
	pool     *pgxpool.Pool
	business bool

	poolTotal, poolIdle, poolInUse, poolMax *prometheus.Desc
	videos, deadLetters                     *prometheus.Desc
}

// RegisterDBCollector adds pgx pool gauges (labelled with process) and, when business is set,
// mytube_videos{stage,status} and mytube_dead_letter_jobs{source,stage}. Turn business on in one process only,
// or those gauges are scraped twice.
func RegisterDBCollector(pool *pgxpool.Pool, process string, business bool) {
	constLabels := prometheus.Labels{"process": process}
	prometheus.MustRegister(&dbCollector{
		pool:        pool,
		business:    business,
		poolTotal:   prometheus.NewDesc("mytube_db_pool_conns", "Open pgx pool connections.", nil, constLabels),
		poolIdle:    prometheus.NewDesc("mytube_db_pool_idle_conns", "Idle pgx pool connections.", nil, constLabels),
		poolInUse:   prometheus.NewDesc("mytube_db_pool_acquired_conns", "Acquired pgx pool connections.", nil, constLabels),
		poolMax:     prometheus.NewDesc("mytube_db_pool_max_conns", "pgx pool size limit.", nil, constLabels),
		videos:      prometheus.NewDesc("mytube_videos", "Videos by pipeline stage and status.", []string{"stage", "status"}, nil),
		deadLetters: prometheus.NewDesc("mytube_dead_letter_jobs", "Dead-lettered pipeline work by source and stage.", []string{"source", "stage"}, nil),
	})
}

func (d *dbCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range []*prometheus.Desc{d.poolTotal, d.poolIdle, d.poolInUse, d.poolMax, d.videos, d.deadLetters} {
		ch <- desc
	}
}

func (d *dbCollector) Collect(ch chan<- prometheus.Metric) {
	s := d.pool.Stat()
	ch <- prometheus.MustNewConstMetric(d.poolTotal, prometheus.GaugeValue, float64(s.TotalConns()))
	ch <- prometheus.MustNewConstMetric(d.poolIdle, prometheus.GaugeValue, float64(s.IdleConns()))
	ch <- prometheus.MustNewConstMetric(d.poolInUse, prometheus.GaugeValue, float64(s.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(d.poolMax, prometheus.GaugeValue, float64(s.MaxConns()))

	if !d.business {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// a failed query just leaves the gauge out of this scrape
	d.collectCounts(ctx, ch, d.videos, `SELECT COALESCE(stage, 0), COALESCE(status, ''), count(*) FROM videos GROUP BY 1, 2`)
	d.collectCounts(ctx, ch, d.deadLetters, `SELECT source, COALESCE(stage, 0), count(*) FROM dead_letter_jobs GROUP BY 1, 2`)
}

// collectCounts expects rows of (label1 text|int, label2 text|int, count).
func (d *dbCollector) collectCounts(ctx context.Context, ch chan<- prometheus.Metric, desc *prometheus.Desc, query string) {
	rows, err := d.pool.Query(ctx, query)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil || len(vals) != 3 {
			return
		}
		n, ok := vals[2].(int64)
		if !ok {
			return
		}
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(n), label(vals[0]), label(vals[1]))
	}
}

func label(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case int32:
		return strconv.Itoa(int(t))
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return ""
	}
}
