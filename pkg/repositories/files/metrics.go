package files

import (
	"github.com/prometheus/client_golang/prometheus"
)

type metricsMiddleware struct {
	reqCount    *prometheus.CounterVec
	reqDuration *prometheus.HistogramVec
	repo        Repository
}

func NewMetrics(reqCount *prometheus.CounterVec, reqDuration *prometheus.HistogramVec, repo Repository) Repository {
	return &metricsMiddleware{
		reqCount:    reqCount,
		reqDuration: reqDuration,
		repo:        repo,
	}
}
