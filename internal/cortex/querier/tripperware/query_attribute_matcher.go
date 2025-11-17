// Copyright (c) The Cortex Authors.
// Licensed under the Apache License 2.0.

package tripperware

import (
	"regexp"
	"strings"
	"time"

	"github.com/thanos-io/thanos/internal/cortex/querier/queryrange"
)

// QueryAttributeMatcher matches queries based on various attributes
type QueryAttributeMatcher struct {
	QueryPatterns  []string   `yaml:"query_patterns"`
	ApiType        string     `yaml:"api_type"`
	TimeWindow     TimeWindow `yaml:"time_window"` // in seconds
	TimeRange      TimeRange  `yaml:"time_range"`
	QueryStepLimit StepLimit  `yaml:"query_step_limit"` // in milliseconds
	UserAgent      string     `yaml:"user_agent"`       // User-Agent to match against
	DashboardUID   string     `yaml:"dashboard_uid"`    // Dashboard UID to match against
	PanelID        string     `yaml:"panel_id"`         // Panel ID to match against
}

// TimeRange defines a time range for query matching
type TimeWindow struct {
	Start time.Time `yaml:"start"`
	End   time.Time `yaml:"end"`
}

// TimeRange defines a time range for query matching
type TimeRange struct {
	Min int `yaml:"min"` // Start time of the range
	Max int `yaml:"max"` // End time of the range
}

type StepLimit struct {
	Min int `yaml:"min"` // Minimum step limit in milliseconds
	Max int `yaml:"max"` // Maximum step limit in milliseconds
}

// Match checks if a query matches the configured patterns and time range
func (qam *QueryAttributeMatcher) Match(req queryrange.Request) bool {

	//Check for other attributes
	//if qam.ApiType != "" && req.GetApiType() != qam.ApiType {

	// Check query pattern matching
	if len(qam.QueryPatterns) > 0 {
		query := req.GetQuery()
		matched := false
		for _, pattern := range qam.QueryPatterns {
			if qam.matchQueryPattern(query, pattern) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check time range matching
	if !qam.TimeWindow.Start.IsZero() || !qam.TimeWindow.End.IsZero() {
		reqStart := time.Unix(req.GetStart()/1000, 0)
		reqEnd := time.Unix(req.GetEnd()/1000, 0)

		// Check if the query time range overlaps with the blocked time range
		// A query should be rejected if it overlaps with the blocked range
		if !qam.TimeWindow.Start.IsZero() && !qam.TimeWindow.End.IsZero() {
			// Query overlaps with blocked range if:
			// - query start is before blocked end AND query end is after blocked start
			if reqStart.Before(qam.TimeWindow.End) && reqEnd.After(qam.TimeWindow.Start) {
				return true // Match found - should be rejected
			}
		} else if !qam.TimeWindow.Start.IsZero() {
			// Only start time specified - reject if query starts before blocked start
			if reqStart.Before(qam.TimeWindow.Start) {
				return true // Match found - should be rejected
			}
		} else if !qam.TimeWindow.End.IsZero() {
			// Only end time specified - reject if query ends after blocked end
			if reqEnd.After(qam.TimeWindow.End) {
				return true // Match found - should be rejected
			}
		}
	}

	return true
}

// matchQueryPattern checks if a query matches a pattern
func (qam *QueryAttributeMatcher) matchQueryPattern(query, pattern string) bool {
	// Convert pattern to regex if it contains regex syntax
	if strings.Contains(pattern, "*") {
		// Convert glob pattern to regex
		regexPattern := strings.ReplaceAll(pattern, "*", ".*")
		regexPattern = "^" + regexPattern + "$"
		re, err := regexp.Compile(regexPattern)
		if err != nil {
			return false
		}
		return re.MatchString(query)
	}

	// Simple string matching
	return strings.Contains(query, pattern)
}
