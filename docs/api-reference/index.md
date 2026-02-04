# API Reference

## Packages
- [networking.metal.ironcore.dev/v1alpha1](#networking-metal-ironcore-dev-v1alpha1)
- [nx.cisco.networking.metal.ironcore.dev/v1alpha1](#nx-cisco-networking-metal-ironcore-dev-v1alpha1)
- [xe.cisco.networking.metal.ironcore.dev/v1alpha1](#xe-cisco-networking-metal-ironcore-dev-v1alpha1)
- [xr.cisco.networking.metal.ironcore.dev/v1alpha1](#xr-cisco-networking-metal-ironcore-dev-v1alpha1)


## networking.metal.ironcore.dev/v1alpha1

Package v1alpha1 contains API Schema definitions for the networking.metal.ironcore.dev v1alpha1 API group.

SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
SPDX-License-Identifier: Apache-2.0

### Resource Types
- [BGP](#bgp)
- [BGPPeer](#bgppeer)
- [Banner](#banner)
- [Certificate](#certificate)
- [DNS](#dns)
- [Device](#device)
- [EVPNInstance](#evpninstance)
- [ISIS](#isis)
- [Interface](#interface)
- [ManagementAccess](#managementaccess)
- [NTP](#ntp)
- [NetworkVirtualizationEdge](#networkvirtualizationedge)
- [OSPF](#ospf)
- [PIM](#pim)
- [PrefixSet](#prefixset)
- [RoutingPolicy](#routingpolicy)
- [SNMP](#snmp)
- [Syslog](#syslog)
- [User](#user)
- [VLAN](#vlan)
- [VRF](#vrf)



#### ACLAction

_Underlying type:_ _string_

ACLAction represents the type of action that can be taken by an ACL rule.

_Validation:_
- Enum: [Permit Deny]

_Appears in:_
- [ACLEntry](#aclentry)

| Field | Description |
| --- | --- |
| `Permit` | ActionPermit allows traffic that matches the rule.<br /> |
| `Deny` | ActionDeny blocks traffic that matches the rule.<br /> |


#### ACLEntry







_Appears in:_
- [AccessControlListSpec](#accesscontrollistspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `sequence` _integer_ | The sequence number of the ACL entry. |  | Minimum: 1 <br />Required: \{\} <br /> |
| `action` _[ACLAction](#aclaction)_ | The forwarding action of the ACL entry. |  | Enum: [Permit Deny] <br />Required: \{\} <br /> |
| `protocol` _[Protocol](#protocol)_ | The protocol to match. If not specified, defaults to "IP".<br />Available options are: ICMP, IP, OSPF, PIM, TCP, UDP. | IP | Enum: [ICMP IP OSPF PIM TCP UDP] <br />Optional: \{\} <br /> |
| `sourceAddress` _[IPPrefix](#ipprefix)_ | Source IP address prefix. Can be IPv4 or IPv6.<br />Use 0.0.0.0/0 (::/0) to represent 'any'. |  | Format: cidr <br />Type: string <br />Required: \{\} <br /> |
| `destinationAddress` _[IPPrefix](#ipprefix)_ | Destination IP address prefix. Can be IPv4 or IPv6.<br />Use 0.0.0.0/0 (::/0) to represent 'any'. |  | Format: cidr <br />Type: string <br />Required: \{\} <br /> |
| `description` _string_ | Description provides a human-readable description of the ACL entry. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |






#### AddressFamily

_Underlying type:_ _string_

AddressFamily represents the address family of an ISIS instance.

_Validation:_
- Enum: [IPv4Unicast IPv6Unicast]

_Appears in:_
- [ISISSpec](#isisspec)

| Field | Description |
| --- | --- |
| `IPv4Unicast` |  |
| `IPv6Unicast` |  |


#### AddressFamilyStatus



AddressFamilyStatus defines the prefix exchange statistics for a single address family (e.g., IPv4-Unicast).



_Appears in:_
- [BGPPeerStatus](#bgppeerstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `afiSafi` _[BGPAddressFamilyType](#bgpaddressfamilytype)_ | AfiSafi identifies the address family and subsequent address family. |  | Enum: [IPv4Unicast IPv6Unicast L2vpnEvpn] <br />Required: \{\} <br /> |
| `acceptedPrefixes` _integer_ | AcceptedPrefixes is the number of prefixes received from the peer that have passed the inbound policy<br />and are stored in the neighbor-specific table (Adj-RIB-In). |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `advertisedPrefixes` _integer_ | AdvertisedPrefixes is the number of prefixes currently being advertised to the peer after passing<br />the outbound policy. This reflects the state of the outbound routing table for the peer (Adj-RIB-Out). |  | Minimum: 0 <br />Optional: \{\} <br /> |


#### AdminState

_Underlying type:_ _string_

AdminState represents the administrative state of a resource.
This type is used across multiple resources including interfaces, protocols (BGP, OSPF, ISIS, PIM),
and system services (NTP, DNS) to indicate whether these are administratively enabled or disabled.

_Validation:_
- Enum: [Up Down]

_Appears in:_
- [BGPPeerSpec](#bgppeerspec)
- [BGPSpec](#bgpspec)
- [BorderGatewaySpec](#bordergatewayspec)
- [DNSSpec](#dnsspec)
- [ISISSpec](#isisspec)
- [InterfaceSpec](#interfacespec)
- [NTPSpec](#ntpspec)
- [NetworkVirtualizationEdgeSpec](#networkvirtualizationedgespec)
- [OSPFSpec](#ospfspec)
- [PIMSpec](#pimspec)
- [Peer](#peer)
- [VLANSpec](#vlanspec)
- [VPCDomainSpec](#vpcdomainspec)

| Field | Description |
| --- | --- |
| `Up` | AdminStateUp indicates that the resource is administratively enabled.<br /> |
| `Down` | AdminStateDown indicates that the resource is administratively disabled.<br /> |


#### Aggregation







_Appears in:_
- [InterfaceSpec](#interfacespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `memberInterfaceRefs` _[LocalObjectReference](#localobjectreference) array_ | MemberInterfaceRefs is a list of interface references that are part of the aggregate interface. |  | MaxItems: 32 <br />MinItems: 1 <br />Required: \{\} <br /> |
| `controlProtocol` _[ControlProtocol](#controlprotocol)_ | ControlProtocol defines the lacp configuration for the aggregate interface. | \{ mode:Active \} | Optional: \{\} <br /> |
| `multichassis` _[MultiChassis](#multichassis)_ | Multichassis defines the multichassis configuration for the aggregate interface. |  | Optional: \{\} <br /> |


#### AnycastGateway



AnycastGateway defines distributed anycast gateway configuration.
Multiple NVEs in the fabric share the same virtual MAC address,
enabling active-active default gateway redundancy for hosts.



_Appears in:_
- [NetworkVirtualizationEdgeSpec](#networkvirtualizationedgespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `virtualMAC` _string_ | VirtualMAC is the shared MAC address used by all NVEs in the fabric<br />for anycast gateway functionality on RoutedVLAN (SVI) interfaces.<br />All switches in the fabric must use the same MAC address.<br />Format: IEEE 802 MAC-48 address (e.g., "00:00:5E:00:01:01") |  | Pattern: `^([0-9A-Fa-f]\{2\}:)\{5\}[0-9A-Fa-f]\{2\}$` <br />Required: \{\} <br /> |


#### BFD



BFD defines the Bidirectional Forwarding Detection configuration for an interface.



_Appears in:_
- [InterfaceSpec](#interfacespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled indicates whether BFD is enabled on the interface. |  | Required: \{\} <br /> |
| `desiredMinimumTxInterval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#duration-v1-meta)_ | DesiredMinimumTxInterval is the minimum interval between transmission of BFD control<br />packets that the operator desires. This value is advertised to the peer.<br />The actual interval used is the maximum of this value and the remote<br />required-minimum-receive interval value. |  | Pattern: `^([0-9]+(\.[0-9]+)?(ns\|us\|µs\|ms\|s\|m\|h))+$` <br />Type: string <br />Optional: \{\} <br /> |
| `requiredMinimumReceive` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#duration-v1-meta)_ | RequiredMinimumReceive is the minimum interval between received BFD control packets<br />that this system should support. This value is advertised to the remote peer to<br />indicate the maximum frequency between BFD control packets that is acceptable<br />to the local system. |  | Pattern: `^([0-9]+(\.[0-9]+)?(ns\|us\|µs\|ms\|s\|m\|h))+$` <br />Type: string <br />Optional: \{\} <br /> |
| `detectionMultiplier` _integer_ | DetectionMultiplier is the number of packets that must be missed to declare<br />this session as down. The detection interval for the BFD session is calculated<br />by multiplying the value of the negotiated transmission interval by this value. |  | Maximum: 255 <br />Minimum: 1 <br />Optional: \{\} <br /> |


#### BGP



BGP is the Schema for the bgp API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `BGP` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[BGPSpec](#bgpspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[BGPStatus](#bgpstatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### BGPAddressFamilies



BGPAddressFamilies defines the configuration for supported BGP address families.



_Appears in:_
- [BGPSpec](#bgpspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ipv4Unicast` _[BGPAddressFamily](#bgpaddressfamily)_ | Ipv4Unicast configures IPv4 unicast address family support.<br />Enables exchange of IPv4 unicast routes between BGP peers. |  | Optional: \{\} <br /> |
| `ipv6Unicast` _[BGPAddressFamily](#bgpaddressfamily)_ | Ipv6Unicast configures IPv6 unicast address family support.<br />Enables exchange of IPv6 unicast routes between BGP peers. |  | Optional: \{\} <br /> |
| `l2vpnEvpn` _[BGPL2vpnEvpn](#bgpl2vpnevpn)_ | L2vpnEvpn configures L2VPN EVPN address family support.<br />Enables exchange of Ethernet VPN routes for overlay network services. |  | Optional: \{\} <br /> |


#### BGPAddressFamily



BGPAddressFamily defines common configuration for a BGP address family.



_Appears in:_
- [BGPAddressFamilies](#bgpaddressfamilies)
- [BGPL2vpnEvpn](#bgpl2vpnevpn)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled determines whether this address family is activated for BGP sessions.<br />When false, the address family is not negotiated with peers. |  | Optional: \{\} <br /> |
| `multipath` _[BGPMultipath](#bgpmultipath)_ | Multipath configures address family specific multipath behavior.<br />When specified, overrides global multipath settings for this address family. |  | Optional: \{\} <br /> |


#### BGPAddressFamilyType

_Underlying type:_ _string_

BGPAddressFamilyType represents the BGP address family identifier (AFI/SAFI combination).

_Validation:_
- Enum: [IPv4Unicast IPv6Unicast L2vpnEvpn]

_Appears in:_
- [AddressFamilyStatus](#addressfamilystatus)

| Field | Description |
| --- | --- |
| `IPv4Unicast` | BGPAddressFamilyIpv4Unicast represents the IPv4 Unicast address family (AFI=1, SAFI=1).<br /> |
| `IPv6Unicast` | BGPAddressFamilyIpv6Unicast represents the IPv6 Unicast address family (AFI=2, SAFI=1).<br /> |
| `L2vpnEvpn` | BGPAddressFamilyL2vpnEvpn represents the L2VPN EVPN address family (AFI=25, SAFI=70).<br /> |


#### BGPCommunityType

_Underlying type:_ _string_

BGPCommunityType represents the type of BGP community attributes that can be sent to peers.

_Validation:_
- Enum: [Standard Extended Both]

_Appears in:_
- [BGPPeerAddressFamily](#bgppeeraddressfamily)

| Field | Description |
| --- | --- |
| `Standard` | BGPCommunityTypeStandard sends only standard community attributes (RFC 1997)<br /> |
| `Extended` | BGPCommunityTypeExtended sends only extended community attributes (RFC 4360)<br /> |
| `Both` | BGPCommunityTypeBoth sends both standard and extended community attributes<br /> |


#### BGPL2vpnEvpn



BGPL2vpnEvpn defines the configuration for L2VPN EVPN address family.



_Appears in:_
- [BGPAddressFamilies](#bgpaddressfamilies)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled determines whether this address family is activated for BGP sessions.<br />When false, the address family is not negotiated with peers. |  | Optional: \{\} <br /> |
| `multipath` _[BGPMultipath](#bgpmultipath)_ | Multipath configures address family specific multipath behavior.<br />When specified, overrides global multipath settings for this address family. |  | Optional: \{\} <br /> |
| `routeTargetPolicy` _[BGPRouteTargetPolicy](#bgproutetargetpolicy)_ | RouteTargetPolicy configures route target filtering behavior for EVPN routes.<br />Controls which routes are retained based on route target matching. |  | Optional: \{\} <br /> |


#### BGPMultipath



BGPMultipath defines the configuration for BGP multipath behavior.



_Appears in:_
- [BGPAddressFamily](#bgpaddressfamily)
- [BGPL2vpnEvpn](#bgpl2vpnevpn)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled determines whether BGP is allowed to use multiple paths for forwarding.<br />When false, BGP will only use a single best path regardless of multiple equal-cost paths. |  | Optional: \{\} <br /> |
| `ebgp` _[BGPMultipathEbgp](#bgpmultipathebgp)_ | Ebgp configures multipath behavior for external BGP (eBGP) paths. |  | Optional: \{\} <br /> |
| `ibgp` _[BGPMultipathIbgp](#bgpmultipathibgp)_ | Ibgp configures multipath behavior for internal BGP (iBGP) paths. |  | Optional: \{\} <br /> |


#### BGPMultipathEbgp



BGPMultipathEbgp defines the configuration for eBGP multipath behavior.



_Appears in:_
- [BGPMultipath](#bgpmultipath)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `allowMultipleAs` _boolean_ | AllowMultipleAs enables the use of multiple paths with different AS paths for eBGP.<br />When true, relaxes the requirement that multipath candidates must have identical AS paths.<br />This corresponds to the "RelaxAs" mode. |  | Optional: \{\} <br /> |
| `maximumPaths` _integer_ | MaximumPaths sets the maximum number of eBGP paths that can be used for multipath load balancing.<br />Valid range is 1-64 when specified. When omitted, no explicit limit is configured. |  | Maximum: 64 <br />Minimum: 1 <br />Optional: \{\} <br /> |


#### BGPMultipathIbgp



BGPMultipathIbgp defines the configuration for iBGP multipath behavior.



_Appears in:_
- [BGPMultipath](#bgpmultipath)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maximumPaths` _integer_ | MaximumPaths sets the maximum number of iBGP paths that can be used for multipath load balancing.<br />Valid range is 1-64 when specified. When omitted, no explicit limit is configured. |  | Maximum: 64 <br />Minimum: 1 <br />Optional: \{\} <br /> |


#### BGPPeer



BGPPeer is the Schema for the bgppeers API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `BGPPeer` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[BGPPeerSpec](#bgppeerspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[BGPPeerStatus](#bgppeerstatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### BGPPeerAddressFamilies



BGPPeerAddressFamilies defines the address family specific configuration for a BGP peer.



_Appears in:_
- [BGPPeerSpec](#bgppeerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ipv4Unicast` _[BGPPeerAddressFamily](#bgppeeraddressfamily)_ | Ipv4Unicast configures IPv4 unicast address family settings for this peer.<br />Controls IPv4 unicast route exchange and peer-specific behavior. |  | Optional: \{\} <br /> |
| `ipv6Unicast` _[BGPPeerAddressFamily](#bgppeeraddressfamily)_ | Ipv6Unicast configures IPv6 unicast address family settings for this peer.<br />Controls IPv6 unicast route exchange and peer-specific behavior. |  | Optional: \{\} <br /> |
| `l2vpnEvpn` _[BGPPeerAddressFamily](#bgppeeraddressfamily)_ | L2vpnEvpn configures L2VPN EVPN address family settings for this peer.<br />Controls EVPN route exchange and peer-specific behavior. |  | Optional: \{\} <br /> |


#### BGPPeerAddressFamily



BGPPeerAddressFamily defines common configuration for a BGP peer's address family.



_Appears in:_
- [BGPPeerAddressFamilies](#bgppeeraddressfamilies)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled determines whether this address family is activated for this specific peer.<br />When false, the address family is not negotiated with this peer.<br />Defaults to false. |  | Optional: \{\} <br /> |
| `sendCommunity` _[BGPCommunityType](#bgpcommunitytype)_ | SendCommunity specifies which community attributes should be sent to this BGP peer<br />for this address family. If not specified, no community attributes are sent. |  | Enum: [Standard Extended Both] <br />Optional: \{\} <br /> |
| `routeReflectorClient` _boolean_ | RouteReflectorClient indicates whether this peer should be treated as a route reflector client<br />for this specific address family. Defaults to false. |  | Optional: \{\} <br /> |


#### BGPPeerLocalAddress



BGPPeerLocalAddress defines the local address configuration for a BGP peer.



_Appears in:_
- [BGPPeerSpec](#bgppeerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `interfaceRef` _[LocalObjectReference](#localobjectreference)_ | InterfaceRef is a reference to an Interface resource whose IP address will be used<br />as the source address for BGP packets sent to this peer.<br />The Interface object must exist in the same namespace. |  | Required: \{\} <br /> |


#### BGPPeerSessionState

_Underlying type:_ _string_

BGPPeerSessionState represents the operational state of a BGP peer session.

_Validation:_
- Enum: [Idle Connect Active OpenSent OpenConfirm Established Unknown]

_Appears in:_
- [BGPPeerStatus](#bgppeerstatus)

| Field | Description |
| --- | --- |
| `Idle` | BGPPeerSessionStateIdle indicates the peer is down and in the idle state of the FSM.<br /> |
| `Connect` | BGPPeerSessionStateConnect indicates the peer is down and the session is waiting for<br />the underlying transport session to be established.<br /> |
| `Active` | BGPPeerSessionStateActive indicates the peer is down and the local system is awaiting<br />a connection from the remote peer.<br /> |
| `OpenSent` | BGPPeerSessionStateOpenSent indicates the peer is in the process of being established.<br />The local system has sent an OPEN message.<br /> |
| `OpenConfirm` | BGPPeerSessionStateOpenConfirm indicates the peer is in the process of being established.<br />The local system is awaiting a NOTIFICATION or KEEPALIVE message.<br /> |
| `Established` | BGPPeerSessionStateEstablished indicates the peer is up - the BGP session with the peer is established.<br /> |
| `Unknown` | BGPPeerSessionStateUnknown indicates the peer state is unknown.<br /> |


#### BGPPeerSpec



BGPPeerSpec defines the desired state of BGPPeer



_Appears in:_
- [BGPPeer](#bgppeer)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the BGP to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `adminState` _[AdminState](#adminstate)_ | AdminState indicates whether this BGP peer is administratively up or down.<br />When Down, the BGP session with this peer is administratively shut down. | Up | Enum: [Up Down] <br />Optional: \{\} <br /> |
| `address` _string_ | Address is the IPv4 address of the BGP peer. |  | Format: ipv4 <br />Required: \{\} <br /> |
| `asNumber` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#intorstring-intstr-util)_ | ASNumber is the autonomous system number (ASN) of the BGP peer.<br />Supports both plain format (1-4294967295) and dotted notation (1-65535.0-65535) as per RFC 5396. |  | Required: \{\} <br /> |
| `description` _string_ | Description is an optional human-readable description for this BGP peer.<br />This field is used for documentation purposes and may be displayed in management interfaces. |  | Optional: \{\} <br /> |
| `localAddress` _[BGPPeerLocalAddress](#bgppeerlocaladdress)_ | LocalAddress specifies the local address configuration for the BGP session with this peer.<br />This determines the source address/interface for BGP packets sent to this peer. |  | Optional: \{\} <br /> |
| `addressFamilies` _[BGPPeerAddressFamilies](#bgppeeraddressfamilies)_ | AddressFamilies configures address family specific settings for this BGP peer.<br />Controls which address families are enabled and their specific configuration. |  | Optional: \{\} <br /> |


#### BGPPeerStatus



BGPPeerStatus defines the observed state of BGPPeer.



_Appears in:_
- [BGPPeer](#bgppeer)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `sessionState` _[BGPPeerSessionState](#bgppeersessionstate)_ | SessionState is the current operational state of the BGP session. |  | Enum: [Idle Connect Active OpenSent OpenConfirm Established Unknown] <br />Optional: \{\} <br /> |
| `lastEstablishedTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#time-v1-meta)_ | LastEstablishedTime is the timestamp when the BGP session last transitioned to the ESTABLISHED state.<br />A frequently changing timestamp indicates session instability (flapping). |  | Optional: \{\} <br /> |
| `advertisedPrefixesSummary` _string_ | AdvertisedPrefixesSummary provides a human-readable summary of advertised prefixes<br />across all address families (e.g., "10 (IPv4Unicast), 5 (IPv6Unicast)").<br />This field is computed by the controller from the AddressFamilies field. |  | Optional: \{\} <br /> |
| `addressFamilies` _[AddressFamilyStatus](#addressfamilystatus) array_ | AddressFamilies contains per-address-family statistics for this peer.<br />Only address families that are enabled and negotiated with the peer are included. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration reflects the .metadata.generation that was last processed by the controller. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the BGP. |  | Optional: \{\} <br /> |


#### BGPRouteTargetPolicy



BGPRouteTargetPolicy defines the policy for route target filtering in EVPN.



_Appears in:_
- [BGPL2vpnEvpn](#bgpl2vpnevpn)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `retainAll` _boolean_ | RetainAll controls whether all route targets are retained regardless of import policy. |  | Optional: \{\} <br /> |


#### BGPSpec



BGPSpec defines the desired state of BGP



_Appears in:_
- [BGP](#bgp)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the BGP to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `adminState` _[AdminState](#adminstate)_ | AdminState indicates whether this BGP router is administratively up or down. | Up | Enum: [Up Down] <br />Optional: \{\} <br /> |
| `asNumber` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#intorstring-intstr-util)_ | ASNumber is the autonomous system number (ASN) for the BGP router.<br />Supports both plain format (1-4294967295) and dotted notation (1-65535.0-65535) as per RFC 5396. |  | Required: \{\} <br /> |
| `routerId` _string_ | RouterID is the BGP router identifier, used in BGP messages to identify the originating router.<br />Follows dotted quad notation (IPv4 format). |  | Format: ipv4 <br />Required: \{\} <br /> |
| `addressFamilies` _[BGPAddressFamilies](#bgpaddressfamilies)_ | AddressFamilies configures supported BGP address families and their specific settings. |  | Optional: \{\} <br /> |


#### BGPStatus



BGPStatus defines the observed state of BGP.



_Appears in:_
- [BGP](#bgp)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the BGP. |  | Optional: \{\} <br /> |


#### Banner



Banner is the Schema for the banners API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `Banner` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[BannerSpec](#bannerspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[BannerStatus](#bannerstatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### BannerSpec



BannerSpec defines the desired state of Banner



_Appears in:_
- [Banner](#banner)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the Banner to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `type` _[BannerType](#bannertype)_ | Type specifies the banner type to configure, either PreLogin or PostLogin.<br />Immutable. | PreLogin | Enum: [PreLogin PostLogin] <br />Optional: \{\} <br /> |
| `message` _[TemplateSource](#templatesource)_ | Message is the banner message to display. |  | Required: \{\} <br /> |


#### BannerStatus



BannerStatus defines the observed state of Banner.



_Appears in:_
- [Banner](#banner)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the Banner. |  | Optional: \{\} <br /> |


#### BannerType

_Underlying type:_ _string_

BannerType represents the type of banner to configure

_Validation:_
- Enum: [PreLogin PostLogin]

_Appears in:_
- [BannerSpec](#bannerspec)

| Field | Description |
| --- | --- |
| `PreLogin` | BannerTypePreLogin represents the login banner displayed before user authentication.<br />This corresponds to the openconfig-system login-banner leaf.<br /> |
| `PostLogin` | BannerTypePostLogin represents the message banner displayed after user authentication.<br />This corresponds to the openconfig-system motd-banner leaf.<br /> |


#### BgpActions



BgpActions defines BGP-specific actions for a policy statement.



_Appears in:_
- [PolicyActions](#policyactions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `setCommunity` _[SetCommunityAction](#setcommunityaction)_ | SetCommunity configures BGP standard community attributes. |  | Optional: \{\} <br /> |
| `setExtCommunity` _[SetExtCommunityAction](#setextcommunityaction)_ | SetExtCommunity configures BGP extended community attributes. |  | Optional: \{\} <br /> |


#### Certificate



Certificate is the Schema for the certificates API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `Certificate` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[CertificateSpec](#certificatespec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[CertificateStatus](#certificatestatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### CertificateSource



CertificateSource represents a source for the value of a certificate.



_Appears in:_
- [TLS](#tls)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretRef` _[SecretReference](#secretreference)_ | Secret containing the certificate.<br />The secret must be of type kubernetes.io/tls and as such contain the following keys: 'tls.crt' and 'tls.key'. |  | Required: \{\} <br /> |


#### CertificateSpec



CertificateSpec defines the desired state of Certificate



_Appears in:_
- [Certificate](#certificate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the Certificate to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `id` _string_ | The certificate management id.<br />Immutable. |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `secretRef` _[SecretReference](#secretreference)_ | Secret containing the certificate source.<br />The secret must be of type kubernetes.io/tls and as such contain the following keys: 'tls.crt' and 'tls.key'. |  | Required: \{\} <br /> |


#### CertificateStatus



CertificateStatus defines the observed state of Certificate.



_Appears in:_
- [Certificate](#certificate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the Certificate. |  | Optional: \{\} <br /> |


#### ChecksumType

_Underlying type:_ _string_

ChecksumType defines the type of checksum used for image verification.

_Validation:_
- Enum: [SHA256 MD5]

_Appears in:_
- [Image](#image)

| Field | Description |
| --- | --- |
| `SHA256` |  |
| `MD5` |  |


#### ConfigMapKeySelector



ConfigMapKeySelector contains enough information to select a key of a ConfigMap.



_Appears in:_
- [TemplateSource](#templatesource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is unique within a namespace to reference a configmap resource. |  | MaxLength: 253 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `namespace` _string_ | Namespace defines the space within which the configmap name must be unique.<br />If omitted, the namespace of the object being reconciled will be used. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `key` _string_ | Key is the of the entry in the configmap resource's `data` or `binaryData`<br />field to be used. |  | MaxLength: 253 <br />MinLength: 1 <br />Required: \{\} <br /> |


#### ConfigMapReference



ConfigMapReference represents a ConfigMap Reference. It has enough information to retrieve a ConfigMap
in any namespace.



_Appears in:_
- [ConfigMapKeySelector](#configmapkeyselector)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is unique within a namespace to reference a configmap resource. |  | MaxLength: 253 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `namespace` _string_ | Namespace defines the space within which the configmap name must be unique.<br />If omitted, the namespace of the object being reconciled will be used. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |


#### ControlProtocol







_Appears in:_
- [Aggregation](#aggregation)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `mode` _[LACPMode](#lacpmode)_ | Mode defines the LACP mode for the aggregate interface. |  | Enum: [Active Passive] <br />Required: \{\} <br /> |


#### DNS



DNS is the Schema for the dns API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `DNS` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[DNSSpec](#dnsspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[DNSStatus](#dnsstatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### DNSSpec



DNSSpec defines the desired state of DNS



_Appears in:_
- [DNS](#dns)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the DNS to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `adminState` _[AdminState](#adminstate)_ | AdminState indicates whether DNS is administratively up or down. | Up | Enum: [Up Down] <br />Optional: \{\} <br /> |
| `domain` _string_ | Default domain name that the device uses to complete unqualified hostnames. |  | Format: hostname <br />MaxLength: 253 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `servers` _[NameServer](#nameserver) array_ | A list of DNS servers to use for address resolution. |  | MaxItems: 6 <br />MinItems: 1 <br />Optional: \{\} <br /> |
| `sourceInterfaceName` _string_ | Source interface for all DNS traffic. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |


#### DNSStatus



DNSStatus defines the observed state of DNS.



_Appears in:_
- [DNS](#dns)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the DNS. |  | Optional: \{\} <br /> |


#### Device



Device is the Schema for the devices API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `Device` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[DeviceSpec](#devicespec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  |  |
| `status` _[DeviceStatus](#devicestatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  |  |


#### DevicePhase

_Underlying type:_ _string_

DevicePhase represents the current phase of the Device as it's being provisioned and managed by the operator.

_Validation:_
- Enum: [Pending Provisioning Running Failed Provisioned]

_Appears in:_
- [DeviceStatus](#devicestatus)

| Field | Description |
| --- | --- |
| `Pending` | DevicePhasePending indicates that the device is pending and has not yet been provisioned.<br /> |
| `Provisioning` | DevicePhaseProvisioning indicates that the device is being provisioned.<br /> |
| `Provisioned` | DevicePhaseProvisioned indicates that the device provisioning has completed and the operator is performing post-provisioning tasks.<br /> |
| `Running` | DevicePhaseRunning indicates that the device has been successfully provisioned and is now ready for use.<br /> |
| `Failed` | DevicePhaseFailed indicates that the device provisioning has failed.<br /> |


#### DevicePort







_Appears in:_
- [DeviceStatus](#devicestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the port. |  | Required: \{\} <br /> |
| `type` _string_ | Type is the type of the port, e.g. "10g". |  | Optional: \{\} <br /> |
| `supportedSpeedsGbps` _integer array_ | SupportedSpeedsGbps is the list of supported speeds in Gbps for this port. |  | Optional: \{\} <br /> |
| `transceiver` _string_ | Transceiver is the type of transceiver plugged into the port, if any. |  | Optional: \{\} <br /> |
| `interfaceName` _[LocalObjectReference](#localobjectreference)_ | InterfaceRef is the reference to the corresponding Interface resource<br />configuring this port, if any. |  | Optional: \{\} <br /> |


#### DeviceSpec



DeviceSpec defines the desired state of Device.



_Appears in:_
- [Device](#device)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `endpoint` _[Endpoint](#endpoint)_ | Endpoint contains the connection information for the device. |  | Required: \{\} <br /> |
| `provisioning` _[Provisioning](#provisioning)_ | Provisioning is an optional configuration for the device provisioning process.<br />It can be used to provide initial configuration templates or scripts that are applied during the device provisioning. |  | Optional: \{\} <br /> |


#### DeviceStatus



DeviceStatus defines the observed state of Device.



_Appears in:_
- [Device](#device)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[DevicePhase](#devicephase)_ | Phase represents the current phase of the Device. | Pending | Enum: [Pending Provisioning Running Failed Provisioned] <br />Required: \{\} <br /> |
| `manufacturer` _string_ | Manufacturer is the manufacturer of the Device. |  | Optional: \{\} <br /> |
| `model` _string_ | Model is the model identifier of the Device. |  | Optional: \{\} <br /> |
| `serialNumber` _string_ | SerialNumber is the serial number of the Device. |  | Optional: \{\} <br /> |
| `firmwareVersion` _string_ | FirmwareVersion is the firmware version running on the Device. |  | Optional: \{\} <br /> |
| `provisioning` _[ProvisioningInfo](#provisioninginfo) array_ | Provisioning is the list of provisioning attempts for the Device. |  | Optional: \{\} <br /> |
| `ports` _[DevicePort](#deviceport) array_ | Ports is the list of ports on the Device. |  | Optional: \{\} <br /> |
| `portSummary` _string_ | PostSummary shows a summary of the port configured, grouped by type, e.g. "1/4 (10g), 3/64 (100g)". |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the Device. |  | Optional: \{\} <br /> |


#### EVPNInstance



EVPNInstance is the Schema for the evpninstances API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `EVPNInstance` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[EVPNInstanceSpec](#evpninstancespec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[EVPNInstanceStatus](#evpninstancestatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### EVPNInstanceSpec



EVPNInstanceSpec defines the desired state of EVPNInstance

It models an EVPN instance (EVI) context on a single network device based on VXLAN encapsulation and the VLAN-based service type defined in [RFC 8365].
[RFC 8365]: https://datatracker.ietf.org/doc/html/rfc8365



_Appears in:_
- [EVPNInstance](#evpninstance)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the BGP to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `vni` _integer_ | VNI is the VXLAN Network Identifier.<br />Immutable. |  | Maximum: 1.6777214e+07 <br />Minimum: 1 <br />Required: \{\} <br /> |
| `type` _[EVPNInstanceType](#evpninstancetype)_ | Type specifies the EVPN instance type.<br />Immutable. |  | Enum: [Bridged Routed] <br />Required: \{\} <br /> |
| `multicastGroupAddress` _string_ | MulticastGroupAddress specifies the IPv4 multicast group address used for BUM (Broadcast, Unknown unicast, Multicast) traffic.<br />The address must be in the valid multicast range (224.0.0.0 - 239.255.255.255). |  | Format: ipv4 <br />Optional: \{\} <br /> |
| `routeDistinguisher` _string_ | RouteDistinguisher is the route distinguisher for the EVI.<br />Formats supported:<br /> - Type 0: ASN(0-65535):Number(0-4294967295)<br /> - Type 1: IPv4:Number(0-65535)<br /> - Type 2: ASN(65536-4294967295):Number(0-65535) |  | Optional: \{\} <br /> |
| `routeTargets` _[EVPNRouteTarget](#evpnroutetarget) array_ | RouteTargets is the list of route targets for the EVI. |  | MinItems: 1 <br />Optional: \{\} <br /> |
| `vlanRef` _[LocalObjectReference](#localobjectreference)_ | VLANRef is a reference to a VLAN resource for which this EVPNInstance builds the MAC-VRF.<br />This field is only applicable when Type is Bridged (L2VNI).<br />The VLAN resource must exist in the same namespace.<br />Immutable. |  | Optional: \{\} <br /> |


#### EVPNInstanceStatus



EVPNInstanceStatus defines the observed state of EVPNInstance.



_Appears in:_
- [EVPNInstance](#evpninstance)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the EVPNInstance. |  | Optional: \{\} <br /> |


#### EVPNInstanceType

_Underlying type:_ _string_

EVPNInstanceType defines the type of EVPN instance.

_Validation:_
- Enum: [Bridged Routed]

_Appears in:_
- [EVPNInstanceSpec](#evpninstancespec)

| Field | Description |
| --- | --- |
| `Bridged` | EVPNInstanceTypeBridged represents an L2VNI (MAC-VRF) EVPN instance.<br />Corresponds to OpenConfig network-instance type L2VSI.<br /> |
| `Routed` | EVPNInstanceTypeRouted represents an L3VNI (IP-VRF) EVPN instance.<br />Corresponds to OpenConfig network-instance type L3VRF.<br /> |


#### EVPNRouteTarget







_Appears in:_
- [EVPNInstanceSpec](#evpninstancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `value` _string_ | Value is the route target value, must have the format as RouteDistinguisher. |  | MinLength: 1 <br />Required: \{\} <br /> |
| `action` _[RouteTargetAction](#routetargetaction)_ | Action defines whether the route target is imported, exported, or both. |  | Enum: [Import Export Both] <br />Required: \{\} <br /> |


#### Endpoint



Endpoint contains the connection information for the device.



_Appears in:_
- [DeviceSpec](#devicespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | Address is the management address of the device provided in IP:Port format. |  | Pattern: `^(\d\{1,3\}\.)\{3\}\d\{1,3\}:\d\{1,5\}$` <br />Required: \{\} <br /> |
| `secretRef` _[SecretReference](#secretreference)_ | SecretRef is name of the authentication secret for the device containing the username and password.<br />The secret must be of type kubernetes.io/basic-auth and as such contain the following keys: 'username' and 'password'. |  | Optional: \{\} <br /> |
| `tls` _[TLS](#tls)_ | Transport credentials for grpc connection to the switch. |  | Optional: \{\} <br /> |


#### GNMI







_Appears in:_
- [GRPC](#grpc)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxConcurrentCall` _integer_ | The maximum number of concurrent gNMI calls that can be made to the gRPC server on the switch for each VRF.<br />Configure a limit from 1 through 16. The default limit is 8. | 8 | ExclusiveMaximum: false <br />Maximum: 16 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `keepAliveTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#duration-v1-meta)_ | Configure the keepalive timeout for inactive or unauthorized connections.<br />The gRPC agent is expected to periodically send an empty response to the client, on which the client is expected to respond with an empty request.<br />If the client does not respond within the keepalive timeout, the gRPC agent should close the connection.<br />The default interval value is 10 minutes. | 10m | Pattern: `^([0-9]+(\.[0-9]+)?(ns\|us\|µs\|ms\|s\|m\|h))+$` <br />Type: string <br />Optional: \{\} <br /> |


#### GRPC







_Appears in:_
- [ManagementAccessSpec](#managementaccessspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enable or disable the gRPC server on the device.<br />If not specified, the gRPC server is enabled by default. | true | Optional: \{\} <br /> |
| `port` _integer_ | The TCP port on which the gRPC server should listen.<br />The range of port-id is from 1024 to 65535.<br />Port 9339 is the default. | 9339 | ExclusiveMaximum: false <br />Maximum: 65535 <br />Minimum: 1024 <br />Optional: \{\} <br /> |
| `certificateId` _string_ | Name of the certificate that is associated with the gRPC service.<br />The certificate is provisioned through other interfaces on the device,<br />such as e.g. the gNOI certificate management service. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `vrfName` _string_ | Enable the gRPC agent to accept incoming (dial-in) RPC requests from a given vrf. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `gnmi` _[GNMI](#gnmi)_ | Additional gNMI configuration for the gRPC server.<br />This may not be supported by all devices. | \{ keepAliveTimeout:10m maxConcurrentCall:8 \} | Optional: \{\} <br /> |


#### HostReachabilityType

_Underlying type:_ _string_

HostReachabilityType defines the method used for host reachability.

_Validation:_
- Enum: [FloodAndLearn BGP]

_Appears in:_
- [NetworkVirtualizationEdgeSpec](#networkvirtualizationedgespec)

| Field | Description |
| --- | --- |
| `BGP` | HostReachabilityTypeBGP uses BGP EVPN control-plane for MAC/IP advertisement.<br /> |
| `FloodAndLearn` | HostReachabilityTypeFloodAndLearn uses data-plane learning for MAC addresses.<br /> |


#### IPPrefix



IPPrefix represents an IP prefix in CIDR notation.
It is used to define a range of IP addresses in a network.

_Validation:_
- Format: cidr
- Type: string

_Appears in:_
- [ACLEntry](#aclentry)
- [InterfaceIPv4](#interfaceipv4)
- [PrefixEntry](#prefixentry)
- [RendezvousPoint](#rendezvouspoint)



#### ISIS



ISIS is the Schema for the isis API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `ISIS` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ISISSpec](#isisspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[ISISStatus](#isisstatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### ISISLevel

_Underlying type:_ _string_

ISISLevel represents the level of an ISIS instance.

_Validation:_
- Enum: [Level1 Level2 Level1-2]

_Appears in:_
- [ISISSpec](#isisspec)

| Field | Description |
| --- | --- |
| `Level1` |  |
| `Level2` |  |
| `Level1-2` |  |


#### ISISSpec



ISISSpec defines the desired state of ISIS



_Appears in:_
- [ISIS](#isis)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the Interface to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `adminState` _[AdminState](#adminstate)_ | AdminState indicates whether the ISIS instance is administratively up or down. | Up | Enum: [Up Down] <br />Optional: \{\} <br /> |
| `instance` _string_ | Instance is the name of the ISIS instance. |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `networkEntityTitle` _string_ | NetworkEntityTitle is the NET of the ISIS instance. |  | Pattern: `^[a-fA-F0-9]\{2\}(\.[a-fA-F0-9]\{4\})\{3,9\}\.[a-fA-F0-9]\{2\}$` <br />Required: \{\} <br /> |
| `type` _[ISISLevel](#isislevel)_ | Type indicates the level of the ISIS instance. |  | Enum: [Level1 Level2 Level1-2] <br />Required: \{\} <br /> |
| `overloadBit` _[OverloadBit](#overloadbit)_ | OverloadBit indicates the overload bit of the ISIS instance. | Never | Enum: [Always Never OnStartup] <br />Optional: \{\} <br /> |
| `addressFamilies` _[AddressFamily](#addressfamily) array_ | AddressFamilies is a list of address families for the ISIS instance. |  | Enum: [IPv4Unicast IPv6Unicast] <br />MaxItems: 2 <br />MinItems: 1 <br />Required: \{\} <br /> |
| `interfaceRefs` _[LocalObjectReference](#localobjectreference) array_ | InterfaceRefs is a list of interfaces that are part of the ISIS instance. |  | Optional: \{\} <br /> |


#### ISISStatus



ISISStatus defines the observed state of ISIS.



_Appears in:_
- [ISIS](#isis)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the ISIS. |  | Optional: \{\} <br /> |


#### Image







_Appears in:_
- [Provisioning](#provisioning)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `url` _string_ | URL is the location of the image to be used for provisioning. |  | Required: \{\} <br /> |
| `checksum` _string_ | Checksum is the checksum of the image for verification.<br />kubebuilder:validation:MinLength=1 |  | Required: \{\} <br /> |
| `checksumType` _[ChecksumType](#checksumtype)_ | ChecksumType is the type of the checksum (e.g., sha256, md5). | MD5 | Enum: [SHA256 MD5] <br />Required: \{\} <br /> |


#### Interface



Interface is the Schema for the interfaces API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `Interface` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[InterfaceSpec](#interfacespec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[InterfaceStatus](#interfacestatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### InterfaceIPv4



InterfaceIPv4 defines the IPv4 configuration for an interface.



_Appears in:_
- [InterfaceSpec](#interfacespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `addresses` _[IPPrefix](#ipprefix) array_ | Addresses defines the list of IPv4 addresses assigned to the interface.<br />The first address in the list is considered the primary address,<br />and any additional addresses are considered secondary addresses. |  | Format: cidr <br />MinItems: 1 <br />Type: string <br />Optional: \{\} <br /> |
| `unnumbered` _[InterfaceIPv4Unnumbered](#interfaceipv4unnumbered)_ | Unnumbered defines the unnumbered interface configuration.<br />When specified, the interface borrows the IP address from another interface. |  | Optional: \{\} <br /> |
| `anycastGateway` _boolean_ | AnycastGateway enables distributed anycast gateway functionality.<br />When enabled, this interface uses the virtual MAC configured in the<br />device's NVE resource for active-active default gateway redundancy.<br />Only applicable for RoutedVLAN interfaces in EVPN/VXLAN fabrics. | false | Optional: \{\} <br /> |


#### InterfaceIPv4Unnumbered



InterfaceIPv4Unnumbered defines the unnumbered interface configuration.
An unnumbered interface borrows the IP address from another interface,
allowing the interface to function without its own IP address assignment.



_Appears in:_
- [InterfaceIPv4](#interfaceipv4)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `interfaceRef` _[LocalObjectReference](#localobjectreference)_ | InterfaceRef is a reference to the interface from which to borrow the IP address.<br />The referenced interface must exist and have at least one IPv4 address configured. |  | Required: \{\} <br /> |


#### InterfaceSpec



InterfaceSpec defines the desired state of Interface.



_Appears in:_
- [Interface](#interface)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the Interface to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `name` _string_ | Name is the name of the interface. |  | MaxLength: 255 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `adminState` _[AdminState](#adminstate)_ | AdminState indicates whether the interface is administratively up or down. | Up | Enum: [Up Down] <br />Optional: \{\} <br /> |
| `description` _string_ | Description provides a human-readable description of the interface. |  | MaxLength: 255 <br />Optional: \{\} <br /> |
| `type` _[InterfaceType](#interfacetype)_ | Type indicates the type of the interface. |  | Enum: [Physical Loopback Aggregate RoutedVLAN] <br />Required: \{\} <br /> |
| `mtu` _integer_ | MTU (Maximum Transmission Unit) specifies the size of the largest packet that can be sent over the interface. |  | Maximum: 9216 <br />Minimum: 576 <br />Optional: \{\} <br /> |
| `switchport` _[Switchport](#switchport)_ | Switchport defines the switchport configuration for the interface.<br />This is only applicable for Ethernet and Aggregate interfaces. |  | Optional: \{\} <br /> |
| `ipv4` _[InterfaceIPv4](#interfaceipv4)_ | IPv4 defines the IPv4 configuration for the interface. |  | Optional: \{\} <br /> |
| `aggregation` _[Aggregation](#aggregation)_ | Aggregation defines the aggregation (bundle) configuration for the interface.<br />This is only applicable for interfaces of type Aggregate. |  | Optional: \{\} <br /> |
| `vlanRef` _[LocalObjectReference](#localobjectreference)_ | VlanRef is a reference to the VLAN resource that this interface provides routing for.<br />This is only applicable for interfaces of type RoutedVLAN.<br />The referenced VLAN must exist in the same namespace. |  | Optional: \{\} <br /> |
| `vrfRef` _[LocalObjectReference](#localobjectreference)_ | VrfRef is a reference to the VRF resource that this interface belongs to.<br />If not specified, the interface will be part of the default VRF.<br />This is only applicable for Layer 3 interfaces.<br />The referenced VRF must exist in the same namespace. |  | Optional: \{\} <br /> |
| `bfd` _[BFD](#bfd)_ | BFD defines the Bidirectional Forwarding Detection configuration for the interface.<br />BFD is only applicable for Layer 3 interfaces (Physical, Loopback, RoutedVLAN). |  | Optional: \{\} <br /> |


#### InterfaceStatus



InterfaceStatus defines the observed state of Interface.



_Appears in:_
- [Interface](#interface)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the Interface. |  | Optional: \{\} <br /> |
| `memberOf` _[LocalObjectReference](#localobjectreference)_ | MemberOf references the aggregate interface this interface is a member of, if any.<br />This field only applies to physical interfaces that are part of an aggregate interface. |  | Optional: \{\} <br /> |


#### InterfaceType

_Underlying type:_ _string_

InterfaceType represents the type of the interface.

_Validation:_
- Enum: [Physical Loopback Aggregate RoutedVLAN]

_Appears in:_
- [InterfaceSpec](#interfacespec)

| Field | Description |
| --- | --- |
| `Physical` | InterfaceTypePhysical indicates that the interface is a physical/ethernet interface.<br /> |
| `Loopback` | InterfaceTypeLoopback indicates that the interface is a loopback interface.<br /> |
| `Aggregate` | InterfaceTypeAggregate indicates that the interface is an aggregate (bundle) interface.<br /> |
| `RoutedVLAN` | InterfaceTypeRoutedVLAN indicates that the interface is a routed VLAN interface (SVI/IRB).<br /> |


#### LACPMode

_Underlying type:_ _string_

LACPMode represents the LACP mode of an interface.

_Validation:_
- Enum: [Active Passive]

_Appears in:_
- [ControlProtocol](#controlprotocol)

| Field | Description |
| --- | --- |
| `Active` | LACPModeActive indicates that LACP is in active mode.<br /> |
| `Passive` | LACPModePassive indicates that LACP is in passive mode.<br /> |


#### LocalObjectReference



LocalObjectReference contains enough information to locate a
referenced object inside the same namespace.



_Appears in:_
- [AccessControlListSpec](#accesscontrollistspec)
- [Aggregation](#aggregation)
- [BGPPeerLocalAddress](#bgppeerlocaladdress)
- [BGPPeerReference](#bgppeerreference)
- [BGPPeerSpec](#bgppeerspec)
- [BGPSpec](#bgpspec)
- [BannerSpec](#bannerspec)
- [BorderGatewaySpec](#bordergatewayspec)
- [CertificateSpec](#certificatespec)
- [DNSSpec](#dnsspec)
- [DevicePort](#deviceport)
- [EVPNInstanceSpec](#evpninstancespec)
- [ISISSpec](#isisspec)
- [InterconnectInterfaceReference](#interconnectinterfacereference)
- [InterfaceIPv4Unnumbered](#interfaceipv4unnumbered)
- [InterfaceSpec](#interfacespec)
- [InterfaceStatus](#interfacestatus)
- [KeepAlive](#keepalive)
- [ManagementAccessSpec](#managementaccessspec)
- [NTPSpec](#ntpspec)
- [NetworkVirtualizationEdgeSpec](#networkvirtualizationedgespec)
- [OSPFInterface](#ospfinterface)
- [OSPFNeighbor](#ospfneighbor)
- [OSPFSpec](#ospfspec)
- [PIMInterface](#piminterface)
- [PIMSpec](#pimspec)
- [Peer](#peer)
- [PrefixSetMatchCondition](#prefixsetmatchcondition)
- [PrefixSetSpec](#prefixsetspec)
- [RoutingPolicySpec](#routingpolicyspec)
- [SNMPSpec](#snmpspec)
- [SyslogSpec](#syslogspec)
- [SystemSpec](#systemspec)
- [UserSpec](#userspec)
- [VLANSpec](#vlanspec)
- [VLANStatus](#vlanstatus)
- [VPCDomainSpec](#vpcdomainspec)
- [VRFSpec](#vrfspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the referent.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |


#### LogFacility







_Appears in:_
- [SyslogSpec](#syslogspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | The name of the log facility. |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `severity` _[Severity](#severity)_ | The severity level of the log messages for this facility. |  | Enum: [Debug Info Notice Warning Error Critical Alert Emergency] <br />Required: \{\} <br /> |


#### LogServer







_Appears in:_
- [SyslogSpec](#syslogspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | IP address or hostname of the remote log server |  | MaxLength: 253 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `severity` _[Severity](#severity)_ | The servity level of the log messages sent to the server. |  | Enum: [Debug Info Notice Warning Error Critical Alert Emergency] <br />Required: \{\} <br /> |
| `vrfName` _string_ | The name of the vrf used to reach the log server. |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `port` _integer_ | The destination port number for syslog UDP messages to<br />the server. The default is 514. | 514 | Optional: \{\} <br /> |


#### ManagementAccess



ManagementAccess is the Schema for the managementaccesses API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `ManagementAccess` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ManagementAccessSpec](#managementaccessspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[ManagementAccessStatus](#managementaccessstatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### ManagementAccessSpec



ManagementAccessSpec defines the desired state of ManagementAccess



_Appears in:_
- [ManagementAccess](#managementaccess)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the Interface to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `grpc` _[GRPC](#grpc)_ | Configuration for the gRPC server on the device.<br />Currently, only a single "default" gRPC server is supported. | \{ enabled:true port:9339 \} | Optional: \{\} <br /> |
| `ssh` _[SSH](#ssh)_ | Configuration for the SSH server on the device. | \{ enabled:true sessionLimit:32 timeout:10m \} | Optional: \{\} <br /> |


#### ManagementAccessStatus



ManagementAccessStatus defines the observed state of ManagementAccess.



_Appears in:_
- [ManagementAccess](#managementaccess)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the ManagementAccess. |  | Optional: \{\} <br /> |


#### MaskLengthRange







_Appears in:_
- [PrefixEntry](#prefixentry)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `min` _integer_ | Minimum mask length. |  | Maximum: 128 <br />Minimum: 0 <br />Required: \{\} <br /> |
| `max` _integer_ | Maximum mask length. |  | Maximum: 128 <br />Minimum: 0 <br />Required: \{\} <br /> |


#### MultiChassis







_Appears in:_
- [Aggregation](#aggregation)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled indicates whether the aggregate interface is part of a multichassis setup. | true | Required: \{\} <br /> |
| `id` _integer_ | ID is the multichassis identifier. |  | Maximum: 4094 <br />Minimum: 1 <br />Required: \{\} <br /> |


#### MulticastGroups



MulticastGroups defines multicast group addresses for overlay BUM traffic.
Only supports IPv4 multicast addresses.



_Appears in:_
- [NetworkVirtualizationEdgeSpec](#networkvirtualizationedgespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `l2` _string_ | L2 is the multicast group for Layer 2 VNIs (BUM traffic in bridged VLANs). |  | Format: ipv4 <br />Optional: \{\} <br /> |
| `l3` _string_ | L3 is the multicast group for Layer 3 VNIs (BUM traffic in routed VRFs). |  | Format: ipv4 <br />Optional: \{\} <br /> |


#### NTP



NTP is the Schema for the ntp API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `NTP` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[NTPSpec](#ntpspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[NTPStatus](#ntpstatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### NTPServer







_Appears in:_
- [NTPSpec](#ntpspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | Hostname/IP address of the NTP server. |  | MaxLength: 253 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `prefer` _boolean_ | Indicates whether this server should be preferred or not. | false | Optional: \{\} <br /> |
| `vrfName` _string_ | The name of the vrf used to communicate with the NTP server. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |


#### NTPSpec



NTPSpec defines the desired state of NTP



_Appears in:_
- [NTP](#ntp)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the NTP to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `adminState` _[AdminState](#adminstate)_ | AdminState indicates whether NTP is administratively up or down. | Up | Enum: [Up Down] <br />Optional: \{\} <br /> |
| `sourceInterfaceName` _string_ | Source interface for all NTP traffic. |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `servers` _[NTPServer](#ntpserver) array_ | NTP servers. |  | MinItems: 1 <br />Required: \{\} <br /> |


#### NTPStatus



NTPStatus defines the observed state of NTP.



_Appears in:_
- [NTP](#ntp)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the NTP. |  | Optional: \{\} <br /> |


#### NameServer







_Appears in:_
- [DNSSpec](#dnsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | The Hostname or IP address of the DNS server. |  | MaxLength: 253 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `vrfName` _string_ | The name of the vrf used to communicate with the DNS server. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |


#### NetworkVirtualizationEdge



NetworkVirtualizationEdge is the Schema for the networkvirtualizationedges API
The NVE resource is the equivalent to an Endpoint for a Network Virtualization Overlay Object in OpenConfig (`nvo:Ep`).





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `NetworkVirtualizationEdge` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[NetworkVirtualizationEdgeSpec](#networkvirtualizationedgespec)_ |  |  | Required: \{\} <br /> |
| `status` _[NetworkVirtualizationEdgeStatus](#networkvirtualizationedgestatus)_ |  |  | Optional: \{\} <br /> |


#### NetworkVirtualizationEdgeSpec



NetworkVirtualizationEdgeSpec defines the desired state of a Network Virtualization Edge (NVE).



_Appears in:_
- [NetworkVirtualizationEdge](#networkvirtualizationedge)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration for this NVE.<br />If not specified the provider applies the target platform's default settings. |  | Optional: \{\} <br /> |
| `adminState` _[AdminState](#adminstate)_ | AdminState indicates whether the interface is administratively up or down. |  | Enum: [Up Down] <br />Required: \{\} <br /> |
| `sourceInterfaceRef` _[LocalObjectReference](#localobjectreference)_ | SourceInterface is the reference to the loopback interface used for the primary NVE IP address. |  | Required: \{\} <br /> |
| `anycastSourceInterfaceRef` _[LocalObjectReference](#localobjectreference)_ | AnycastSourceInterfaceRef is the reference to the loopback interface used for anycast NVE IP address. |  | Optional: \{\} <br /> |
| `suppressARP` _boolean_ | SuppressARP indicates whether ARP suppression is enabled for this NVE. | false | Optional: \{\} <br /> |
| `hostReachability` _[HostReachabilityType](#hostreachabilitytype)_ | HostReachability specifies the method used for host reachability. |  | Enum: [FloodAndLearn BGP] <br />Required: \{\} <br /> |
| `multicastGroups` _[MulticastGroups](#multicastgroups)_ | MulticastGroups defines multicast group addresses for BUM traffic. |  | Optional: \{\} <br /> |
| `anycastGateway` _[AnycastGateway](#anycastgateway)_ | AnycastGateway defines the distributed anycast gateway configuration.<br />This enables multiple NVEs to share the same gateway IP and MAC<br />for active-active first-hop redundancy. |  | Optional: \{\} <br /> |


#### NetworkVirtualizationEdgeStatus



NetworkVirtualizationEdgeStatus defines the observed state of the NVE.



_Appears in:_
- [NetworkVirtualizationEdge](#networkvirtualizationedge)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | conditions represent the current state of the NVE resource.<br />Each condition has a unique type and reflects the status of a specific aspect of the resource.<br />Standard condition types include:<br />- "Available": the resource is fully functional<br />- "Progressing": the resource is being created or updated<br />- "Degraded": the resource failed to reach or maintain its desired state<br />The conditions are a list of status objects that describe the state of the NVE. |  | Optional: \{\} <br /> |
| `sourceInterfaceName` _string_ | SourceInterfaceName is the resolved source interface IP address used for NVE encapsulation. |  |  |
| `anycastSourceInterfaceName` _string_ | AnycastSourceInterfaceName is the resolved anycast source interface IP address used for NVE encapsulation. |  |  |
| `hostReachability` _string_ | HostReachability indicates the actual method used for host reachability. |  |  |


#### OSPF



OSPF is the Schema for the ospf API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `OSPF` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OSPFSpec](#ospfspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[OSPFStatus](#ospfstatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### OSPFInterface



OSPFInterface defines the OSPF-specific configuration for an interface
that is participating in an OSPF instance.



_Appears in:_
- [OSPFSpec](#ospfspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the referent.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `area` _string_ | Area is the OSPF area identifier for this interface.<br />Must be specified in dotted-quad notation (e.g., "0.0.0.0" for the backbone area).<br />This is semantically a 32-bit identifier displayed in IPv4 address format,<br />not an actual IPv4 address. Area 0 (0.0.0.0) is the OSPF backbone area and<br />is required for proper OSPF operation in multi-area configurations. |  | Format: ipv4 <br />Required: \{\} <br /> |
| `passive` _boolean_ | Passive indicates whether this interface should operate in passive mode.<br />In passive mode, OSPF will advertise the interface's network in LSAs but will not<br />send or receive OSPF protocol packets (Hello, LSU, etc.) on this interface.<br />This is typically used for loopback interfaces where OSPF adjacencies<br />should not be formed but the network should still be advertised.<br />Defaults to false (active mode). |  | Optional: \{\} <br /> |


#### OSPFNeighbor



OSPFNeighbor represents an OSPF neighbor with its adjacency information.



_Appears in:_
- [OSPFStatus](#ospfstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `routerId` _string_ | RouterID is the router identifier of the remote OSPF neighbor. |  | Required: \{\} <br /> |
| `address` _string_ | Address is the IP address of the remote OSPF neighbor. |  | Required: \{\} <br /> |
| `interfaceRef` _[LocalObjectReference](#localobjectreference)_ | InterfaceRef is a reference to the local interface through which this neighbor is connected. |  | Required: \{\} <br /> |
| `priority` _integer_ | Priority is the remote system's priority to become the designated router.<br />Valid range is 0-255. |  | Optional: \{\} <br /> |
| `lastEstablishedTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#time-v1-meta)_ | LastEstablishedTime is the timestamp when the adjacency last transitioned to the FULL state.<br />A frequently changing timestamp indicates adjacency instability (flapping). |  | Optional: \{\} <br /> |
| `adjacencyState` _[OSPFNeighborState](#ospfneighborstate)_ | AdjacencyState is the current state of the adjacency with this neighbor. |  | Enum: [Down Attempt Init TwoWay ExStart Exchange Loading Full] <br />Optional: \{\} <br /> |


#### OSPFNeighborState

_Underlying type:_ _string_

OSPFNeighborState represents the state of an OSPF adjacency as defined in RFC 2328.

_Validation:_
- Enum: [Down Attempt Init TwoWay ExStart Exchange Loading Full]

_Appears in:_
- [OSPFNeighbor](#ospfneighbor)

| Field | Description |
| --- | --- |
| `Unknown` | OSPFNeighborStateUnknown indicates an unknown or undefined state.<br /> |
| `Down` | OSPFNeighborStateDown indicates the initial state of a neighbor.<br />No recent information has been received from the neighbor.<br /> |
| `Attempt` | OSPFNeighborStateAttempt is only valid for neighbors on NBMA networks.<br />It indicates that no recent information has been received but effort should be made to contact the neighbor.<br /> |
| `Init` | OSPFNeighborStateInit indicates a Hello packet has been received from the neighbor<br />but bidirectional communication has not yet been established.<br /> |
| `TwoWay` | OSPFNeighborStateTwoWay indicates bidirectional communication has been established.<br />This is the most advanced state short of forming an adjacency.<br /> |
| `ExStart` | OSPFNeighborStateExStart indicates the first step in creating an adjacency.<br />The routers are determining the relationship and initial DD sequence number.<br /> |
| `Exchange` | OSPFNeighborStateExchange indicates the routers are exchanging Database Description packets.<br /> |
| `Loading` | OSPFNeighborStateLoading indicates Link State Request packets are being sent to the neighbor<br />to obtain more recent LSAs that were discovered during the Exchange state.<br /> |
| `Full` | OSPFNeighborStateFull indicates the neighboring routers are fully adjacent.<br />LSDBs are synchronized and the adjacency will appear in Router and Network LSAs.<br /> |


#### OSPFSpec



OSPFSpec defines the desired state of OSPF



_Appears in:_
- [OSPF](#ospf)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the Interface to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `adminState` _[AdminState](#adminstate)_ | AdminState indicates whether the OSPF instance is administratively up or down. | Up | Enum: [Up Down] <br />Optional: \{\} <br /> |
| `instance` _string_ | Instance is the process tag of the OSPF instance. |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `routerId` _string_ | RouterID is the OSPF router identifier, used in OSPF messages to identify the originating router.<br />Follows dotted quad notation (IPv4 format). |  | Format: ipv4 <br />Required: \{\} <br /> |
| `logAdjacencyChanges` _boolean_ | LogAdjacencyChanges enables logging when the state of an OSPF neighbor changes.<br />When true, a log message is generated for adjacency state transitions. |  | Optional: \{\} <br /> |
| `interfaceRefs` _[OSPFInterface](#ospfinterface) array_ | InterfaceRefs is a list of interfaces that are part of the OSPF instance. |  | MinItems: 1 <br />Optional: \{\} <br /> |


#### OSPFStatus



OSPFStatus defines the observed state of OSPF.



_Appears in:_
- [OSPF](#ospf)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `adjacencySummary` _string_ | AdjacencySummary provides a human-readable summary of neighbor adjacencies<br />by state (e.g., "3 Full, 1 ExStart, 1 Down").<br />This field is computed by the controller from the Neighbors field. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration reflects the .metadata.generation that was last processed by the controller. |  | Optional: \{\} <br /> |
| `neighbors` _[OSPFNeighbor](#ospfneighbor) array_ | Neighbors is a list of OSPF neighbors and their adjacency states. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the OSPF. |  | Optional: \{\} <br /> |


#### OverloadBit

_Underlying type:_ _string_

OverloadBit represents the overload bit of an ISIS instance.

_Validation:_
- Enum: [Always Never OnStartup]

_Appears in:_
- [ISISSpec](#isisspec)

| Field | Description |
| --- | --- |
| `Always` |  |
| `Never` |  |
| `OnStartup` |  |


#### PIM



PIM is the Schema for the pim API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `PIM` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PIMSpec](#pimspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[PIMStatus](#pimstatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### PIMInterface







_Appears in:_
- [PIMSpec](#pimspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the referent.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `mode` _[PIMInterfaceMode](#piminterfacemode)_ | Mode is the PIM mode to use when delivering multicast traffic via this interface. | Sparse | Enum: [Sparse Dense] <br />Optional: \{\} <br /> |


#### PIMInterfaceMode

_Underlying type:_ _string_

PIMInterfaceMode represents the mode of a PIM interface.

_Validation:_
- Enum: [Sparse Dense]

_Appears in:_
- [PIMInterface](#piminterface)

| Field | Description |
| --- | --- |
| `Sparse` |  |
| `Dense` |  |


#### PIMSpec



PIMSpec defines the desired state of PIM



_Appears in:_
- [PIM](#pim)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the PIM to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `adminState` _[AdminState](#adminstate)_ | AdminState indicates whether the PIM instance is administratively up or down. | Up | Enum: [Up Down] <br />Optional: \{\} <br /> |
| `rendezvousPoints` _[RendezvousPoint](#rendezvouspoint) array_ | RendezvousPoints defines the list of rendezvous points for sparse mode multicast. |  | MinItems: 1 <br />Optional: \{\} <br /> |
| `interfaceRefs` _[PIMInterface](#piminterface) array_ | InterfaceRefs is a list of interfaces that are part of the PIM instance. |  | MinItems: 1 <br />Optional: \{\} <br /> |


#### PIMStatus



PIMStatus defines the observed state of PIM.



_Appears in:_
- [PIM](#pim)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the PIM. |  | Optional: \{\} <br /> |


#### PasswordSource



PasswordSource represents a source for the value of a password.



_Appears in:_
- [UserSpec](#userspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretKeyRef` _[SecretKeySelector](#secretkeyselector)_ | Selects a key of a secret. |  | Required: \{\} <br /> |


#### PolicyActions



PolicyActions defines the actions to take when a policy statement matches.



_Appears in:_
- [PolicyStatement](#policystatement)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `routeDisposition` _[RouteDisposition](#routedisposition)_ | RouteDisposition specifies whether to accept or reject the route. |  | Enum: [AcceptRoute RejectRoute] <br />Required: \{\} <br /> |
| `bgpActions` _[BgpActions](#bgpactions)_ | BgpActions specifies BGP-specific actions to apply when the route is accepted.<br />Only applicable when RouteDisposition is AcceptRoute. |  | Optional: \{\} <br /> |


#### PolicyConditions



PolicyConditions defines the match criteria for a policy statement.



_Appears in:_
- [PolicyStatement](#policystatement)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `matchPrefixSet` _[PrefixSetMatchCondition](#prefixsetmatchcondition)_ | MatchPrefixSet matches routes against a PrefixSet resource. |  | Optional: \{\} <br /> |


#### PolicyStatement







_Appears in:_
- [RoutingPolicySpec](#routingpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `sequence` _integer_ | The sequence number of the policy statement. |  | Minimum: 1 <br />Required: \{\} <br /> |
| `conditions` _[PolicyConditions](#policyconditions)_ | Conditions define the match criteria for this statement.<br />If no conditions are specified, the statement matches all routes. |  | Optional: \{\} <br /> |
| `actions` _[PolicyActions](#policyactions)_ | Actions define what to do when conditions match. |  | Required: \{\} <br /> |


#### PrefixEntry







_Appears in:_
- [PrefixSetSpec](#prefixsetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `sequence` _integer_ | The sequence number of the Prefix entry. |  | Minimum: 1 <br />Required: \{\} <br /> |
| `prefix` _[IPPrefix](#ipprefix)_ | IP prefix. Can be IPv4 or IPv6.<br />Use 0.0.0.0/0 (::/0) to represent 'any'. |  | Format: cidr <br />Type: string <br />Required: \{\} <br /> |
| `maskLengthRange` _[MaskLengthRange](#masklengthrange)_ | Optional mask length range for the prefix.<br />If not specified, only the exact prefix length is matched. |  | Optional: \{\} <br /> |


#### PrefixSet



PrefixSet is the Schema for the prefixsets API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `PrefixSet` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PrefixSetSpec](#prefixsetspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[PrefixSetStatus](#prefixsetstatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### PrefixSetMatchCondition



PrefixSetMatchCondition defines the condition for matching against a PrefixSet.



_Appears in:_
- [PolicyConditions](#policyconditions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `prefixSetRef` _[LocalObjectReference](#localobjectreference)_ | PrefixSetRef references a PrefixSet in the same namespace.<br />The PrefixSet must exist and belong to the same device. |  | Required: \{\} <br /> |


#### PrefixSetSpec



PrefixSetSpec defines the desired state of PrefixSet



_Appears in:_
- [PrefixSet](#prefixset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the Banner to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `name` _string_ | Name is the name of the PrefixSet.<br />Immutable. |  | MaxLength: 32 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `entries` _[PrefixEntry](#prefixentry) array_ | A list of entries to apply.<br />The address families (IPv4, IPv6) of all prefixes in the list must match. |  | MaxItems: 100 <br />MinItems: 1 <br />Required: \{\} <br /> |


#### PrefixSetStatus



PrefixSetStatus defines the observed state of PrefixSet.



_Appears in:_
- [PrefixSet](#prefixset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `entriesSummary` _string_ | EntriesSummary provides a human-readable summary of the number of prefix entries. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the PrefixSet. |  | Optional: \{\} <br /> |


#### Protocol

_Underlying type:_ _string_

Protocol represents the protocol type for an ACL entry.

_Validation:_
- Enum: [ICMP IP OSPF PIM TCP UDP]

_Appears in:_
- [ACLEntry](#aclentry)

| Field | Description |
| --- | --- |
| `ICMP` |  |
| `IP` |  |
| `OSPF` |  |
| `PIM` |  |
| `TCP` |  |
| `UDP` |  |


#### Provisioning



Provisioning defines the configuration for device bootstrap.



_Appears in:_
- [DeviceSpec](#devicespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _[Image](#image)_ | Image defines the image to be used for provisioning the device. |  | Required: \{\} <br /> |
| `bootScript` _[TemplateSource](#templatesource)_ | BootScript defines the script delivered by a TFTP server to the device during bootstrapping. |  | Optional: \{\} <br /> |


#### ProvisioningInfo







_Appears in:_
- [DeviceStatus](#devicestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `startTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#time-v1-meta)_ |  |  |  |
| `token` _string_ |  |  |  |
| `phase` _[ProvisioningPhase](#provisioningphase)_ |  |  |  |
| `endTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#time-v1-meta)_ |  |  | Optional: \{\} <br /> |
| `reboot` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#time-v1-meta)_ |  |  | Optional: \{\} <br /> |
| `error` _string_ |  |  | Optional: \{\} <br /> |


#### ProvisioningPhase

_Underlying type:_ _string_

ProvisioningPhase represents the reason for the current provisioning status.



_Appears in:_
- [ProvisioningInfo](#provisioninginfo)

| Field | Description |
| --- | --- |
| `DataRetrieved` |  |
| `ScriptExecutionStarted` |  |
| `ScriptExecutionFailed` |  |
| `InstallingCertificates` |  |
| `DownloadingImage` |  |
| `ImageDownloadFailed` |  |
| `UpgradeStarting` |  |
| `UpgradeFailed` |  |
| `RebootingDevice` |  |
| `ExecutionFinishedWithoutReboot` |  |


#### RendezvousPoint







_Appears in:_
- [PIMSpec](#pimspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | Address is the IPv4 address of the rendezvous point. |  | Format: ipv4 <br />Required: \{\} <br /> |
| `multicastGroups` _[IPPrefix](#ipprefix) array_ | MulticastGroups defined the list of multicast IPv4 address ranges associated with the rendezvous point.<br />If not specified, the rendezvous point will be used for all multicast groups. |  | Format: cidr <br />Type: string <br />Optional: \{\} <br /> |
| `anycastAddresses` _string array_ | AnycastAddresses is a list of redundant anycast ipv4 addresses associated with the rendezvous point. |  | items:Format: ipv4 <br />Optional: \{\} <br /> |


#### RouteDisposition

_Underlying type:_ _string_

RouteDisposition defines the final disposition of a route.

_Validation:_
- Enum: [AcceptRoute RejectRoute]

_Appears in:_
- [PolicyActions](#policyactions)

| Field | Description |
| --- | --- |
| `AcceptRoute` | AcceptRoute permits the route and applies any configured actions.<br /> |
| `RejectRoute` | RejectRoute denies the route immediately.<br /> |


#### RouteTarget







_Appears in:_
- [VRFSpec](#vrfspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `value` _string_ | Value is the route target value, must have the format as VRFSpec.RouteDistinguisher. Validation via<br />admission webhook. |  | Required: \{\} <br /> |
| `addressFamilies` _[RouteTargetAF](#routetargetaf) array_ | AddressFamilies is the list of address families for the route target. |  | Enum: [IPv4 IPv6 IPv4EVPN IPv6EVPN] <br />MinItems: 1 <br />Required: \{\} <br /> |
| `action` _[RouteTargetAction](#routetargetaction)_ | Action defines whether the route target is imported, exported, or both |  | Enum: [Import Export Both] <br />Required: \{\} <br /> |


#### RouteTargetAF

_Underlying type:_ _string_

RouteTargetAF represents a supported address family value.

_Validation:_
- Enum: [IPv4 IPv6 IPv4EVPN IPv6EVPN]

_Appears in:_
- [RouteTarget](#routetarget)

| Field | Description |
| --- | --- |
| `IPv4` |  |
| `IPv6` |  |
| `IPv4EVPN` |  |
| `IPv6EVPN` |  |


#### RouteTargetAction

_Underlying type:_ _string_

RouteTargetAction represents the action for a route target.

_Validation:_
- Enum: [Import Export Both]

_Appears in:_
- [EVPNRouteTarget](#evpnroutetarget)
- [RouteTarget](#routetarget)

| Field | Description |
| --- | --- |
| `Import` |  |
| `Export` |  |
| `Both` |  |


#### RoutingPolicy



RoutingPolicy is the Schema for the routingpolicies API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `RoutingPolicy` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RoutingPolicySpec](#routingpolicyspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[RoutingPolicyStatus](#routingpolicystatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### RoutingPolicySpec



RoutingPolicySpec defines the desired state of RoutingPolicy



_Appears in:_
- [RoutingPolicy](#routingpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the Banner to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `name` _string_ | Name is the identifier of the RoutingPolicy on the device.<br />Immutable. |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `statements` _[PolicyStatement](#policystatement) array_ | A list of policy statements to apply. |  | MaxItems: 100 <br />MinItems: 1 <br />Required: \{\} <br /> |


#### RoutingPolicyStatus



RoutingPolicyStatus defines the observed state of RoutingPolicy.



_Appears in:_
- [RoutingPolicy](#routingpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `statementsSummary` _string_ | StatementsSummary provides a human-readable summary of the number of policy statements. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the RoutingPolicy. |  | Optional: \{\} <br /> |


#### SNMP



SNMP is the Schema for the snmp API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `SNMP` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[SNMPSpec](#snmpspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[SNMPStatus](#snmpstatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### SNMPCommunity







_Appears in:_
- [SNMPSpec](#snmpspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the community. |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `group` _string_ | Group to which the community belongs. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `aclName` _string_ | ACL name to filter SNMP requests. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |


#### SNMPHosts







_Appears in:_
- [SNMPSpec](#snmpspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | The Hostname or IP address of the SNMP host to send notifications to. |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `type` _string_ | Type of message to send to host. Default is traps. | Traps | Enum: [Traps Informs] <br />Optional: \{\} <br /> |
| `version` _string_ | SNMP version. Default is v2c. | v2c | Enum: [v1 v2c v3] <br />Optional: \{\} <br /> |
| `community` _string_ | SNMP community or user name. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `vrfName` _string_ | The name of the vrf instance to use to source traffic. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |


#### SNMPSpec



SNMPSpec defines the desired state of SNMP



_Appears in:_
- [SNMP](#snmp)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the SNMP to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `contact` _string_ | The contact information for the SNMP server. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `location` _string_ | The location information for the SNMP server. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `sourceInterfaceName` _string_ | The name of the interface to be used for sending out SNMP Trap/Inform notifications. |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `communities` _[SNMPCommunity](#snmpcommunity) array_ | SNMP communities for SNMPv1 or SNMPv2c. |  | MaxItems: 16 <br />MinItems: 1 <br />Optional: \{\} <br /> |
| `hosts` _[SNMPHosts](#snmphosts) array_ | SNMP destination hosts for SNMP traps or informs messages. |  | MaxItems: 16 <br />MinItems: 1 <br />Required: \{\} <br /> |
| `traps` _string array_ | The list of trap notifications to enable. |  | MinItems: 1 <br />Optional: \{\} <br /> |


#### SNMPStatus



SNMPStatus defines the observed state of SNMP.



_Appears in:_
- [SNMP](#snmp)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the SNMP. |  | Optional: \{\} <br /> |


#### SSH







_Appears in:_
- [ManagementAccessSpec](#managementaccessspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enable or disable the SSH server on the device.<br />If not specified, the SSH server is enabled by default. | true | Optional: \{\} <br /> |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#duration-v1-meta)_ | The timeout duration for SSH sessions.<br />If not specified, the default timeout is 10 minutes. | 10m | Type: string <br />Optional: \{\} <br /> |
| `sessionLimit` _integer_ | The maximum number of concurrent SSH sessions allowed.<br />If not specified, the default limit is 32. | 32 | ExclusiveMaximum: false <br />Maximum: 64 <br />Minimum: 1 <br />Optional: \{\} <br /> |


#### SSHPublicKeySource



SSHPublicKeySource represents a source for the value of an SSH public key.



_Appears in:_
- [UserSpec](#userspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretKeyRef` _[SecretKeySelector](#secretkeyselector)_ | Selects a key of a secret. |  | Required: \{\} <br /> |


#### SecretKeySelector



SecretKeySelector contains enough information to select a key of a Secret.



_Appears in:_
- [PasswordSource](#passwordsource)
- [SSHPublicKeySource](#sshpublickeysource)
- [TLS](#tls)
- [TemplateSource](#templatesource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is unique within a namespace to reference a secret resource. |  | MaxLength: 253 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `namespace` _string_ | Namespace defines the space within which the secret name must be unique.<br />If omitted, the namespace of the object being reconciled will be used. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `key` _string_ | Key is the of the entry in the secret resource's `data` or `stringData`<br />field to be used. |  | MaxLength: 253 <br />MinLength: 1 <br />Required: \{\} <br /> |


#### SecretReference



SecretReference represents a Secret Reference. It has enough information to retrieve a Secret
in any namespace.



_Appears in:_
- [CertificateSource](#certificatesource)
- [CertificateSpec](#certificatespec)
- [Endpoint](#endpoint)
- [SecretKeySelector](#secretkeyselector)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is unique within a namespace to reference a secret resource. |  | MaxLength: 253 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `namespace` _string_ | Namespace defines the space within which the secret name must be unique.<br />If omitted, the namespace of the object being reconciled will be used. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |


#### SetCommunityAction



SetCommunityAction defines the action to set BGP standard communities.



_Appears in:_
- [BgpActions](#bgpactions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `communities` _string array_ | Communities is the list of BGP standard communities to set.<br />The communities must be in the format defined by [RFC 1997].<br />[RFC 1997]: https://datatracker.ietf.org/doc/html/rfc1997 |  | MinItems: 1 <br />Required: \{\} <br /> |


#### SetExtCommunityAction



SetExtCommunityAction defines the action to set BGP extended communities.



_Appears in:_
- [BgpActions](#bgpactions)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `communities` _string array_ | Communities is the list of BGP extended communities to set.<br />The communities must be in the format defined by [RFC 4360].<br />[RFC 4360]: https://datatracker.ietf.org/doc/html/rfc4360 |  | MinItems: 1 <br />Required: \{\} <br /> |


#### Severity

_Underlying type:_ _string_

Severity represents the severity level of a log message.

_Validation:_
- Enum: [Debug Info Notice Warning Error Critical Alert Emergency]

_Appears in:_
- [LogFacility](#logfacility)
- [LogServer](#logserver)

| Field | Description |
| --- | --- |
| `Debug` |  |
| `Info` |  |
| `Notice` |  |
| `Warning` |  |
| `Error` |  |
| `Critical` |  |
| `Alert` |  |
| `Emergency` |  |


#### Switchport



Switchport defines the switchport configuration for an interface.



_Appears in:_
- [InterfaceSpec](#interfacespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `mode` _[SwitchportMode](#switchportmode)_ | Mode defines the switchport mode, such as access or trunk. |  | Enum: [Access Trunk] <br />Required: \{\} <br /> |
| `accessVlan` _integer_ | AccessVlan specifies the VLAN ID for access mode switchports.<br />Only applicable when Mode is set to "Access". |  | Maximum: 4094 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `nativeVlan` _integer_ | NativeVlan specifies the native VLAN ID for trunk mode switchports.<br />Only applicable when Mode is set to "Trunk". |  | Maximum: 4094 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `allowedVlans` _integer array_ | AllowedVlans is a list of VLAN IDs that are allowed on the trunk port.<br />If not specified, all VLANs (1-4094) are allowed.<br />Only applicable when Mode is set to "Trunk". |  | MinItems: 1 <br />items:Maximum: 4094 <br />items:Minimum: 1 <br />Optional: \{\} <br /> |


#### SwitchportMode

_Underlying type:_ _string_

SwitchportMode represents the switchport mode of an interface.

_Validation:_
- Enum: [Access Trunk]

_Appears in:_
- [Switchport](#switchport)

| Field | Description |
| --- | --- |
| `Access` | SwitchportModeAccess indicates that the switchport is in access mode.<br /> |
| `Trunk` | SwitchportModeTrunk indicates that the switchport is in trunk mode.<br /> |


#### Syslog



Syslog is the Schema for the syslogs API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `Syslog` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[SyslogSpec](#syslogspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[SyslogStatus](#syslogstatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### SyslogSpec



SyslogSpec defines the desired state of Syslog



_Appears in:_
- [Syslog](#syslog)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the Interface to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `servers` _[LogServer](#logserver) array_ | Servers is a list of remote log servers to which the device will send logs. |  | MaxItems: 16 <br />MinItems: 1 <br />Required: \{\} <br /> |
| `facilities` _[LogFacility](#logfacility) array_ | Facilities is a list of log facilities to configure on the device. |  | MaxItems: 64 <br />MinItems: 1 <br />Required: \{\} <br /> |


#### SyslogStatus



SyslogStatus defines the observed state of Syslog.



_Appears in:_
- [Syslog](#syslog)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serversSummary` _string_ | ServersSummary provides a human-readable summary of the number of log servers. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the Banner. |  | Optional: \{\} <br /> |


#### TLS







_Appears in:_
- [Endpoint](#endpoint)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `ca` _[SecretKeySelector](#secretkeyselector)_ | The CA certificate to verify the server's identity. |  | Required: \{\} <br /> |
| `certificate` _[CertificateSource](#certificatesource)_ | The client certificate and private key to use for mutual TLS authentication.<br />Leave empty if mTLS is not desired. |  | Optional: \{\} <br /> |


#### TemplateSource



TemplateSource defines a source for template content.
It can be provided inline, or as a reference to a Secret or ConfigMap.



_Appears in:_
- [BannerSpec](#bannerspec)
- [Provisioning](#provisioning)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `inline` _string_ | Inline template content |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `secretRef` _[SecretKeySelector](#secretkeyselector)_ | Reference to a Secret containing the template |  | Optional: \{\} <br /> |
| `configMapRef` _[ConfigMapKeySelector](#configmapkeyselector)_ | Reference to a ConfigMap containing the template |  | Optional: \{\} <br /> |


#### TypedLocalObjectReference



TypedLocalObjectReference contains enough information to locate a
typed referenced object inside the same namespace.



_Appears in:_
- [AccessControlListSpec](#accesscontrollistspec)
- [BGPPeerSpec](#bgppeerspec)
- [BGPSpec](#bgpspec)
- [BannerSpec](#bannerspec)
- [CertificateSpec](#certificatespec)
- [DNSSpec](#dnsspec)
- [EVPNInstanceSpec](#evpninstancespec)
- [ISISSpec](#isisspec)
- [InterfaceSpec](#interfacespec)
- [ManagementAccessSpec](#managementaccessspec)
- [NTPSpec](#ntpspec)
- [NetworkVirtualizationEdgeSpec](#networkvirtualizationedgespec)
- [OSPFSpec](#ospfspec)
- [PIMSpec](#pimspec)
- [PrefixSetSpec](#prefixsetspec)
- [RoutingPolicySpec](#routingpolicyspec)
- [SNMPSpec](#snmpspec)
- [SyslogSpec](#syslogspec)
- [UserSpec](#userspec)
- [VLANSpec](#vlanspec)
- [VRFSpec](#vrfspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kind` _string_ | Kind of the resource being referenced.<br />Kind must consist of alphanumeric characters or '-', start with an alphabetic character, and end with an alphanumeric character. |  | MaxLength: 63 <br />MinLength: 1 <br />Pattern: `^[a-zA-Z]([-a-zA-Z0-9]*[a-zA-Z0-9])?$` <br />Required: \{\} <br /> |
| `name` _string_ | Name of the resource being referenced.<br />Name must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character. |  | MaxLength: 253 <br />MinLength: 1 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$` <br />Required: \{\} <br /> |
| `apiVersion` _string_ | APIVersion is the api group version of the resource being referenced. |  | MaxLength: 253 <br />MinLength: 1 <br />Pattern: `^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/)?([a-z0-9]([-a-z0-9]*[a-z0-9])?)$` <br />Required: \{\} <br /> |


#### User



User is the Schema for the users API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `User` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[UserSpec](#userspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[UserStatus](#userstatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### UserRole



UserRole represents a role that can be assigned to a user.



_Appears in:_
- [UserSpec](#userspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | The name of the role. |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |


#### UserSpec



UserSpec defines the desired state of User



_Appears in:_
- [User](#user)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the User to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `username` _string_ | Assigned username for this user.<br />Immutable. |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `password` _[PasswordSource](#passwordsource)_ | The user password, supplied in cleartext. |  | Required: \{\} <br /> |
| `roles` _[UserRole](#userrole) array_ | Role which the user is to be assigned to. |  | MaxItems: 64 <br />MinItems: 1 <br />Required: \{\} <br /> |
| `sshPublicKey` _[SSHPublicKeySource](#sshpublickeysource)_ | SSH public key for this user. |  | Optional: \{\} <br /> |


#### UserStatus



UserStatus defines the observed state of User.



_Appears in:_
- [User](#user)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the User. |  | Optional: \{\} <br /> |


#### VLAN



VLAN is the Schema for the vlans API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `VLAN` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[VLANSpec](#vlanspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[VLANStatus](#vlanstatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### VLANSpec



VLANSpec defines the desired state of VLAN



_Appears in:_
- [VLAN](#vlan)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this vlan.<br />This reference is used to link the VLAN to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `id` _integer_ | ID is the VLAN ID. Valid values are between 1 and 4094.<br />Immutable. |  | Maximum: 4094 <br />Minimum: 1 <br />Required: \{\} <br /> |
| `name` _string_ | Name is the name of the VLAN. |  | MaxLength: 128 <br />MinLength: 1 <br />Pattern: `^[^\s]+$` <br />Optional: \{\} <br /> |
| `adminState` _[AdminState](#adminstate)_ | AdminState indicates whether the VLAN is administratively active or inactive/suspended. | Up | Enum: [Up Down] <br />Optional: \{\} <br /> |


#### VLANStatus



VLANStatus defines the observed state of VLAN.



_Appears in:_
- [VLAN](#vlan)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the VLAN. |  | Optional: \{\} <br /> |
| `routedBy` _[LocalObjectReference](#localobjectreference)_ | RoutedBy references the interface that provides Layer 3 routing for this VLAN, if any.<br />This field is set when an Interface of type RoutedVLAN references this VLAN. |  | Optional: \{\} <br /> |
| `bridgedBy` _[LocalObjectReference](#localobjectreference)_ | BridgedBy references the EVPNInstance that provides a L2VNI for this VLAN, if any.<br />This field is set when an EVPNInstance of type Bridged references this VLAN. |  | Optional: \{\} <br /> |


#### VRF



VRF is the Schema for the vrfs API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `VRF` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[VRFSpec](#vrfspec)_ | spec defines the desired state of VRF<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[VRFStatus](#vrfstatus)_ | status of the resource. This is set and updated automatically.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### VRFSpec



VRFSpec defines the desired state of VRF



_Appears in:_
- [VRF](#vrf)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `providerConfigRef` _[TypedLocalObjectReference](#typedlocalobjectreference)_ | ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.<br />This reference is used to link the VRF to its provider-specific configuration. |  | Optional: \{\} <br /> |
| `name` _string_ | Name is the name of the VRF.<br />Immutable. |  | MaxLength: 32 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `description` _string_ | Description provides a human-readable description of the VRF. |  | MaxLength: 255 <br />MinLength: 1 <br />Optional: \{\} <br /> |
| `vni` _integer_ | VNI is the VXLAN Network Identifier for the VRF (always an L3). |  | Maximum: 1.6777215e+07 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `routeDistinguisher` _string_ | RouteDistinguisher is the route distinguisher for the VRF.<br />Formats supported:<br /> - Type 0: ASN(0-65535):Number(0-4294967295)<br /> - Type 1: IPv4:Number(0-65535)<br /> - Type 2: ASN(65536-4294967295):Number(0-65535)<br />Validation via admission webhook for the VRF type. |  | Optional: \{\} <br /> |
| `routeTargets` _[RouteTarget](#routetarget) array_ | RouteTargets is the list of route targets for the VRF. |  | Optional: \{\} <br /> |


#### VRFStatus



VRFStatus defines the observed state of VRF.



_Appears in:_
- [VRF](#vrf)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the VRF. |  | Optional: \{\} <br /> |



## nx.cisco.networking.metal.ironcore.dev/v1alpha1

Package v1alpha1 contains API Schema definitions for the nx.cisco.networking.metal.ironcore.dev v1alpha1 API group.

### Resource Types
- [BorderGateway](#bordergateway)
- [InterfaceConfig](#interfaceconfig)
- [ManagementAccessConfig](#managementaccessconfig)
- [NetworkVirtualizationEdgeConfig](#networkvirtualizationedgeconfig)
- [System](#system)
- [VPCDomain](#vpcdomain)



#### AutoRecovery



AutoRecovery holds settings to automatically restore vPC domain's operation after detecting
that the peer is no longer reachable via the keepalive link.



_Appears in:_
- [Peer](#peer)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled indicates whether auto-recovery is enabled.<br />When enabled, the switch will wait for ReloadDelay seconds after peer failure<br />before assuming the peer is dead and restoring the vPC's domain functionality. |  | Required: \{\} <br /> |
| `reloadDelay` _integer_ | ReloadDelay is the time in seconds (60-3600) to wait before assuming the peer is dead<br />and automatically attempting to restore the communication with the peer. | 240 | Maximum: 3600 <br />Minimum: 60 <br />Optional: \{\} <br /> |


#### BGPPeerReference



BGPPeerReference defines a BGP peer used for border gateway with peer type configuration.



_Appears in:_
- [BorderGatewaySpec](#bordergatewayspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the referent.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `peerType` _[BGPPeerType](#bgppeertype)_ | PeerType specifies the role of this BGP peer in the EVPN multisite topology.<br />FabricExternal is used for peers outside the fabric, while FabricBorderLeaf is used<br />for border leaf peers within the fabric. |  | Enum: [FabricExternal FabricBorderLeaf] <br />Required: \{\} <br /> |


#### BGPPeerType

_Underlying type:_ _string_

BGPPeerType defines the peer type for border gateway BGP peers.

_Validation:_
- Enum: [FabricExternal FabricBorderLeaf]

_Appears in:_
- [BGPPeerReference](#bgppeerreference)

| Field | Description |
| --- | --- |
| `FabricExternal` | BGPPeerTypeFabricExternal represents a BGP peer outside the fabric.<br />Used for external peers in EVPN multisite configurations.<br /> |
| `FabricBorderLeaf` | BGPPeerTypeFabricBorderLeaf represents a BGP peer that is a border leaf within the fabric.<br />Used for border leaf peers in EVPN multisite configurations.<br /> |


#### BorderGateway



BorderGateway is the Schema for the bordergateways API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nx.cisco.networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `BorderGateway` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[BorderGatewaySpec](#bordergatewayspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[BorderGatewayStatus](#bordergatewaystatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### BorderGatewaySpec



BorderGatewaySpec defines the desired state of BorderGateway



_Appears in:_
- [BorderGateway](#bordergateway)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `adminState` _[AdminState](#adminstate)_ | AdminState indicates whether the BorderGateway instance is administratively up or down. | Up | Enum: [Up Down] <br />Optional: \{\} <br /> |
| `multisiteId` _integer_ | MultisiteID is the identifier for the multisite border gateway. |  | ExclusiveMaximum: false <br />Maximum: 2.81474976710655e+14 <br />Minimum: 1 <br />Required: \{\} <br /> |
| `sourceInterfaceRef` _[LocalObjectReference](#localobjectreference)_ | SourceInterfaceRef is a reference to the loopback interface used as the source for the<br />border gateway virtual IP address. A best practice is to use a separate loopback address<br />for the NVE source interface and multi-site source interface. The loopback interface must<br />be configured with a /32 IPv4 address. This /32 IP address needs be known by the transient<br />devices in the transport network and the remote VTEPs. |  | Required: \{\} <br /> |
| `delayRestoreTime` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#duration-v1-meta)_ | DelayRestoreTime specifies the time to wait before restoring EVPN multisite border gateway<br />functionality after a failure. This allows time for the network to stabilize before resuming<br />traffic forwarding across sites. | 180s | Pattern: `^([0-9]+(\.[0-9]+)?(ns\|us\|µs\|ms\|s\|m\|h))+$` <br />Type: string <br />Optional: \{\} <br /> |
| `interconnectInterfaceRefs` _[InterconnectInterfaceReference](#interconnectinterfacereference) array_ | InterconnectInterfaceRefs is a list of interfaces that provide connectivity to the border gateway.<br />Each interface can be configured with object tracking to monitor its availability. |  | MinItems: 1 <br />Optional: \{\} <br /> |
| `bgpPeerRefs` _[BGPPeerReference](#bgppeerreference) array_ | BGPPeerRefs is a list of BGP peers that are part of the border gateway configuration.<br />Each peer can be configured with a peer type to specify its role in the EVPN multisite topology. |  | MinItems: 1 <br />Optional: \{\} <br /> |
| `stormControl` _[StormControl](#stormcontrol) array_ | StormControl is the storm control configuration for the border gateway, allowing to rate-limit<br />BUM (Broadcast, Unknown unicast, Multicast) traffic on the border gateway interface. |  | MinItems: 1 <br />Optional: \{\} <br /> |


#### BorderGatewayStatus



BorderGatewayStatus defines the observed state of BorderGateway.



_Appears in:_
- [BorderGateway](#bordergateway)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the Banner. |  | Optional: \{\} <br /> |


#### BufferBoost



BufferBoost defines the buffer boost configuration for an interface.



_Appears in:_
- [InterfaceConfigSpec](#interfaceconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled indicates whether buffer boost is enabled on the interface.<br />Maps to CLI command: hardware profile buffer boost |  | Required: \{\} <br /> |


#### Console







_Appears in:_
- [ManagementAccessConfigSpec](#managementaccessconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#duration-v1-meta)_ | Timeout defines the inactivity timeout for console sessions.<br />If a session is inactive for the specified duration, it will be automatically disconnected.<br />The format is a string representing a duration (e.g., "10m" for 10 minutes). | 10m | Pattern: `^([0-9]+(\.[0-9]+)?(ns\|us\|µs\|ms\|s\|m\|h))+$` <br />Type: string <br />Optional: \{\} <br /> |


#### Enabled



Enabled represents a simple enabled/disabled configuration.



_Appears in:_
- [Peer](#peer)
- [VPCDomainSpec](#vpcdomainspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled indicates whether a configuration property is administratively enabled (true) or disabled (false). |  | Required: \{\} <br /> |


#### InterconnectInterfaceReference



InterconnectInterfaceReference defines an interface used for border gateway interconnectivity
with optional object tracking configuration.



_Appears in:_
- [BorderGatewaySpec](#bordergatewayspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the referent.<br />More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names |  | MaxLength: 63 <br />MinLength: 1 <br />Required: \{\} <br /> |
| `tracking` _[InterconnectTrackingType](#interconnecttrackingtype)_ | Tracking specifies the EVPN multisite tracking mode for this interconnect interface. |  | Enum: [DataCenterInterconnect Fabric] <br />Required: \{\} <br /> |


#### InterconnectTrackingType

_Underlying type:_ _string_

InterconnectTrackingType defines the tracking mode for border gateway interconnect interfaces.

_Validation:_
- Enum: [DataCenterInterconnect Fabric]

_Appears in:_
- [InterconnectInterfaceReference](#interconnectinterfacereference)

| Field | Description |
| --- | --- |
| `DataCenterInterconnect` | InterconnectTrackingTypeDCI represents Data Center Interconnect tracking mode.<br />Used for interfaces connecting to remote data centers.<br /> |
| `Fabric` | InterconnectTrackingTypeFabric represents Fabric tracking mode.<br />Used for interfaces connecting to the local fabric.<br /> |


#### InterfaceConfig



InterfaceConfig is the Schema for the interfaceconfigs API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nx.cisco.networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `InterfaceConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[InterfaceConfigSpec](#interfaceconfigspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |


#### InterfaceConfigSpec



InterfaceConfigSpec defines the desired state of InterfaceConfig



_Appears in:_
- [InterfaceConfig](#interfaceconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `spanningTree` _[SpanningTree](#spanningtree)_ | SpanningTree defines the spanning tree configuration for the interface. |  | Optional: \{\} <br /> |
| `bufferBoost` _[BufferBoost](#bufferboost)_ | BufferBoost defines the buffer boost configuration for the interface.<br />Buffer boost increases the shared buffer space allocation for the interface. |  | Optional: \{\} <br /> |


#### KeepAlive



KeepAlive defines the vPCDomain keepalive link configuration.
The keep-alive is an out-of-band connection (often over mgmt0) used to monitor
peer health. It does not carry data traffic.



_Appears in:_
- [Peer](#peer)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `destination` _string_ | Destination is the destination IP address of the vPC's domain peer keepalive interface.<br />This is the IP address the local switch will send keepalive messages to. |  | Format: ipv4 <br />Required: \{\} <br /> |
| `source` _string_ | Source is the source IP address for keepalive messages.<br />This is the local IP address used to send keepalive packets to the peer. |  | Format: ipv4 <br />Required: \{\} <br /> |
| `vrfRef` _[LocalObjectReference](#localobjectreference)_ | VRFRef is an optional reference to a VRF resource, e.g., the management VRF.<br />If specified, the switch sends keepalive packets throughout this VRF.<br />If omitted, the management VRF is used. |  | Optional: \{\} <br /> |


#### ManagementAccessConfig



ManagementAccessConfig is the Schema for the managementaccessconfigs API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nx.cisco.networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `ManagementAccessConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ManagementAccessConfigSpec](#managementaccessconfigspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |


#### ManagementAccessConfigSpec



ManagementAccessConfigSpec defines the desired state of ManagementAccessConfig



_Appears in:_
- [ManagementAccessConfig](#managementaccessconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `console` _[Console](#console)_ | Console defines the configuration for the terminal console access on the device. | \{ timeout:10m \} | Optional: \{\} <br /> |
| `ssh` _[SSH](#ssh)_ | SSH defines the SSH server configuration for the VTY terminal access on the device. |  | Optional: \{\} <br /> |


#### NetworkVirtualizationEdgeConfig



NetworkVirtualizationEdgeConfig is the Schema for the NetworkVirtualizationEdgeConfig API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nx.cisco.networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `NetworkVirtualizationEdgeConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[NetworkVirtualizationEdgeConfigSpec](#networkvirtualizationedgeconfigspec)_ | spec defines the desired state of NVE |  | Required: \{\} <br /> |


#### NetworkVirtualizationEdgeConfigSpec



NetworkVirtualizationEdgeConfig defines the Cisco-specific configuration of a Network Virtualization Edge (NVE) object.



_Appears in:_
- [NetworkVirtualizationEdgeConfig](#networkvirtualizationedgeconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `advertiseVirtualMAC` _boolean_ | AdvertiseVirtualMAC controls if the NVE should advertise a virtual MAC address | false | Optional: \{\} <br /> |
| `holdDownTime` _integer_ | HoldDownTime defines the duration for which the switch suppresses the advertisement of the NVE loopback address. | 180 | Maximum: 1500 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `infraVLANs` _[VLANListItem](#vlanlistitem) array_ | InfraVLANs specifies VLANs used by all SVI interfaces for uplink and vPC peer-links in VXLAN as infra-VLANs.<br />The total number of VLANs configured must not exceed 512.<br />Elements in the list must not overlap with each other. |  | MaxItems: 10 <br />Optional: \{\} <br /> |


#### Peer



Peer defines settings to configure peer settings



_Appears in:_
- [VPCDomainSpec](#vpcdomainspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `adminState` _[AdminState](#adminstate)_ | AdminState defines the administrative state of the peer-link. | Up | Enum: [Up Down] <br />Optional: \{\} <br /> |
| `interfaceRef` _[LocalObjectReference](#localobjectreference)_ | InterfaceRef is a reference to an Interface resource and identifies the interface to be used as the vPC domain's peer-link.<br />This interface carries control and data traffic between the two vPC domain peers.<br />It is usually dedicated port-channel, but it can also be a single physical interface. |  | Required: \{\} <br /> |
| `keepalive` _[KeepAlive](#keepalive)_ | KeepAlive defines the out-of-band keepalive configuration. |  | Required: \{\} <br /> |
| `autoRecovery` _[AutoRecovery](#autorecovery)_ | AutoRecovery defines auto-recovery settings for restoring vPC domain after peer failure. |  | Optional: \{\} <br /> |
| `switch` _[Enabled](#enabled)_ | Switch enables peer-switch functionality on this peer.<br />When enabled, both vPC domain peers use the same spanning-tree bridge ID, allowing both<br />to forward traffic for all VLANs without blocking any ports. | \{ enabled:false \} | Optional: \{\} <br /> |
| `gateway` _[Enabled](#enabled)_ | Gateway enables peer-gateway functionality on this peer.<br />When enabled, each vPC domain peer can act as the active gateway for packets destined to the<br />peer's MAC address, improving convergence. | \{ enabled:false \} | Optional: \{\} <br /> |
| `l3router` _[Enabled](#enabled)_ | L3Router enables Layer 3 peer-router functionality on this peer. | \{ enabled:false \} | Optional: \{\} <br /> |


#### SSH







_Appears in:_
- [ManagementAccessConfigSpec](#managementaccessconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `accessControlListName` _string_ | AccessControlListName defines the name of the access control list (ACL) to apply for incoming<br />SSH connections on the VTY terminal. The ACL must be configured separately on the device. |  | MaxLength: 63 <br />MinLength: 1 <br />Optional: \{\} <br /> |


#### SpanningTree



SpanningTree defines the spanning tree configuration for an interface.



_Appears in:_
- [InterfaceConfigSpec](#interfaceconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `portType` _[SpanningTreePortType](#spanningtreeporttype)_ | PortType defines the spanning tree port type. |  | Enum: [Normal Edge Network] <br />Required: \{\} <br /> |
| `bpduGuard` _boolean_ | BPDUGuard enables BPDU guard on the interface.<br />When enabled, the port is shut down if a BPDU is received. |  | Optional: \{\} <br /> |
| `bpduFilter` _boolean_ | BPDUFilter enables BPDU filter on the interface.<br />When enabled, BPDUs are not sent or received on the port. |  | Optional: \{\} <br /> |


#### SpanningTreePortType

_Underlying type:_ _string_

SpanningTreePortType represents the spanning tree port type.

_Validation:_
- Enum: [Normal Edge Network]

_Appears in:_
- [SpanningTree](#spanningtree)

| Field | Description |
| --- | --- |
| `Normal` | SpanningTreePortTypeNormal indicates a normal spanning tree port.<br /> |
| `Edge` | SpanningTreePortTypeEdge indicates an edge port (connects to end devices).<br /> |
| `Network` | SpanningTreePortTypeNetwork indicates a network port (connects to other switches).<br /> |


#### Status

_Underlying type:_ _string_





_Appears in:_
- [VPCDomainStatus](#vpcdomainstatus)

| Field | Description |
| --- | --- |
| `Unknown` |  |
| `Up` |  |
| `Down` |  |


#### StormControl







_Appears in:_
- [BorderGatewaySpec](#bordergatewayspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `level` _string_ | Level is the suppression level as a percentage of the interface bandwidth.<br />Must be a floating point number between 1.0 and 100.0. |  | Pattern: `^([1-9][0-9]?(\.[0-9]+)?\|100(\.0+)?)$` <br />Required: \{\} <br /> |
| `traffic` _[TrafficType](#traffictype)_ | Traffic specifies the type of BUM traffic the storm control applies to. |  | Enum: [Broadcast Multicast Unicast] <br />Required: \{\} <br /> |


#### System



System is the Schema for the systems API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nx.cisco.networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `System` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[SystemSpec](#systemspec)_ | Specification of the desired state of the resource.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Required: \{\} <br /> |
| `status` _[SystemStatus](#systemstatus)_ | Status of the resource. This is set and updated automatically.<br />Read-only.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status |  | Optional: \{\} <br /> |


#### SystemSpec



SystemSpec defines the desired state of System



_Appears in:_
- [System](#system)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `jumboMtu` _integer_ | JumboMtu defines the system-wide jumbo MTU setting.<br />Valid values are from 1501 to 9216. | 9216 | ExclusiveMaximum: false <br />Maximum: 9216 <br />Minimum: 1501 <br />Optional: \{\} <br /> |
| `reservedVlan` _integer_ | ReservedVlan specifies the VLAN ID to be reserved for system use.<br />Valid values are from 1 to 4032. | 3968 | ExclusiveMaximum: false <br />Maximum: 4032 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `vlanLongName` _boolean_ | VlanLongName enables or disables 128-character VLAN names<br />Disabled by default. | false | Optional: \{\} <br /> |


#### SystemStatus



SystemStatus defines the observed state of System.



_Appears in:_
- [System](#system)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | The conditions are a list of status objects that describe the state of the Banner. |  | Optional: \{\} <br /> |


#### TrafficType

_Underlying type:_ _string_

TrafficType defines the type of traffic for storm control.

_Validation:_
- Enum: [Broadcast Multicast Unicast]

_Appears in:_
- [StormControl](#stormcontrol)

| Field | Description |
| --- | --- |
| `Broadcast` | TrafficTypeBroadcast represents broadcast traffic.<br /> |
| `Multicast` | TrafficTypeMulticast represents multicast traffic.<br /> |
| `Unicast` | TrafficTypeUnicast represents unicast traffic.<br /> |


#### VLANListItem



VLANListItem represents a single VLAN ID or a range start-end. If ID is set, rangeMin and rangeMax must be absent. If ID is absent, both rangeMin
and rangeMax must be set.



_Appears in:_
- [NetworkVirtualizationEdgeConfigSpec](#networkvirtualizationedgeconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `id` _integer_ |  |  | Maximum: 3967 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `rangeMin` _integer_ |  |  | Maximum: 3967 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `rangeMax` _integer_ |  |  | Maximum: 3967 <br />Minimum: 1 <br />Optional: \{\} <br /> |


#### VPCDomain



VPCDomain is the Schema for the VPCDomains API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `nx.cisco.networking.metal.ironcore.dev/v1alpha1` | | |
| `kind` _string_ | `VPCDomain` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[VPCDomainSpec](#vpcdomainspec)_ | spec defines the desired state of VPCDomain resource |  | Required: \{\} <br /> |
| `status` _[VPCDomainStatus](#vpcdomainstatus)_ | status defines the observed state of VPCDomain resource |  | Optional: \{\} <br /> |


#### VPCDomainRole

_Underlying type:_ _string_

The VPCDomainRole type represents the operational role of a vPC domain peer as returned by the device.



_Appears in:_
- [VPCDomainStatus](#vpcdomainstatus)

| Field | Description |
| --- | --- |
| `Primary` |  |
| `Primary/Secondary` |  |
| `Secondary` |  |
| `Secondary/Primary` |  |
| `Unknown` |  |


#### VPCDomainSpec



VPCDomainSpec defines the desired state of a vPC domain (Virtual Port Channel Domain)



_Appears in:_
- [VPCDomain](#vpcdomain)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `deviceRef` _[LocalObjectReference](#localobjectreference)_ | DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.<br />Immutable. |  | Required: \{\} <br /> |
| `domainId` _integer_ | DomainID is the vPC domain ID (1-1000).<br />This uniquely identifies the vPC domain and must match on both peer switches.<br />Changing this value will recreate the vPC domain and flap the peer-link. |  | Maximum: 1000 <br />Minimum: 1 <br />Required: \{\} <br /> |
| `adminState` _[AdminState](#adminstate)_ | AdminState is the administrative state of the vPC domain (enabled/disabled).<br />When disabled, the vPC domain is administratively shut down. | Up | Enum: [Up Down] <br />Optional: \{\} <br /> |
| `rolePriority` _integer_ | RolePriority is the role priority for this vPC domain (1-65535).<br />The switch with the lower role priority becomes the operational primary. | 32667 | Maximum: 65535 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `systemPriority` _integer_ | SystemPriority is the system priority for this vPC domain (1-65535).<br />Used to ensure that the vPC domain devices are primary devices on LACP. Must match on both peers. | 32667 | Maximum: 65535 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `delayRestoreSVI` _integer_ | DelayRestoreSVI is the delay in seconds (1-3600) before bringing up interface-vlan (SVI) after peer-link comes up.<br />This prevents traffic blackholing during convergence. | 10 | Maximum: 3600 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `delayRestoreVPC` _integer_ | DelayRestoreVPC is the delay in seconds (1-3600) before bringing up the member ports after the peer-link is restored. | 30 | Maximum: 3600 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `fastConvergence` _[Enabled](#enabled)_ | FastConvergence ensures that both SVIs and member ports are shut down simultaneously when the peer-link goes down.<br />This synchronization helps prevent traffic loss. | \{ enabled:false \} | Optional: \{\} <br /> |
| `peer` _[Peer](#peer)_ | Peer contains the vPC's domain peer configuration including peer-link, keepalive. |  | Required: \{\} <br /> |


#### VPCDomainStatus



VPCDomainStatus defines the observed state of VPCDomain.



_Appears in:_
- [VPCDomain](#vpcdomain)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#condition-v1-meta) array_ | Conditions represent the latest available observations about the vPCDomain state.<br />Standard conditions include:<br />- Ready: overall readiness of the vPC domain<br />- Configured: whether the vPCDomain configuration was successfully applied to the device<br />- Operational: whether the vPC domain is operationally up. This condition is true when<br />  the status fields `PeerLinkIfOperStatus`, `KeepAliveStatus`, and `PeerStatus` are all set<br />  to `UP`.<br />For this Cisco model there is not one single unique operational property that reflects the<br />operational status of the vPC domain. The combination of peer status, keepalive status, and<br />the interface used as peer-link determine the overall health and operational condition of<br />the vPC domain. |  | Optional: \{\} <br /> |
| `role` _[VPCDomainRole](#vpcdomainrole)_ | Role indicates the current operational role of this vPC domain peer. |  | Optional: \{\} <br /> |
| `keepaliveStatus` _[Status](#status)_ | KeepAliveStatus indicates the status of the peer via the keepalive link. |  | Optional: \{\} <br /> |
| `keepaliveStatusMsg` _string array_ | KeepAliveStatusMsg provides additional information about the keepalive status, a list of strings reported by the device. |  | Optional: \{\} <br /> |
| `peerStatus` _[Status](#status)_ | PeerStatus indicates the status of the vPC domain peer-link in the latest consistency check with the peer. This means that if<br />the adjacency is lost, e.g., due to a shutdown link, the device will not be able to perform such check and the reported status<br />will remain unchanged (with the value of the last check). |  | Optional: \{\} <br /> |
| `peerStatusMsg` _string array_ | PeerStatusMsg provides additional information about the peer status, a list of strings reported by the device. |  | Optional: \{\} <br /> |
| `peerUptime` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.35/#duration-v1-meta)_ | PeerUptime indicates how long the vPC domain peer has been up and reachable via keepalive. |  | Optional: \{\} <br /> |
| `peerLinkIf` _string_ | PeerLinkIf is the name of the interface used as the vPC domain peer-link. |  | Optional: \{\} <br /> |
| `peerLinkIfOperStatus` _[Status](#status)_ | PeerLinkIfOperStatus is the Operational status of `PeerLinkIf`. |  | Optional: \{\} <br /> |



## xe.cisco.networking.metal.ironcore.dev/v1alpha1

Package v1alpha1 contains API Schema definitions for the xe.cisco.networking.metal.ironcore.dev v1alpha1 API group.




## xr.cisco.networking.metal.ironcore.dev/v1alpha1

Package v1alpha1 contains API Schema definitions for the xr.cisco.networking.metal.ironcore.dev v1alpha1 API group.



