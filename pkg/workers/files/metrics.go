package filesworker

import (
	"context"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type metricsMiddleware struct {
	reqCount    *prometheus.CounterVec
	reqDuration *prometheus.HistogramVec
	worker      Worker
}

func (m *metricsMiddleware) RemoveUnpaidFiles(ctx context.Context) (interval time.Duration, err error) {
	defer func(s time.Time) {
		labels := []string{
			"RemoveUnpaidFiles", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.worker.RemoveUnpaidFiles(ctx)
}

func (m *metricsMiddleware) MarkToRemoveUnpaidFiles(ctx context.Context) (interval time.Duration, err error) {
	defer func(s time.Time) {
		labels := []string{
			"CleanupRemovedFiles", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.worker.MarkToRemoveUnpaidFiles(ctx)
}

func (m *metricsMiddleware) RemoveNotifiedFiles(ctx context.Context) (interval time.Duration, err error) {
	defer func(s time.Time) {
		labels := []string{
			"RemoveNotifiedFiles", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.worker.RemoveNotifiedFiles(ctx)
}

func (m *metricsMiddleware) TriggerProvidersDownload(ctx context.Context) (interval time.Duration, err error) {
	defer func(s time.Time) {
		labels := []string{
			"TriggerProvidersDownload", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.worker.TriggerProvidersDownload(ctx)
}

func (m *metricsMiddleware) DownloadChecker(ctx context.Context) (interval time.Duration, err error) {
	defer func(s time.Time) {
		labels := []string{
			"DownloadChecker", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.worker.DownloadChecker(ctx)
}

func (m *metricsMiddleware) CollectContractProvidersToNotify(ctx context.Context) (interval time.Duration, err error) {
	defer func(s time.Time) {
		labels := []string{
			"CollectContractProvidersToNotify", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.worker.CollectContractProvidersToNotify(ctx)
}

func NewMetrics(reqCount *prometheus.CounterVec, reqDuration *prometheus.HistogramVec, worker Worker) Worker {
	return &metricsMiddleware{
		reqCount:    reqCount,
		reqDuration: reqDuration,
		worker:      worker,
	}
}
