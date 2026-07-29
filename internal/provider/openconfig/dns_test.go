// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package openconfig

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func Test_DNS_Payload(t *testing.T) {
	servers := &DNSServers{}
	address := "1.1.1.1"
	servers.Server.Set(&DNSServer{Address: address, Config: &DNSServerConfig{Address: address}})

	d := &DNS{
		Config:  &DNSConfig{Search: []string{"example.com"}},
		Servers: servers,
	}

	got, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	data, err := os.ReadFile("testdata/dns.json")
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	var want bytes.Buffer
	if err := json.Compact(&want, data); err != nil {
		t.Fatalf("json.Compact() error = %v", err)
	}

	if !bytes.Equal(want.Bytes(), got) {
		t.Errorf("payload mismatch:\nwant: %s\n got: %s", want.Bytes(), got)
	}
}
