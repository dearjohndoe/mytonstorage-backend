package files

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

func (m *metricsMiddleware) AddBag(ctx context.Context, bag db.BagInfo, userAddr string) (err error) {
	defer func(s time.Time) {
		labels := []string{
			"AddBag", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.repo.AddBag(ctx, bag, userAddr)
}

func (m *metricsMiddleware) RemoveUserBagRelation(ctx context.Context, bagID, userAddress string) (cnt int64, err error) {
	defer func(s time.Time) {
		labels := []string{
			"RemoveUserBagRelation", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.repo.RemoveUserBagRelation(ctx, bagID, userAddress)
}

func (m *metricsMiddleware) RemoveUnusedBags(ctx context.Context) (removed []string, err error) {
	defer func(s time.Time) {
		labels := []string{
			"RemoveUnusedBags", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.repo.RemoveUnusedBags(ctx)
}

func (m *metricsMiddleware) GetUnpaidBags(ctx context.Context, userID string) (bags []db.UserBagInfo, err error) {
	defer func(s time.Time) {
		labels := []string{
			"GetUnpaidBags", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.repo.GetUnpaidBags(ctx, userID)
}

func (m *metricsMiddleware) MarkBagAsPaid(ctx context.Context, bagID, userAddress, storageContract string) (cnt int64, err error) {
	defer func(s time.Time) {
		labels := []string{
			"MarkBagAsPaid", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.repo.MarkBagAsPaid(ctx, bagID, userAddress, storageContract)
}

func (m *metricsMiddleware) GetNotifyInfo(ctx context.Context, limit int, notifyAttempts int) (resp []db.BagStorageContract, err error) {
	defer func(s time.Time) {
		labels := []string{
			"GetNotifyInfo", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.repo.GetNotifyInfo(ctx, limit, notifyAttempts)
}

func (m *metricsMiddleware) IncreaseAttempts(ctx context.Context, bags []db.BagStorageContract) (err error) {
	defer func(s time.Time) {
		labels := []string{
			"IncreaseAttempts", strconv.FormatBool(err != nil),
		}
		m.reqCount.WithLabelValues(labels...).Add(1)
		m.reqDuration.WithLabelValues(labels...).Observe(time.Since(s).Seconds())
	}(time.Now())
	return m.repo.IncreaseAttempts(ctx, bags)
}

func NewMetrics(reqCount *prometheus.CounterVec, reqDuration *prometheus.HistogramVec, repo Repository) Repository {
	return &metricsMiddleware{
		reqCount:    reqCount,
		reqDuration: reqDuration,
		repo:        repo,
	}
}
