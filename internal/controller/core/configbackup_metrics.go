// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var configBackupSizeBytes = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "configbackup_size_bytes",
		Help:    "Observed size of successful config backups.",
		Buckets: []float64{1 << 10, 1 << 12, 1 << 14, 1 << 16, 1 << 18, 1 << 20, 1 << 22, 1 << 24, 1 << 26},
	},
	[]string{"type"},
)

func init() {
	metrics.Registry.MustRegister(configBackupSizeBytes)
}
