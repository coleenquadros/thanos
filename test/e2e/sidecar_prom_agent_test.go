// Copyright (c) The Thanos Authors.
// Licensed under the Apache License 2.0.

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"testing"
	"time"

	"github.com/efficientgo/core/testutil"
	"github.com/efficientgo/e2e"
	e2edb "github.com/efficientgo/e2e/db"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/thanos-io/objstore/client"
	"github.com/thanos-io/thanos/pkg/query"
	"github.com/thanos-io/thanos/pkg/runutil"
	"github.com/thanos-io/thanos/test/e2e/e2ethanos"
)

// e2e test to verify storeapi in sidecar is disabled when prometheus is running in agent mode.
func TestSidecarInPromAgentMode(t *testing.T) {
	t.Parallel()

	e, err := e2e.NewDockerEnvironment("e2e-test-pa")
	testutil.Ok(t, err)
	t.Cleanup(e2ethanos.CleanScenario(t, e))

	receiver := e2ethanos.NewReceiveBuilder(e, "1").WithIngestionEnabled().Init()
	testutil.Ok(t, e2e.StartAndWaitReady(receiver))

	prom1, sidecar1 := e2ethanos.NewPrometheusWithSidecar(e, "prom1", e2ethanos.DefaultPromConfig("prom-alone1", 0, "", "", e2ethanos.LocalPrometheusTarget), "", e2ethanos.DefaultPrometheusImage(), "")
	prom2, sidecar2 := e2ethanos.NewPrometheusWithSidecar(e, "prom2-agent", e2ethanos.DefaultPromConfig("prom-both-remote-write-and-sidecar", 1234, e2ethanos.RemoteWriteEndpoint(receiver.InternalEndpoint("remote-write")), "", e2ethanos.LocalPrometheusTarget), "", e2ethanos.DefaultPrometheusImage(), "", "agent")
	testutil.Ok(t, e2e.StartAndWaitReady(prom1, sidecar1, prom2, sidecar2))

	const bucket = "info-api-test"
	m := e2edb.NewMinio(e, "thanos-minio", bucket, e2edb.WithMinioTLS())
	testutil.Ok(t, e2e.StartAndWaitReady(m))
	store := e2ethanos.NewStoreGW(
		e,
		"1",
		client.BucketConfig{
			Type:   client.S3,
			Config: e2ethanos.NewS3Config(bucket, m.InternalEndpoint("http"), m.InternalDir()),
		},
		"",
		"",
		nil,
	)
	testutil.Ok(t, e2e.StartAndWaitReady(store))

	q := e2ethanos.NewQuerierBuilder(e, "1").
		WithTargetAddresses(sidecar1.InternalEndpoint("grpc")).
		WithMetadataAddresses(sidecar1.InternalEndpoint("grpc")).
		WithEndpoints(
			sidecar1.InternalEndpoint("grpc"),
			sidecar2.InternalEndpoint("grpc"),
			store.InternalEndpoint("grpc"),
		).
		Init()
	testutil.Ok(t, e2e.StartAndWaitReady(q))

	expected := map[string][]query.EndpointStatus{
		"sidecar": {
			{
				Name: "e2e-test-pa-sidecar-prom1:9091",
				LabelSets: []labels.Labels{{
					{
						Name:  "prometheus",
						Value: "prom-alone1",
					},
					{
						Name:  "replica",
						Value: "0",
					},
				}},
			},
			{
				Name:      "e2e-test-pa-sidecar-prom2-agent:9091",
				LabelSets: []labels.Labels{},
			},
		},
		"store": {
			{
				Name:      "e2e-test-pa-store-gw-1:9091",
				LabelSets: []labels.Labels{},
			},
		},
	}

	url := "http://" + path.Join(q.Endpoint("http"), "/api/v1/stores")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	err = runutil.Retry(time.Second, ctx.Done(), func() error {

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}

		body, err := io.ReadAll(resp.Body)
		defer runutil.CloseWithErrCapture(&err, resp.Body, "response body close")

		var res struct {
			Data map[string][]query.EndpointStatus `json:"data"`
		}

		err = json.Unmarshal(body, &res)
		if err != nil {
			return err
		}

		if err = assertPromAgentStoreStatus(t, "sidecar", res.Data, expected); err != nil {
			return err
		}

		if err = assertPromAgentStoreStatus(t, "store", res.Data, expected); err != nil {
			return err
		}

		return nil
	})
	testutil.Ok(t, err)
}

func assertPromAgentStoreStatus(t *testing.T, component string, res map[string][]query.EndpointStatus, expected map[string][]query.EndpointStatus) error {
	t.Helper()

	if len(res[component]) != len(expected[component]) {
		return fmt.Errorf("expected %d %s, got: %d", len(expected[component]), component, len(res[component]))
	}

	for i, v := range res[component] {
		// Set value of the fields which keep changing in every test run to their default value.
		v.MaxTime = 0
		v.MinTime = 0
		v.LastCheck = time.Time{}

		testutil.Equals(t, expected[component][i], v)
	}

	return nil
}