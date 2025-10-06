// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package nxos

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
)

// VPNIPv4Address represents a VPN-IPv4 Address Family as per RFC 4364.
type VPNIPv4Address struct {
	Type AFTYPE
	// Administrator is the ASN or IP address
	Administrator string
	// AssignedNumber is stored as a 4-byte unsigned integer (uint32). While for Type 0 addresses, it is a 4-byte value;
	// for Type 1 and Type 2 addresses, it is a only a 2-byte value.
	AssignedNumber uint32
}

// String returns the string representation of the VPNIPv4Address as per cisco conventions.
func (a *VPNIPv4Address) String() string {
	nnStr := "nn2"
	if a.AssignedNumber > 65535 {
		nnStr = "nn4"
	}
	switch a.Type {
	case AFType0:
		return fmt.Sprintf("as2-%s:%s:%d", nnStr, a.Administrator, a.AssignedNumber)
	case AFType1:
		return fmt.Sprintf("ipv4-%s:%s:%d", nnStr, a.Administrator, a.AssignedNumber)
	case AFType2:
		return fmt.Sprintf("as4-%s:%s:%d", nnStr, a.Administrator, a.AssignedNumber)
	default:
		return fmt.Sprintf("Unknown VPNIPv4Address type: %s", a.Type)
	}
}

type AFTYPE string

const (
	AFType0 AFTYPE = "type-0"
	AFType1 AFTYPE = "type-1"
	AFType2 AFTYPE = "type-2"
)

var (
	// TYPE0: ASN(2byte):AssignedNumber(4bytes)
	reType0 = regexp.MustCompile(`^(0|[1-9][0-9]{0,4}):(0|[1-9][0-9]{0,9})$`)
	// TYPE1: IPaddress(4bytes):AssignedNumber(2bytes)
	reType1 = regexp.MustCompile(`^([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}):(0|[1-9][0-9]{0,4})$`)
	// TYPE2: ASN(4byte):AssignedNumber(2bytes)
	reType2 = regexp.MustCompile(`^([1-9][0-9]{0,9}):(0|[1-9][0-9]{0,4})$`)
)

// NewVPNIPv4Address creates a new VPNIPv4Address based on the provided address family type and value.
// The value must be in the format defined by the type:
//   - Type 0: <2-byte ASN>:<4-byte AssignedNumber> (e.g., "65000:100")
//   - Type 1: <IPv4 address>:<2-byte AssignedNumber> (e.g, "1.2.3.4:100")
//   - Type 2: <4-byte ASN>:<2-byte AssignedNumber> (e.g., "65000:100")
//
// This function returns an error if the value does not conform to the expected format or if the ASN or assigned number is out of range. Also:
//   - Type 0: the ASN numbers 0, 65535 are reserved and not allowed by this function.
//   - Type 1:
//   - Type 2: the ASN numbers 0, 4294967295 are reserved and not allowed by this function.
func NewVPNIPv4Address(afType AFTYPE, value string) (*VPNIPv4Address, error) {
	if value == "" {
		return nil, errors.New("value cannot be empty")
	}
	v := &VPNIPv4Address{
		Type: afType,
	}
	switch afType {
	case AFType0:
		err := v.parseValueType0(value)
		if err != nil {
			return nil, fmt.Errorf("invalid address for type 0: %w", err)
		}
	case AFType1:
		err := v.parseValueType1(value)
		if err != nil {
			return nil, fmt.Errorf("invalid address for type 1: %w", err)
		}
	case AFType2:
		err := v.parseValueType2(value)
		if err != nil {
			return nil, fmt.Errorf("invalid address for type 2: %w", err)
		}
	default:
		return nil, fmt.Errorf("invalid address family type: %s", afType)
	}
	return v, nil
}

// validateValueType0 validates the route target value for Type 0 (ASN:AssignedNumber), where:
// ASN: 2 bytes (0-65535)
// Assigned Number: 4 bytes (0-4294967295)
func (a *VPNIPv4Address) parseValueType0(value string) error {
	matches := reType0.FindStringSubmatch(value)
	if matches == nil {
		return errors.New("route target value must be in the format <ASN>:<AssignedNumber> (Type 0 RD)")
	}
	asn, err := strconv.Atoi(matches[1])
	if err != nil || asn <= 0 || asn >= 65535 {
		return errors.New("administrator subfield (ASN) must be a number between 1 and 65534")
	}
	assigned, err := strconv.ParseUint(matches[2], 10, 64)
	if err != nil || assigned > 4294967295 {
		return errors.New("assigned number subfield must be a number between 1 and 4294967294")
	}
	a.Administrator = matches[1]
	a.AssignedNumber = uint32(assigned)
	return nil
}

func (a *VPNIPv4Address) parseValueType1(value string) error {
	matches := reType1.FindStringSubmatch(value)
	if matches == nil {
		return errors.New("route target value must be in the format <IPaddress>:<AssignedNumber> (Type 1 RD)")
	}
	parsedIP := net.ParseIP(matches[1])
	if parsedIP == nil || parsedIP.To4() == nil {
		return fmt.Errorf("invalid IP address format in route target value %s", matches[1])
	}
	assigned, err := strconv.Atoi(matches[2])
	if err != nil || assigned < 0 || assigned > 65535 {
		return errors.New("assigned number subfield must be a number between 1 and 65534")
	}
	a.Administrator = matches[1]
	a.AssignedNumber = uint32(assigned)
	return nil
}

func (a *VPNIPv4Address) parseValueType2(value string) error {
	matches := reType2.FindStringSubmatch(value)
	if matches == nil {
		return errors.New("route target value must be in the format <ASN>:<AssignedNumber> (Type 2 RD)")
	}
	asn, err := strconv.Atoi(matches[1])
	if err != nil || asn <= 0 || asn >= 4294967295 {
		return errors.New("administrator subfield (ASN) must be a number between 1 and 4294967294")
	}
	assigned, err := strconv.Atoi(matches[2])
	if err != nil || assigned < 0 || assigned > 65535 {
		return errors.New("assigned number subfield must be a number between 1 and 65534")
	}
	a.Administrator = matches[1]
	a.AssignedNumber = uint32(assigned)
	return nil
}
