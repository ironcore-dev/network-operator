// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package iosxr

import (
	"fmt"
	"strings"
	"time"

	"github.com/ironcore-dev/network-operator/internal/transport/gnmiext"
)

var (
	_ gnmiext.DataElement = (*MacSecPolicy)(nil)
	_ gnmiext.DataElement = (*KeyChain)(nil)
	_ gnmiext.DataElement = (*KeyChainOperData)(nil)
)

const ConfOffsetPrefix = "CONF-OFFSET-%d"

type CipherSuite string

const (
	CipherSuiteGcmAes256    CipherSuite = "GCM-AES-256"
	CipherSuiteGcmAes128    CipherSuite = "GCM-AES-128"
	CipherSuiteGcmAesXpn256 CipherSuite = "GCM-AES-XPN-256"
	CipherSuiteGcmAesXpn128 CipherSuite = "GCM-AES-XPN-128"
)

type CryptographicAlgorithm string

const (
	CryptographicAlgorithmMd5          CryptographicAlgorithm = "md5"
	CryptographicAlgorithmSha1         CryptographicAlgorithm = "sha-1"
	CryptographicAlgorithmHmacMd5      CryptographicAlgorithm = "hmac-md5"
	CryptographicAlgorithmHmacSha1_20  CryptographicAlgorithm = "hmac-sha1-20"
	CryptographicAlgorithmHmacSha1_12  CryptographicAlgorithm = "hmac-sha1-12"
	CryptographicAlgorithmHmacSha1_96  CryptographicAlgorithm = "hmac-sha1-96"
	CryptographicAlgorithmHmacSha256   CryptographicAlgorithm = "hmac-sha-256"
	CryptographicAlgorithmAes128Cmac96 CryptographicAlgorithm = "aes-128-cmac-96"
)

type MacSecPolicy struct {
	Name              string      `json:"-"`
	CipherSuite       CipherSuite `json:"cipher-suite,omitzero"`
	ConfOffset        string      `json:"conf-offset,omitzero"`
	KeyServerPriority uint8       `json:"key-server-priority,omitzero"`
	RelayProtection   uint16      `json:"window-size,omitzero"`
}

func (m *MacSecPolicy) XPath() string {
	return "Cisco-IOS-XR-um-macsec-cfg:macsec-policy/policy-names/policy-name[policy-name=" + m.Name + "]"
}

func ExtractCipherSuite(cipherSuite string) (CipherSuite, error) {
	switch cipherSuite {
	case "GCM-AES-256":
		return CipherSuiteGcmAes256, nil
	case "GCM-AES-128":
		return CipherSuiteGcmAes128, nil
	case "GCM-AES-XPN-256":
		return CipherSuiteGcmAesXpn256, nil
	case "GCM-AES-XPN-128":
		return CipherSuiteGcmAesXpn128, nil
	default:
		return "", fmt.Errorf("unsupported cipher suite: %s", cipherSuite)
	}
}

func ExtractCryptographicAlgorithm(cipherSuite string) (string, error) {
	switch cipherSuite {
	case "md5":
		return string(CryptographicAlgorithmMd5), nil
	case "sha-1":
		return string(CryptographicAlgorithmSha1), nil
	case "hmac-md5":
		return string(CryptographicAlgorithmHmacMd5), nil
	case "hmac-sha1-20":
		return string(CryptographicAlgorithmHmacSha1_20), nil
	case "hmac-sha1-12":
		return string(CryptographicAlgorithmHmacSha1_12), nil
	case "hmac-sha1-96":
		return string(CryptographicAlgorithmHmacSha1_96), nil
	case "hmac-sha-256":
		return string(CryptographicAlgorithmHmacSha256), nil
	case "aes-128-cmac-96":
		return string(CryptographicAlgorithmAes128Cmac96), nil
	default:
		return "", fmt.Errorf("unsupported cryptographic algorithm: %s", cipherSuite)
	}
}

type KeyChain struct {
	Name string `json:"-"`
	Keys Keys   `json:"keys,omitzero"`
}

type Keys struct {
	Key []Key `json:"key,omitzero"`
}

type Key struct {
	ID                     string       `json:"key-name,omitzero"`
	CryptographicAlgorithm string       `json:"cryptographic-algorithm,omitzero"`
	PreSharedKey           PreSharedKey `json:"key-string,omitzero"`
	StartLifetime          Lifetime     `json:"send-lifetime,omitzero"`
	AcceptLifetime         Lifetime     `json:"accept-lifetime,omitzero"`
}

type PreSharedKey struct {
	Secret string `json:"password,omitzero"`
}

type Lifetime struct {
	Duration  uint32    `json:"duration,omitzero"`
	StartTime CiscoTime `json:"start-time,omitzero"`
}

type CiscoTime struct {
	DayOfMonth int    `json:"day-of-month,omitzero"`
	Hour       int    `json:"hour,omitzero"`
	Minute     int    `json:"minute,omitzero"`
	Month      string `json:"month,omitzero"`
	Second     int    `json:"second,omitzero"`
	Year       int    `json:"year,omitzero"`
}

func (k *KeyChain) XPath() string {
	return "Cisco-IOS-XR-um-key-chain-cfg:key/chains/chain[key-chain-name=" + k.Name + "]"
}

func NewLifetime(endTime string) (Lifetime, error) {
	start := time.Now()

	end, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return Lifetime{}, err
	}
	duration := end.Sub(start)

	t := new(CiscoTime)
	t.DayOfMonth = end.Day()
	t.Hour = end.Hour()
	t.Minute = end.Minute()
	t.Month = strings.ToLower(end.Month().String())
	t.Second = end.Second()
	t.Year = end.Year()

	return Lifetime{
		Duration:  uint32(duration.Seconds()),
		StartTime: *t,
	}, nil
}

func (k *KeyChainOperData) XPath() string {
	return "Cisco-IOS-XR-lib-keychain-oper:keychain/keys/key[key-name=" + k.Name + "]"
}

// KeyChain operational data structures for reading state information
type KeyChainOperData struct {
	Name string  `json:"-"`
	Key  KeyOper `json:"key"`
}

type KeyOper struct {
	Keys []KeyStatus `json:"key-id"`
}

type KeyStatus struct {
	ID                     string         `json:"key-id"`
	AcceptLifetime         StatusLifetime `json:"accept-lifetime"`
	CryptographicAlgorithm string         `json:"cryptographic-algorithm"`
	SendLifetime           StatusLifetime `json:"send-lifetime"`
	Type                   string         `json:"type"`
}

type StatusLifetime struct {
	Duration      string `json:"duration"`
	IsAlwaysValid bool   `json:"is-always-valid"`
	IsValidNow    bool   `json:"is-valid-now"`
	Start         string `json:"start"`
}

type KeyIDOperational struct {
	AcceptLifetime         LifetimeOperational `json:"accept-lifetime"`
	CryptographicAlgorithm string              `json:"cryptographic-algorithm"`
	KeyID                  string              `json:"key-id"`
	SendLifetime           LifetimeOperational `json:"send-lifetime"`
	Type                   string              `json:"type"`
}

type LifetimeOperational struct {
	Duration      string `json:"duration"`
	IsAlwaysValid bool   `json:"is-always-valid"`
	IsValidNow    bool   `json:"is-valid-now"`
	Start         string `json:"start"`
}
