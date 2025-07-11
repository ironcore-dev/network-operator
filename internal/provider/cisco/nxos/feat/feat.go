// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package feat

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	nxos "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/genyang"
	"github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos/gnmiext"
)

var _ gnmiext.DeviceConf = Features{}

// Features the list of enabled features on the device.
// All features not listed here are considered disabled.
type Features []string

func (feat Features) ToYGOT(_ gnmiext.Client) ([]gnmiext.Update, error) {
	fm := &nxos.Cisco_NX_OSDevice_System_FmItems{}
	fm.PopulateDefaults()
	rv := reflect.ValueOf(fm).Elem()
	for _, f := range feat {
		name := strings.ToUpper(f[:1]) + strings.ToLower(f[1:])
		name = strings.TrimSuffix(name, "-items") + "Items"
		field := rv.FieldByName(name)
		if !field.IsValid() {
			return nil, fmt.Errorf("feat: feature %q not found in %T", f, fm)
		}
		state := field.Elem().FieldByName("AdminSt")
		if !state.IsValid() {
			return nil, fmt.Errorf("feat: field %q does not have AdminSt", name)
		}
		admin := nxos.Cisco_NX_OSDevice_Fm_AdminState_enabled
		if !state.Type().AssignableTo(reflect.TypeOf(admin)) {
			return nil, fmt.Errorf("feat: field '%s.AdminSt' is not assignable to %T", name, admin)
		}
		if !state.CanSet() {
			return nil, fmt.Errorf("feat: field '%s.AdminSt' cannot be set", name)
		}
		state.Set(reflect.ValueOf(admin))
	}
	return []gnmiext.Update{
		gnmiext.EditingUpdate{
			XPath: "System/fm-items",
			Value: fm,
		},
	}, nil
}

// do not support resetting features
func (v Features) Reset(_ gnmiext.Client) ([]gnmiext.Update, error) {
	return []gnmiext.Update{}, errors.New("feat: resetting features is not supported")
}
