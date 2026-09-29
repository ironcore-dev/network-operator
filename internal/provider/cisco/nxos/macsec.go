// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"fmt"
	"strings"
	"time"

	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var (
	_ gnmiext.DataElement = (*MacSecPolicy)(nil)
	_ gnmiext.DataElement = (*KeyChain)(nil)
)

// CipherSuite is the MACsec cipher suite as expected by the NX-OS DME model.
type CipherSuite string

const (
	CipherSuiteGcmAes256    CipherSuite = "GCM-AES-256"
	CipherSuiteGcmAes128    CipherSuite = "GCM-AES-128"
	CipherSuiteGcmAesXpn256 CipherSuite = "GCM-AES-XPN-256"
	CipherSuiteGcmAesXpn128 CipherSuite = "GCM-AES-XPN-128"
)

// ExtractCipherSuite validates the API cipher suite string and maps it to the
// NX-OS DME value.
func ExtractCipherSuite(cipherSuite string) (CipherSuite, error) {
	switch cipherSuite {
	case string(CipherSuiteGcmAes256):
		return CipherSuiteGcmAes256, nil
	case string(CipherSuiteGcmAes128):
		return CipherSuiteGcmAes128, nil
	case string(CipherSuiteGcmAesXpn256):
		return CipherSuiteGcmAesXpn256, nil
	case string(CipherSuiteGcmAesXpn128):
		return CipherSuiteGcmAesXpn128, nil
	default:
		return "", fmt.Errorf("unsupported cipher suite: %s", cipherSuite)
	}
}

// MacSecPolicy is the MACsec policy realized under
// System/macsec-items/inst-items/policy-items/Policy-list.
//
// Only the fields modeled by the API are carried here. Optional device knobs
// (includeSci, secPolicy, suspend*, lldpBypass, allowedPeerCipherSuite*, ...)
// are intentionally omitted so the device retains its platform defaults and no
// false diff is produced on subsequent reconciles.
type MacSecPolicy struct {
	PolicyName        string      `json:"policyName"`
	CipherSuite       CipherSuite `json:"cipherSuite,omitzero"`
	ConfOffset        uint16      `json:"confOffset,omitzero"`
	KeyServerPriority uint8       `json:"keySvrPrio,omitzero"`
	ReplayWindow      uint16      `json:"replayWindow,omitzero"`
}

func (m *MacSecPolicy) XPath() string {
	return "System/macsec-items/inst-items/policy-items/Policy-list[policyName=" + m.PolicyName + "]"
}

// KeyChain is the MACsec keychain realized separately from the policy.
//
// TODO(sven-rosenzweig): the nested key-list container tag and the per-key leaf
// names on MacSecKey are still best-guess and must be confirmed against a
// populated device get. The keychain list path and key (keychainName) are
// confirmed.
type KeyChain struct {
	Name    string                           `json:"keychainName"`
	KeyList gnmiext.List[string, *MacSecKey] `json:"Key-list,omitzero"`
}

func (k *KeyChain) XPath() string {
	return "System/kcmgr-items/keychains-items/macseckeychain-items/MacsecKeychain-list[keychainName=" + k.Name + "]"
}

// MacSecKey is a single pre-shared key within a keychain.
type MacSecKey struct {
	ID             string   `json:"keyId"`
	OctetString    string   `json:"octetString,omitzero"`
	CryptoAlg      string   `json:"cryptoAlg,omitzero"`
	SendLifetime   Lifetime `json:"sendLifetime,omitzero"`
	AcceptLifetime Lifetime `json:"acceptLifetime,omitzero"`
}

func (k *MacSecKey) Key() string { return k.ID }

// Lifetime expresses a key lifetime as a start time plus a duration in seconds.
type Lifetime struct {
	Duration  uint32    `json:"duration,omitzero"`
	StartTime CiscoTime `json:"startTime,omitzero"`
}

// CiscoTime is the calendar representation of a lifetime start time.
type CiscoTime struct {
	DayOfMonth int    `json:"dayOfMonth,omitzero"`
	Hour       int    `json:"hour,omitzero"`
	Minute     int    `json:"minute,omitzero"`
	Month      string `json:"month,omitzero"`
	Second     int    `json:"second,omitzero"`
	Year       int    `json:"year,omitzero"`
}

// NewLifetime parses an RFC3339 end time and returns the lifetime as a start
// time of now plus the duration until that end time.
func NewLifetime(endTime string) (Lifetime, error) {
	start := time.Now()

	end, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return Lifetime{}, err
	}
	duration := end.Sub(start)

	return Lifetime{
		Duration: uint32(duration.Seconds()),
		StartTime: CiscoTime{
			DayOfMonth: start.Day(),
			Hour:       start.Hour(),
			Minute:     start.Minute(),
			Month:      strings.ToLower(start.Month().String()),
			Second:     start.Second(),
			Year:       start.Year(),
		},
	}, nil
}
