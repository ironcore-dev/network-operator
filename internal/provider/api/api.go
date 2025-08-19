// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package api

// ConnectionDetails holds all the necessary information for connecting to a device.
type ConnectionDetails struct {
	Address  string
	Username string
	Password string
	Port     int
}

// DeviceSettingsConfig mirrors the spec of the Device CRD.
type DeviceSettingsConfig struct {
	Hostname       string
	NTPServers     []string
	ProviderConfig []byte
	// ... other device-wide settings like DNS, SNMP, banners, etc.
}

type VRF struct {
	Name           string // e.g., "VRF-Blue"
	Description    string // e.g., "Blue VRF for customer A"
	ProviderConfig []byte // Vendor-specific configuration, if needed
}

// DeviceConfig contains configuration for a network device.
type DeviceConfig struct {
	VRF            VRF
	ProviderConfig []byte // Vendor-specific configuration, if needed
}

// DeviceInfo provides a structured representation of key device properties.
type DeviceInfo struct {
	Vendor       string
	Model        string
	SerialNumber string
	OSVersion    string
}

// Interface provides basic identifying information about a physical network port.
type Interface struct {
	Name        string
	Description string
	AdminState  string
}

// LoopbackInterfaceConfig defines the desired declarative state for a logical loopback interface.
type LoopbackInterfaceConfig struct {
	Name           string
	Description    string
	IPv4Address    string // e.g., "192.168.1.1/32"
	ProviderConfig []byte
}

// VLANConfig defines the desired state for a Virtual LAN.
type VLANConfig struct {
	ID             int
	Name           string
	ProviderConfig []byte
}

// LAGConfig defines the desired state for a Link Aggregation Group (Port Channel).
type LAGConfig struct {
	Name string // e.g., "Port-Channel10"
	// PhysicalMemberPorts are the names of the physical interfaces to be bundled.
	// e.g., ["Ethernet1/1", "Ethernet1/2"].
	PhysicalMemberPorts []string
	// LACPMode, etc., could be added here.
	ProviderConfig []byte
}

// L3Config defines Layer 3 addressing for an interface.
type L3Config struct {
	IPv4Address string // e.g., "10.1.1.1/24"
	// IPv6Address, secondary IPs, etc., could be added here.
	ProviderConfig []byte
}

// InterfaceConfig now includes L2 and L3 configuration.
type InterfaceConfig struct {
	Name              string
	Description       string
	Enabled           bool
	AccessVLAN        int       // For an access port.
	TrunkAllowedVLANs []int     // For a trunk port.
	L3Config          *L3Config // Pointer to allow for nil (pure L2) config.
	ProviderConfig    []byte
}

// BGPConfig defines the desired state for the global BGP process.
type BGPConfig struct {
	ASNumber       int
	RouterID       string // Should be a stable loopback IP.
	ProviderConfig []byte
}

// BGPNeighborConfig defines the desired state for a BGP peer.
type BGPNeighborConfig struct {
	PeerAS          int
	NeighborAddress string
	ProviderConfig  []byte
	// ... other neighbor settings ...
}
