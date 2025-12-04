// Copyright (c) The Cortex Authors.
// Licensed under the Apache License 2.0.

package queryfrontend

import (
	"github.com/prometheus/common/model"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQueryAttributeMatcher_ApiType(t *testing.T) {
	tests := []struct {
		name     string
		matcher  QueryAttributeMatcher
		query    string
		expected bool
	}{
		{
			name: "should match api type range",
			matcher: QueryAttributeMatcher{
				ApiType: "range",
			},
			query:    "any_query",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (range)", func(t *testing.T) {
			req := &ThanosQueryRangeRequest{
				Query: tt.query,
				Start: time.Now().Add(-1 * time.Hour).UnixMilli(),
				End:   time.Now().UnixMilli(),
			}

			result := tt.matcher.Match(req)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestQueryAttributeMatcher_QueryPatterns(t *testing.T) {
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
				QueryPatterns: []string{"expensive_query", "another_query"},
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
			name: "should match anchored regex pattern",
			matcher: QueryAttributeMatcher{
				QueryPatterns: []string{"^expensive_.*$"},
			},
			query:    "expensive_query{job=\"test\"}",
			start:    time.Now().Add(-1 * time.Hour).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: true,
		},
		{
			name: "should not match anchored regex pattern when not at start",
			matcher: QueryAttributeMatcher{
				QueryPatterns: []string{"^expensive_.*$"},
			},
			query:    "some_expensive_query{job=\"test\"}",
			start:    time.Now().Add(-1 * time.Hour).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: false,
		},
		{
			name: "should match alternation regex pattern",
			matcher: QueryAttributeMatcher{
				QueryPatterns: []string{"(cpu|memory)_usage"},
			},
			query:    "cpu_usage{job=\"test\"}",
			start:    time.Now().Add(-1 * time.Hour).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: true,
		},
		{
			name: "should match character class regex pattern",
			matcher: QueryAttributeMatcher{
				QueryPatterns: []string{"[0-9]+_errors"},
			},
			query:    "500_errors{job=\"test\"}",
			start:    time.Now().Add(-1 * time.Hour).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: true,
		},
		{
			name: "should not match character class when pattern doesn't fit",
			matcher: QueryAttributeMatcher{
				QueryPatterns: []string{"[0-9]+_errors"},
			},
			query:    "abc_errors{job=\"test\"}",
			start:    time.Now().Add(-1 * time.Hour).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: false,
		},
		{
			name: "should match case-insensitive regex pattern",
			matcher: QueryAttributeMatcher{
				QueryPatterns: []string{"(?i)ERROR"},
			},
			query:    "error_count{job=\"test\"}",
			start:    time.Now().Add(-1 * time.Hour).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: true,
		},
		{
			name: "should match universal regex pattern .*",
			matcher: QueryAttributeMatcher{
				QueryPatterns: []string{".*"},
			},
			query:    "any_query_here{job=\"test\"}",
			start:    time.Now().Add(-1 * time.Hour).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: true,
		},
		{
			name: "should match universal regex pattern .+",
			matcher: QueryAttributeMatcher{
				QueryPatterns: []string{".+"},
			},
			query:    "any_non_empty_query{job=\"test\"}",
			start:    time.Now().Add(-1 * time.Hour).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: true,
		},
		{
			name: "should fail for for invalid regex",
			matcher: QueryAttributeMatcher{
				QueryPatterns: []string{"invalid[regex"},
			},
			query:    "query_with_invalid[regex_pattern{job=\"test\"}",
			start:    time.Now().Add(-1 * time.Hour).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (range)", func(t *testing.T) {
			req := &ThanosQueryRangeRequest{
				Query: tt.query,
				Start: tt.start,
				End:   tt.end,
			}

			result := tt.matcher.Match(req)
			require.Equal(t, tt.expected, result)
		})

		t.Run(tt.name+" (instant)", func(t *testing.T) {
			req := &ThanosQueryInstantRequest{
				Query: tt.query,
				Time:  tt.end,
			}
			result := tt.matcher.Match(req)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestQueryAttributeMatcher_TimeWindow(t *testing.T) {
	tests := []struct {
		name     string
		matcher  QueryAttributeMatcher
		query    string
		start    int64
		end      int64
		expected bool
	}{
		{
			name: "should match time window",
			matcher: QueryAttributeMatcher{
				TimeWindow: TimeWindow{
					Start: model.Duration(2 * time.Hour),
					End:   model.Duration(30 * time.Minute),
				},
			},
			query:    "any_query",
			start:    time.Now().Add(-90 * time.Minute).UnixMilli(),
			end:      time.Now().Add(-45 * time.Minute).UnixMilli(),
			expected: true,
		},
		{
			name: "should not match time window outside bounds",
			matcher: QueryAttributeMatcher{
				TimeWindow: TimeWindow{
					Start: model.Duration(2 * time.Hour),
					End:   model.Duration(1 * time.Hour),
				},
			},
			query:    "any_query",
			start:    time.Now().Add(-30 * time.Minute).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (range)", func(t *testing.T) {
			req := &ThanosQueryRangeRequest{
				Query: tt.query,
				Start: tt.start,
				End:   tt.end,
			}

			result := tt.matcher.Match(req)
			require.Equal(t, tt.expected, result)
		})

		t.Run(tt.name+" (instant)", func(t *testing.T) {
			req := &ThanosQueryInstantRequest{
				Query: tt.query,
				Time:  tt.end,
			}
			result := tt.matcher.Match(req)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestQueryAttributeMatcher_CombinedAttributes(t *testing.T) {
	tests := []struct {
		name     string
		matcher  QueryAttributeMatcher
		query    string
		start    int64
		end      int64
		expected bool
	}{
		{
			name: "should match both query pattern and time window",
			matcher: QueryAttributeMatcher{
				QueryPatterns: []string{"expensive_query"},
				TimeWindow: TimeWindow{
					Start: model.Duration(2 * time.Hour),
					End:   model.Duration(30 * time.Minute),
				},
			},
			query:    "expensive_query{job=\"test\"}",
			start:    time.Now().Add(-90 * time.Minute).UnixMilli(),
			end:      time.Now().Add(-45 * time.Minute).UnixMilli(),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (range)", func(t *testing.T) {
			req := &ThanosQueryRangeRequest{
				Query: tt.query,
				Start: tt.start,
				End:   tt.end,
			}

			result := tt.matcher.Match(req)
			require.Equal(t, tt.expected, result)
		})

		t.Run(tt.name+" (instant)", func(t *testing.T) {
			req := &ThanosQueryInstantRequest{
				Query: tt.query,
				Time:  tt.end,
			}
			result := tt.matcher.Match(req)
			require.Equal(t, tt.expected, result)
		})
	}
}

// Add test cases for checking time range
func TestQueryAtrributeMatcher_matchesTimeRange(t *testing.T) {
	tests := []struct {
		name       string
		timeWindow TimeRange
		start      int64
		end        int64
		expected   bool
	}{
		{
			name: "should match time range within limits",
			timeWindow: TimeRange{
				Min: model.Duration(1 * time.Hour),
				Max: model.Duration(3 * time.Hour),
			},
			start:    time.Now().Add(-2 * time.Hour).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: true,
		},
		{
			name: "should not match time range below minimum",
			timeWindow: TimeRange{
				Min: model.Duration(2 * time.Hour),
				Max: model.Duration(4 * time.Hour),
			},
			start:    time.Now().Add(-1 * time.Hour).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: false,
		},
		{
			name: "should not match time range above maximum",
			timeWindow: TimeRange{
				Min: model.Duration(1 * time.Hour),
				Max: model.Duration(2 * time.Hour),
			},
			start:    time.Now().Add(-3 * time.Hour).UnixMilli(),
			end:      time.Now().UnixMilli(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" (range)", func(t *testing.T) {
			req := &ThanosQueryRangeRequest{
				Query: "any_query",
				Start: tt.start,
				End:   tt.end,
			}

			result := matchesTimeRangeLimits(tt.timeWindow, req)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsWithinQueryStepLimit(t *testing.T) {
	tests := []struct {
		name     string
		limit    StepLimit
		step     int64
		expected bool
	}{
		{
			name: "should match step within limits",
			limit: StepLimit{
				Min: model.Duration(5 * time.Second),
				Max: model.Duration(2 * time.Minute),
			},
			step:     int64((1 * time.Minute).Milliseconds()),
			expected: true,
		},
		{
			name: "should not match step below minimum",
			limit: StepLimit{
				Min: model.Duration(2 * time.Minute),
				Max: model.Duration(10 * time.Minute),
			},
			step:     int64((1 * time.Minute).Milliseconds()),
			expected: false,
		},
		{
			name: "should not match step above maximum",
			limit: StepLimit{
				Min: model.Duration(1 * time.Minute),
				Max: model.Duration(5 * time.Minute),
			},
			step:     int64((10 * time.Minute).Milliseconds()),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isWithinQueryStepLimit(tt.limit, tt.step)
			require.Equal(t, tt.expected, result)
		})
	}
}
