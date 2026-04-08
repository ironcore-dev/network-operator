// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

// Package nxapi provides a JSON-RPC client for Cisco NX-OS devices via NX-API.
//
// Use [NewClient] to create a client for a given [deviceutil.Connection].
// The client supports both HTTP and HTTPS (selected automatically based on
// whether a TLS configuration is present) and authenticates with HTTP Basic Auth.
//
// Commands are expressed as plain NX-OS CLI strings and grouped into a [Request]
// using [NewRequest]. A single HTTP POST is made per [Request], which may contain
// one or more commands. Each command in a batch gets its own result in the
// returned slice.
//
// Errors are surfaced as [RPCErrors] when NX-OS reports one or more command
// failures, or as [HTTPError] for non-2xx responses whose body cannot be parsed
// as JSON-RPC (for example, a 401 Authorization Required).
package nxapi
