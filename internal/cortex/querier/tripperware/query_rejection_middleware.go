// Copyright (c) The Cortex Authors.
// Licensed under the Apache License 2.0.

package tripperware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/weaveworks/common/httpgrpc"

	"github.com/thanos-io/thanos/internal/cortex/querier/queryrange"
	"github.com/thanos-io/thanos/internal/cortex/util/spanlogger"
)

// QueryRejectionConfig holds configuration for query rejection
type QueryRejectionConfig struct {
	BlockedQueries []QueryAttributeMatcher `yaml:"blocked_queries"`
}

// QueryRejectionMiddlewareMetrics holds metrics for query rejection
type QueryRejectionMiddlewareMetrics struct {
	rejectedQueries prometheus.Counter
}

// NewQueryRejectionMiddlewareMetrics creates new metrics for query rejection
func NewQueryRejectionMiddlewareMetrics(registerer prometheus.Registerer) *QueryRejectionMiddlewareMetrics {
	return &QueryRejectionMiddlewareMetrics{
		rejectedQueries: promauto.With(registerer).NewCounter(prometheus.CounterOpts{
			Namespace: "cortex",
			Name:      "query_frontend_rejected_queries_total",
			Help:      "Total number of queries rejected by query rejection middleware",
		}),
	}
}

type queryRejectionMiddleware struct {
	next    queryrange.Handler
	config  QueryRejectionConfig
	logger  log.Logger
	metrics *QueryRejectionMiddlewareMetrics
}

// NewQueryRejectionMiddleware creates a new middleware that rejects queries based on configured patterns
func NewQueryRejectionMiddleware(config QueryRejectionConfig, logger log.Logger, metrics *QueryRejectionMiddlewareMetrics) queryrange.Middleware {
	if metrics == nil {
		metrics = NewQueryRejectionMiddlewareMetrics(nil)
	}

	return queryrange.MiddlewareFunc(func(next queryrange.Handler) queryrange.Handler {
		return queryRejectionMiddleware{
			next:    next,
			config:  config,
			logger:  logger,
			metrics: metrics,
		}
	})
}

func (qrm queryRejectionMiddleware) Do(ctx context.Context, req queryrange.Request) (queryrange.Response, error) {
	log, ctx := spanlogger.New(ctx, "query_rejection")
	defer log.Finish()

	op := req.GetOperation()
	// Check if the query should be rejected
	for _, blockedQuery := range qrm.config.BlockedQueries {
		if blockedQuery.Match(req) {
			qrm.metrics.rejectedQueries.Inc()
			level.Info(log).Log(
				"msg", "query rejected by query rejection middleware",
				"query", req.GetQuery(),
				"start", req.GetStart(),
				"end", req.GetEnd(),
			)
			return nil, httpgrpc.Errorf(
				http.StatusBadRequest,
				"query rejected: %s",
				fmt.Sprintf("query '%s' matches blocked pattern", req.GetQuery()),
			)
		}
	}

	return qrm.next.Do(ctx, req)
}
