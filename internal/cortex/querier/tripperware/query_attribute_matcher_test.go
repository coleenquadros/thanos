// Copyright (c) The Cortex Authors.
// Licensed under the Apache License 2.0.

package tripperware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/thanos-io/thanos/internal/cortex/querier/queryrange"
)

func TestQueryAttributeMatcher(t *testing.T) {
	tests := []struct {
		name     string
		matcher  QueryAttributeMatcher
		query    string
		start    int64
		end      int64
		expected bool
	}{
		{
			name: "should match query pattern",
			matcher: QueryAttributeMatcher{
				QueryPatterns: []string{"expensive_query"},
			},
			query:    "expensive_query{job=\"test\"}",
			start:    time.Now().Add(-1 * time.Hour).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: true,
		},
		{
			name: "should not match different query pattern",
			matcher: QueryAttributeMatcher{
				QueryPatterns: []string{"expensive_query"},
			},
			query:    "simple_query{job=\"test\"}",
			start:    time.Now().Add(-1 * time.Hour).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: false,
		},
		{
			name: "should match glob pattern",
			matcher: QueryAttributeMatcher{
				QueryPatterns: []string{"expensive_*"},
			},
			query:    "expensive_query{job=\"test\"}",
			start:    time.Now().Add(-1 * time.Hour).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: true,
		},
		{
			name: "should match time range",
			matcher: QueryAttributeMatcher{
				TimeWindow: TimeWindow{
					Start: time.Now().Add(-2 * time.Hour),
					End:   time.Now().Add(-30 * time.Minute),
				},
			},
			query:    "any_query",
			start:    time.Now().Add(-90 * time.Minute).UnixMilli(),
			end:      time.Now().Add(-45 * time.Minute).UnixMilli(),
			expected: true,
		},
		{
			name: "should not match time range outside bounds",
			matcher: QueryAttributeMatcher{
				TimeWindow: TimeWindow{
					Start: time.Now().Add(-2 * time.Hour),
					End:   time.Now().Add(-1 * time.Hour),
				},
			},
			query:    "any_query",
			start:    time.Now().Add(-30 * time.Minute).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: false,
		},
		{
			name: "should match both query pattern and time range",
			matcher: QueryAttributeMatcher{
				QueryPatterns: []string{"expensive_query"},
				TimeWindow: TimeWindow{
					Start: time.Now().Add(-2 * time.Hour),
					End:   time.Now().Add(-30 * time.Minute),
				},
			},
			query:    "expensive_query{job=\"test\"}",
			start:    time.Now().Add(-90 * time.Minute).UnixMilli(),
			end:      time.Now().Add(-45 * time.Minute).UnixMilli(),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &queryrange.PrometheusRequest{
				Query: tt.query,
				Start: tt.start,
				End:   tt.end,
			}

			result := tt.matcher.Match(req)
			require.Equal(t, tt.expected, result)
		})
	}
}
