// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

func init() {
	Register("lldp", &LLDP{
		HoldTime:  NewOption(uint16(200)),
		InitDelay: NewOption(uint16(5)),
	})

	items := new(LLDPIfItems)
	items.IfList.Set(&LLDPIfItem{
		InterfaceName: "eth7/1",
		AdminRxSt:     NewOption(AdminStDisabled),
		AdminTxSt:     NewOption(AdminStDisabled),
	})
	items.IfList.Set(&LLDPIfItem{
		InterfaceName: "eth8/1",
		AdminTxSt:     NewOption(AdminStDisabled),
	})
	Register("lldp_if_items", items)
}
