// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	p := &PrefixList{}
	p.Name = "TEST"
	p.EntItems.EntryList.Set(&PrefixEntry{
		Order:      10,
		Action:     ActionPermit,
		Criteria:   CriteriaInexact,
		Pfx:        "10.0.0.0/8",
		FromPfxLen: 24,
		ToPfxLen:   24,
	})
	Register("prefix", p)
}
