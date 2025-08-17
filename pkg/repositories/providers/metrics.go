package providers

import (
	"context"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"mytonstorage-backend/pkg/models/db"
)

type metricsMiddleware struct {
	reqCount    *prometheus.CounterVec
	reqDuration *prometheus.HistogramVec
	repo        Repository
}

func (m *metricsMiddleware) AddProviderToNotifyQueue(ctx context.Context, notifications []db.ProviderNotification) (err error) {
	defer func(s time.Time) {
		labels := []string{
			"AddProviderToNotifyQueue", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.repo.AddProviderToNotifyQueue(ctx, notifications)
}

func (m *metricsMiddleware) GetProvidersToNotify(ctx context.Context, limit int, notifyAttempts int) (notifications []db.ProviderNotification, err error) {
	defer func(s time.Time) {
		labels := []string{
			"GetProvidersToNotify", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.repo.GetProvidersToNotify(ctx, limit, notifyAttempts)
}

func (m *metricsMiddleware) IncreaseNotifyAttempts(ctx context.Context, notifications []db.ProviderNotification) (err error) {
	defer func(s time.Time) {
		labels := []string{
			"IncreaseAttempts", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.repo.IncreaseNotifyAttempts(ctx, notifications)
}

func (m *metricsMiddleware) ArchiveNotifications(ctx context.Context, notifications []db.ProviderNotification) (err error) {
	defer func(s time.Time) {
		labels := []string{
			"ArchiveNotifications", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.repo.ArchiveNotifications(ctx, notifications)
}

func NewMetrics(reqCount *prometheus.CounterVec, reqDuration *prometheus.HistogramVec, repo Repository) Repository {
	return &metricsMiddleware{
		reqCount:    reqCount,
		reqDuration: reqDuration,
		repo:        repo,
	}
}
