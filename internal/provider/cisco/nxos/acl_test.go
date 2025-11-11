// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	acl := &ACL{Name: "TEST-ACL"}
	acl.SeqItems.ACEList.Set(&ACLEntry{
		SeqNum:          10,
		Action:          ActionPermit,
		Protocol:        ProtocolIP,
		SrcPrefix:       "10.0.0.0",
		SrcPrefixLength: 8,
		DstPrefix:       "0.0.0.0",
		DstPrefixLength: 0,
	})
	Register("acl", acl)
}
