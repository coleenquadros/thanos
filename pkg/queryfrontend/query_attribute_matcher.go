// Copyright (c) The Cortex Authors.
// Licensed under the Apache License 2.0.

package queryfrontend

import (
	"github.com/opentracing/opentracing-go/log"
	"github.com/prometheus/common/model"
	"regexp"
	"strings"
	"time"

	"github.com/thanos-io/thanos/internal/cortex/querier/queryrange"
)

type QueryAttributeMatcher struct {
	QueryPatterns  []string   `yaml:"query_patterns"`
	ApiType        string     `yaml:"api_type"`
	TimeWindow     TimeWindow `yaml:"time_window"`
	TimeRange      TimeRange  `yaml:"time_range"`
	QueryStepLimit StepLimit  `yaml:"query_step_limit"`
	UserAgent      string     `yaml:"user_agent"`
	DashboardUID   string     `yaml:"dashboard_uid"`
	PanelID        string     `yaml:"panel_id"`
}

type TimeWindow struct {
	Start model.Duration `yaml:"start"`
	End   model.Duration `yaml:"end"`
}

type TimeRange struct {
	Min model.Duration `yaml:"min"`
	Max model.Duration `yaml:"max"`
}

type StepLimit struct {
	Min model.Duration `yaml:"min"`
	Max model.Duration `yaml:"max"`
}

func (qam *QueryAttributeMatcher) Match(req queryrange.Request) bool {
	op := getReqType(req)
	if op == "range" || op == "instant" {
		if matchAttributesForExpressionQuery(req, qam) {
			return true
		}
	}

	if op == "labels" || op == "series" {
		if matchAttributesForMetadataQuery(req, qam) {
			return true
		}
	}

	return false
}

func matchAttributesForExpressionQuery(req queryrange.Request, qam *QueryAttributeMatcher) bool {
	if qam.ApiType != "" {
		reqType := getReqType(req)
		if reqType != qam.ApiType {
			return false
		}
	}
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

	if !matchesTimeWindow(qam.TimeWindow, req) {
		return false
	}

	if !matchesTimeRangeLimits(qam.TimeRange, req) {
		return false
	}

	if getReqType(req) == "range" {
		if !isWithinQueryStepLimit(qam.QueryStepLimit, req.GetStep()) {
			return false
		}
	}

	headers := getReqHeaders(req)
	if qam.DashboardUID != "" {
		if !isMatchDashboardId(headers, qam.DashboardUID) {
			return false
		}
	}

	if qam.PanelID != "" {
		if !isMatchPanelId(headers, qam.PanelID) {
			return false
		}
	}

	if qam.UserAgent != "" {
		return false
	}

	return true
}

func matchAttributesForMetadataQuery(req queryrange.Request, qam *QueryAttributeMatcher) bool {
	if qam.ApiType != "" {
		reqType := getReqType(req)
		if reqType != qam.ApiType {
			return false
		}
	}

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
	return true
}

// matchesTimeWindow checks whether the request time range overlaps the configured TimeWindow.
// Returns true when the request should be considered a match (i.e., blocked).
func matchesTimeWindow(timeWindow TimeWindow, req queryrange.Request) bool {
	if timeWindow.Start == 0 && timeWindow.End == 0 {
		return true
	}

	reqStart := req.GetStart()
	reqEnd := req.GetEnd()
	now := time.Now()

	if ireq, ok := req.(*ThanosQueryInstantRequest); ok {
		// For instant queries, both start and end are the same
		reqStart = ireq.Time
		reqEnd = ireq.Time
	}

	if timeWindow.Start != 0 {
		startTimeThreshold := now.Add(-1 * time.Duration(timeWindow.Start).Abs()).Add(-1 * time.Minute).Truncate(time.Minute).UnixMilli()
		if reqStart == 0 || reqStart < startTimeThreshold {
			return false
		}
	}
	if timeWindow.End != 0 {
		endTimeThreshold := now.Add(-1 * time.Duration(timeWindow.End).Abs()).Add(1 * time.Minute).Truncate(time.Minute).UnixMilli()
		if reqEnd == 0 || reqEnd > endTimeThreshold {
			return false
		}
	}

	return true
}

// matchesTimeRangeLimits checks whether the request time range falls within the configured TimeRange limits.
func matchesTimeRangeLimits(timeRange TimeRange, req queryrange.Request) bool {
	if timeRange.Min == 0 && timeRange.Max == 0 {
		return true
	}

	startTime := req.GetStart()
	endTime := req.GetEnd()

	if startTime == 0 || endTime == 0 {
		return false
	}

	duration := endTime - startTime

	if timeRange.Min != 0 && duration < time.Duration(timeRange.Min).Milliseconds() {
		return false
	}
	if timeRange.Max != 0 && duration > time.Duration(timeRange.Max).Milliseconds() {
		return false
	}
	return true
}

// isWithinQueryStepLimit checks whether the query step falls within the configured StepLimit.
func isWithinQueryStepLimit(limit StepLimit, step int64) bool {
	if limit.Min == 0 && limit.Max == 0 {
		return true
	}

	if limit.Min != 0 && step < time.Duration(limit.Min).Milliseconds() {
		return false
	}
	if limit.Max != 0 && step > time.Duration(limit.Max).Milliseconds() {
		return false
	}
	return true
}

// matchQueryPattern checks if a query matches a pattern using regex
func (qam *QueryAttributeMatcher) matchQueryPattern(query, pattern string) bool {
	if pattern == ".*" || pattern == ".+" {
		return true
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		log.Error(err)
		return false
	}

	// Use regex matching
	return re.MatchString(query)
}

// isGrafanaPanelQuery checks if the request is from a specific Grafana dashboard panel
func isMatchDashboardId(headers []*RequestHeader, dashboardUID string) bool {
	for _, header := range headers {
		if strings.ToLower(header.Name) == "x-dashboard-uid" {
			for _, value := range header.Values {
				if dashboardUID != "" && value == dashboardUID {
					return true
				}
			}
		}
	}
	return false
}

func isMatchPanelId(headers []*RequestHeader, panelID string) bool {
	for _, header := range headers {
		if strings.ToLower(header.Name) == "x-panel-id" {
			for _, value := range header.Values {
				if panelID != "" && value == panelID {
					return true
				}
			}
		}
	}
	return false
}

func getReqHeaders(req queryrange.Request) []*RequestHeader {
	var headers []*RequestHeader

	switch grafanaReq := req.(type) {
	case *ThanosQueryRangeRequest:
		headers = grafanaReq.GetHeaders()
	case *ThanosQueryInstantRequest:
		headers = grafanaReq.GetHeaders()
	case *ThanosLabelsRequest:
		headers = grafanaReq.GetHeaders()
	case *ThanosSeriesRequest:
		headers = grafanaReq.GetHeaders()
	default:
		return headers
	}

	return headers
}
