// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package nxos

import (
	"fmt"
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

// ConfOffset is the MACsec confidentiality offset as expected by the NX-OS DME
// model (macsec_ConfOffset enumeration).
type ConfOffset string

const (
	ConfOffset0  ConfOffset = "CONF-OFFSET-0"
	ConfOffset30 ConfOffset = "CONF-OFFSET-30"
	ConfOffset50 ConfOffset = "CONF-OFFSET-50"
)

// ExtractConfOffset validates the API confidentiality offset and maps it to the
// NX-OS DME enum value.
func ExtractConfOffset(offset uint16) (ConfOffset, error) {
	switch offset {
	case 0:
		return ConfOffset0, nil
	case 30:
		return ConfOffset30, nil
	case 50:
		return ConfOffset50, nil
	default:
		return "", fmt.Errorf("unsupported confidentiality offset: %d", offset)
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
	ConfOffset        ConfOffset  `json:"confOffset,omitzero"`
	KeyServerPriority uint8       `json:"keySvrPrio,omitzero"`
	ReplayWindow      uint32      `json:"replayWindow,omitzero"`
}

func (m *MacSecPolicy) XPath() string {
	return "System/macsec-items/inst-items/policy-items/Policy-list[policyName=" + m.PolicyName + "]"
}

// KeyChain is the MACsec keychain realized separately from the policy.
//
// The keys nest under the DME container macseckeyid-items, whose MacsecKeyId-list
// holds one entry per key.
type KeyChain struct {
	Name string      `json:"keychainName"`
	Keys MacSecKeyID `json:"macseckeyid-items,omitzero"`
}

// MacSecKeyID is the macseckeyid-items container wrapping the key list.
type MacSecKeyID struct {
	KeyList gnmiext.List[string, *MacSecKey] `json:"MacsecKeyId-list,omitzero"`
}

func (k *KeyChain) XPath() string {
	return "System/kcmgr-items/keychains-items/macseckeychain-items/MacsecKeychain-list[keychainName=" + k.Name + "]"
}

// CryptographicAlgo is the MACsec key cryptographic algorithm as expected by
// the NX-OS DME model (kcmgr_cryptographicAlgoAes).
type CryptographicAlgo string

const (
	CryptographicAlgoAes128Cmac CryptographicAlgo = "AES_128_CMAC"
	CryptographicAlgoAes256Cmac CryptographicAlgo = "AES_256_CMAC"
)

// ExtractCryptographicAlgo validates the API algorithm string and maps it to
// the NX-OS DME value.
func ExtractCryptographicAlgo(algorithm string) (CryptographicAlgo, error) {
	switch algorithm {
	case "aes-128-cmac":
		return CryptographicAlgoAes128Cmac, nil
	case "aes-256-cmac":
		return CryptographicAlgoAes256Cmac, nil
	default:
		return "", fmt.Errorf("unsupported cryptographic algorithm: %s", algorithm)
	}
}

// EncryptType is the MACsec key-string encryption type (kcmgr_encryptionTypeMacsec).
type EncryptType string

const (
	EncryptTypeUnencrypted EncryptType = "unencrypted"
	EncryptTypeType7       EncryptType = "type7"
	EncryptTypeType6       EncryptType = "type6"
)

// MacSecKey is a single pre-shared key within a keychain.
//
// MACsec keys have only a send-lifetime; the device has no accept-lifetime
// container for them (that exists for classic keychains only).
type MacSecKey struct {
	ID           string            `json:"keyId"`
	OctetString  string            `json:"keyHexString,omitzero"`
	EncryptType  EncryptType       `json:"encryptType,omitzero"`
	CryptoAlg    CryptographicAlgo `json:"cryptographicAlgo,omitzero"`
	SendLifetime Lifetime          `json:"macsecsendlifetime-items,omitzero"`
}

func (k *MacSecKey) Key() string { return k.ID }

// Lifetime is the MACsec send-lifetime realized under
// macseckeyid-items/MacsecKeyId-list/macsecsendlifetime-items. The start time
// is split into a clock string ("HH:MM:SS") and separate day/month/year leaves,
// matching the NX-OS DME model.
//
// Infinite is always sent as "disabled": the device reports it on every Get, so
// omitting it would produce a false diff and a Set on every reconcile.
type Lifetime struct {
	StartTime  string `json:"startTime,omitzero"`
	StartDay   uint16 `json:"startDay,omitzero"`
	StartMonth string `json:"startMonth,omitzero"`
	StartYear  uint16 `json:"startYear,omitzero"`
	Duration   uint32 `json:"duration,omitzero"`
	Infinite   string `json:"infinite"`
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
		StartTime:  start.Format("15:04:05"),
		StartDay:   uint16(start.Day()),
		StartMonth: start.Format("Jan"),
		StartYear:  uint16(start.Year()),
		Duration:   uint32(duration.Seconds()),
		Infinite:   "disabled",
	}, nil
}
