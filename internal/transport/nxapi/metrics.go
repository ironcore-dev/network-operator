// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxapi

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	rpcDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "network_operator",
			Name:      "nxapi_rpc_duration_seconds",
			Help:      "Duration of NX-API JSON-RPC requests in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"target", "status"},
	)
	rpcCommandsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "network_operator",
			Name:      "nxapi_rpc_commands_total",
			Help:      "Total number of NX-API CLI commands sent.",
		},
		[]string{"target", "command", "status"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		rpcDurationSeconds,
		rpcCommandsTotal,
	)
}
