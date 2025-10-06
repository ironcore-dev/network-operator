// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tidwall/gjson"

	"github.com/ironcore-dev/network-operator/internal/provider/cisco/gnmiext/v2"
)

type TestCase struct {
	name string
	val  gnmiext.Configurable
}

var tests []TestCase

func Register(name string, val gnmiext.Configurable) {
	tests = append(tests, TestCase{
		name: name,
		val:  val,
	})
}

func Test_Payload(t *testing.T) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b, err := json.Marshal(test.val)
			if err != nil {
				t.Errorf("json.Marshal() error = %v", err)
				return
			}

			file := "testdata/" + test.name + ".json"
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("os.ReadFile(%q) error = %v", file, err)
			}

			var buf bytes.Buffer
			if err := json.Compact(&buf, data); err != nil {
				t.Errorf("json.Compact() error = %v", err)
				return
			}

			xpath, _ := strings.CutPrefix(test.val.XPath(), "System/")
			path, err := gnmiext.StringToStructuredPath(xpath)
			if err != nil {
				t.Errorf("StringToStructuredPath(%q) error = %v", xpath, err)
				return
			}

			var sb strings.Builder
			for _, elem := range path.GetElem() {
				if elem.GetName() == "" {
					continue
				}
				if sb.Len() > 0 {
					sb.WriteByte('|')
				}
				sb.WriteString(elem.GetName())
				if len(elem.GetKey()) == 0 {
					continue
				}
				i := 0
				for k, v := range elem.GetKey() {
					if i > 0 {
						sb.WriteString(`#`)
					}
					sb.WriteByte('|')
					sb.WriteString(`#(`)
					sb.WriteString(k)
					sb.WriteString(`=="`)
					sb.WriteString(v)
					sb.WriteString(`")`)
					i++
				}
			}

			res := gjson.GetBytes(buf.Bytes(), sb.String())
			if want := []byte(res.Raw); !bytes.Equal(want, b) {
				t.Errorf("payload mismatch:\nwant: %s\ngot:  %s", want, b)
			}
		})
	}
}
