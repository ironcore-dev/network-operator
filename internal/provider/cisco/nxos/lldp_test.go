// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	lldp := &LLDP{
		HoldTime:  NewOption(uint16(200)),
		InitDelay: NewOption(uint16(5)),
	}

	lldp.IfItems.IfList.Set(&LLDPIfItem{
		InterfaceName: "eth7/1",
		AdminRxSt:     NewOption(AdminStDisabled),
		AdminTxSt:     NewOption(AdminStDisabled),
	})

	lldp.IfItems.IfList.Set(&LLDPIfItem{
		InterfaceName: "eth8/1",
		AdminTxSt:     NewOption(AdminStDisabled),
	})

	Register("lldp", lldp)
}
