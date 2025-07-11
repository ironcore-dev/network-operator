// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package banner

import (
	"strconv"
	"testing"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func Test_Banner(t *testing.T) {
	message := "Sample MoTD Banner Message: Region: [region1] | Name: [switch1] | IP: [1.1.1.1]"

	banner := &Banner{
		Delimiter: "^",
		Message:   message,
	}

	got, err := banner.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Errorf("expected 1 key, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/userext-items/preloginbanner-items" {
		t.Errorf("expected key 'System/userext-items/preloginbanner-items' to be present")
	}

	_, ok = update.Value.(*nxos.Cisco_NX_OSDevice_System_UserextItems_PreloginbannerItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_UserextItems_PreloginbannerItems")
	}
}

func Test_Banner_Limit(t *testing.T) {
	tests := []string{
		"Sample MoTD Banner Message: Region: [region1]    |  Name: [switch1]    |  IP: [1.1.1.1]",
		"\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n", // 41 lines
	}
	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			banner := &Banner{Delimiter: "^", Message: test}
			if _, err := banner.ToYGOT(&gnmiext.ClientMock{}); err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}
