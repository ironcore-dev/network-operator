// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	Register("grpc", &GRPC{Cert: "trustpoint", CertClientRoot: "client_trustpoint", Port: 9339, UseVrf: DefaultVRFName})
	Register("gnmi", &GNMI{KeepAliveTimeout: 1200, MaxCalls: 4})
}
