// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"strconv"
	"strings"
)

// ASCIIStr represents a string, encoded as a sequence of comma-separated ASCII code points (0 - 127).
type ASCIIStr string

// String decodes the ASCIIStr into a regular string, stopping at the first null character (ASCII code 0).
func (s ASCIIStr) String() string {
	var runes []rune
	for _, v := range strings.Split(string(s), ",") {
		if v == "0" {
			break
		}
		if num, err := strconv.Atoi(v); err == nil {
			runes = append(runes, rune(num))
		}
	}
	return string(runes)
}
