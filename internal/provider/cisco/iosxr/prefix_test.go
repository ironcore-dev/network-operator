// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

func init() {
	prefixList := new(PrefixList)
	prefixList.Name = "PL-IPv4"
	prefixList.Is6 = false
	prefixList.Sequences.Sequence = gnmiext.List[int32, *PrefixEntry]{}

	p1 := &PrefixEntry{
		SequenceNumber: 10,
		Permission:     PermissionPermit,
		Prefix:         "10.10.0.0",
		Mask:           "255.255.0.0",
		MatchPrefixLength: &MatchPrefixLength{
			EQ: 16,
		},
	}

	p2 := &PrefixEntry{
		SequenceNumber: 20,
		Permission:     PermissionPermit,
		Prefix:         "192.168.1.0",
		Mask:           "255.255.255.0",
		MatchPrefixLength: &MatchPrefixLength{
			LE: 30,
		},
	}

	p3 := &PrefixEntry{
		SequenceNumber: 30,
		Permission:     PermissionPermit,
		Prefix:         "172.16.0.0",
		Mask:           "255.240.0.0",
		MatchPrefixLength: &MatchPrefixLength{
			GE: 12,
			LE: 24,
		},
	}

	prefixList.Sequences.Sequence.Set(p1)
	prefixList.Sequences.Sequence.Set(p2)
	prefixList.Sequences.Sequence.Set(p3)

	Register("prefix", prefixList)
}
