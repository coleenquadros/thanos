// Copyright (c) The Cortex Authors.
// Licensed under the Apache License 2.0.

package tripperware

import (
	"context"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/thanos-io/thanos/internal/cortex/querier/queryrange"
)

func TestQueryRejectionMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		config        QueryRejectionConfig
		query         string
		start, end    int64
		shouldReject  bool
		expectedError string
	}{
		{
			name: "should reject query matching pattern",
			config: QueryRejectionConfig{
				BlockedQueries: []QueryAttributeMatcher{
					{
						QueryPatterns: []string{"expensive_query"},
					},
				},
			},
			query:         "expensive_query{job=\"test\"}",
			start:         time.Now().Add(-1 * time.Hour).UnixMilli(),
			end:           time.Now().UnixMilli(),
			shouldReject:  true,
			expectedError: "query rejected: query 'expensive_query{job=\"test\"}' matches blocked pattern",
		},
		{
			name: "should not reject query not matching pattern",
			config: QueryRejectionConfig{
				BlockedQueries: []QueryAttributeMatcher{
					{
						QueryPatterns: []string{"expensive_query"},
					},
				},
			},
			query:        "simple_query{job=\"test\"}",
			start:        time.Now().Add(-1 * time.Hour).UnixMilli(),
			end:          time.Now().UnixMilli(),
			shouldReject: false,
		},
		{
			name: "should reject query matching time range",
			config: QueryRejectionConfig{
				BlockedQueries: []QueryAttributeMatcher{
					{
						TimeRange: TimeRange{
							Start: time.Now().Add(-2 * time.Hour),
							End:   time.Now().Add(-1 * time.Hour),
						},
					},
				},
			},
			query:        "any_query",
			start:        time.Now().Add(-90 * time.Minute).UnixMilli(),
			end:          time.Now().Add(-30 * time.Minute).UnixMilli(),
			shouldReject: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewQueryRejectionMiddleware(tt.config, log.NewNopLogger(), nil)

			// Create a mock request
			req := &queryrange.PrometheusRequest{
				Query: tt.query,
				Start: tt.start,
				End:   tt.end,
			}

			// Create a mock handler that always succeeds
			mockHandler := queryrange.HandlerFunc(func(ctx context.Context, r queryrange.Request) (queryrange.Response, error) {
				return &queryrange.PrometheusResponse{Status: "success"}, nil
			})

			// Wrap with middleware
			wrappedHandler := middleware.Wrap(mockHandler)

			// Execute
			resp, err := wrappedHandler.Do(context.Background(), req)

			if tt.shouldReject {
				require.Error(t, err)
				require.Nil(t, resp)
				require.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
			}
		})
	}
}
