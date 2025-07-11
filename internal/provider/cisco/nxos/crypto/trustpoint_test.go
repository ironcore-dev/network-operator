// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"reflect"
	"testing"

	"github.com/openconfig/ygot/ygot"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

func Test_Trustpoint(t *testing.T) {
	tp := &Trustpoints{{ID: "mytrustpoint"}}

	got, err := tp.ToYGOT(&gnmiext.ClientMock{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Errorf("expected 1 key, got %d", len(got))
	}

	update, ok := got[0].(gnmiext.EditingUpdate)
	if !ok {
		t.Errorf("expected value to be of type EditingUpdate")
	}

	if update.XPath != "System/userext-items/pkiext-items/tp-items" {
		t.Errorf("expected key 'System/userext-items/pkiext-items/tp-items' to be present")
	}

	ti, ok := update.Value.(*nxos.Cisco_NX_OSDevice_System_UserextItems_PkiextItems_TpItems)
	if !ok {
		t.Errorf("expected value to be of type *nxos.Cisco_NX_OSDevice_System_UserextItems_PkiextItems_TpItems")
	}

	want := &nxos.Cisco_NX_OSDevice_System_UserextItems_PkiextItems_TpItems{
		TPList: map[string]*nxos.Cisco_NX_OSDevice_System_UserextItems_PkiextItems_TpItems_TPList{
			"mytrustpoint": {
				Name:            ygot.String("mytrustpoint"),
				KeyLabel:        ygot.String("mytrustpoint"),
				KeyType:         nxos.Cisco_NX_OSDevice_Pki_KeyType_Type_RSA,
				RevokeCheckConf: nxos.Cisco_NX_OSDevice_Pki_CertRevokeCheck_crl,
				EnrollmentType:  nxos.Cisco_NX_OSDevice_Pki_CertEnrollType_none,
			},
		},
	}
	if !reflect.DeepEqual(ti, want) {
		t.Errorf("unexpected value for 'System/userext-items/pkiext-items/tp-items': got=%+v, want=%+v", ti, want)
	}
}
