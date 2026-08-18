// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package gnmiext

import (
	"strings"

	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	operationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "network_operator",
			Name:      "gnmi_operations_total",
			Help:      "Total number of gNMI operations performed.",
		},
		[]string{"target", "rpc", "operation", "path", "status"},
	)
	rpcDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "network_operator",
			Name:      "gnmi_rpc_duration_seconds",
			Help:      "Duration of gNMI RPCs in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"target", "rpc", "status"},
	)
	operationsSkippedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "network_operator",
			Name:      "gnmi_operations_skipped_total",
			Help:      "Total number of gNMI operations skipped due to config being unchanged.",
		},
		[]string{"target", "operation", "path"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		operationsTotal,
		rpcDurationSeconds,
		operationsSkippedTotal,
	)
}

// metricPath renders a gnmipb.Path as a string with key values replaced by "*".
func metricPath(p *gnmipb.Path) string {
	var sb strings.Builder
	for _, e := range p.GetElem() {
		sb.WriteByte('/')
		sb.WriteString(e.GetName())
		for k := range e.GetKey() {
			sb.WriteByte('[')
			sb.WriteString(k)
			sb.WriteString("=*]")
		}
	}
	return sb.String()
}
