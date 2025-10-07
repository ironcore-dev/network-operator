<p>Packages:</p>
<ul>
<li>
<a href="#networking.cloud.sap%2fv1alpha1">networking.cloud.sap/v1alpha1</a>
</li>
</ul>
<h2 id="networking.cloud.sap/v1alpha1">networking.cloud.sap/v1alpha1</h2>
<div>
<p>Package v1alpha1 contains API Schema definitions for the networking.cloud.sap API group</p>
</div>
Resource Types:
<ul></ul>
<h3 id="networking.cloud.sap/v1alpha1.ACL">ACL
</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the access control list.</p>
</td>
</tr>
<tr>
<td>
<code>entries</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.*./api/v1alpha1.ACLEntry">
[]*./api/v1alpha1.ACLEntry
</a>
</em>
</td>
<td>
<p>A list of rules/entries to apply.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.ACLAction">ACLAction
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.ACLEntry">ACLEntry</a>)
</p>
<div>
<p>ACLAction represents the type of action that can be taken by an ACL rule.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Deny&#34;</p></td>
<td><p>ActionDeny blocks traffic that matches the rule.</p>
</td>
</tr><tr><td><p>&#34;Permit&#34;</p></td>
<td><p>ActionPermit allows traffic that matches the rule.</p>
</td>
</tr></tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.ACLEntry">ACLEntry
</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>sequence</code><br/>
<em>
int
</em>
</td>
<td>
<p>The sequence number of the ACL entry.</p>
</td>
</tr>
<tr>
<td>
<code>action</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.ACLAction">
ACLAction
</a>
</em>
</td>
<td>
<p>The forwarding action of the ACL entry.</p>
</td>
</tr>
<tr>
<td>
<code>protocol</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The protocol to match. If not specified, defaults to &ldquo;ip&rdquo; (IPv4).</p>
</td>
</tr>
<tr>
<td>
<code>sourceAddress</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.IPPrefix">
IPPrefix
</a>
</em>
</td>
<td>
<p>Source IPv4 address prefix. Use 0.0.0.0/0 to represent &lsquo;any&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>destinationAddress</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.IPPrefix">
IPPrefix
</a>
</em>
</td>
<td>
<p>Destination IPv4 address prefix. Use 0.0.0.0/0 to represent &lsquo;any&rsquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.AdminState">AdminState
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.InterfaceSpec">InterfaceSpec</a>)
</p>
<div>
<p>AdminState represents the administrative state of the interface.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Down&#34;</p></td>
<td><p>AdminStateDown indicates that the interface is administratively set down.</p>
</td>
</tr><tr><td><p>&#34;Up&#34;</p></td>
<td><p>AdminStateUp indicates that the interface is administratively set up.</p>
</td>
</tr></tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.Bootstrap">Bootstrap
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.DeviceSpec">DeviceSpec</a>)
</p>
<div>
<p>Bootstrap defines the configuration for device bootstrap.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.TemplateSource">
TemplateSource
</a>
</em>
</td>
<td>
<p>Template defines the multiline string template that contains the initial configuration for the device.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.Certificate">Certificate
</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the certificate.</p>
</td>
</tr>
<tr>
<td>
<code>source</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.CertificateSource">
CertificateSource
</a>
</em>
</td>
<td>
<p>The source of the certificate content.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.CertificateSource">CertificateSource
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.Certificate">Certificate</a>, <a href="#networking.cloud.sap/v1alpha1.TLS">TLS</a>)
</p>
<div>
<p>CertificateSource represents a source for the value of a certificate.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>secretRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<p>Secret containing the certificate.
The secret must be of type kubernetes.io/tls and as such contain the following keys: &lsquo;tls.crt&rsquo; and &lsquo;tls.key&rsquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.DNS">DNS
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.DeviceSpec">DeviceSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>domain</code><br/>
<em>
string
</em>
</td>
<td>
<p>Default domain name that the switch uses to complete unqualified hostnames.</p>
</td>
</tr>
<tr>
<td>
<code>servers</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.*./api/v1alpha1.NameServer">
[]*./api/v1alpha1.NameServer
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>A list of DNS servers to use for address resolution.</p>
</td>
</tr>
<tr>
<td>
<code>srcIf</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Source interface for all DNS traffic.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.Device">Device
</h3>
<div>
<p>Device is the Schema for the devices API.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.DeviceSpec">
DeviceSpec
</a>
</em>
</td>
<td>
<p>Specification of the desired state of the resource.
More info: <a href="https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status">https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status</a></p>
<br/>
<br/>
<table>
<tr>
<td>
<code>endpoint</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.Endpoint">
Endpoint
</a>
</em>
</td>
<td>
<p>Endpoint contains the connection information for the device.</p>
</td>
</tr>
<tr>
<td>
<code>bootstrap</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.Bootstrap">
Bootstrap
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Bootstrap is an optional configuration for the device bootstrap process.
It can be used to provide initial configuration templates or scripts that are applied during the device provisioning.</p>
</td>
</tr>
<tr>
<td>
<code>dns</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.DNS">
DNS
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Top-level configuration for DNS / resolver.</p>
</td>
</tr>
<tr>
<td>
<code>ntp</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.NTP">
NTP
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configuration data for system-wide NTP process.</p>
</td>
</tr>
<tr>
<td>
<code>acl</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.*./api/v1alpha1.ACL">
[]*./api/v1alpha1.ACL
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Access Control Lists (ACLs) configuration.</p>
</td>
</tr>
<tr>
<td>
<code>pki</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.PKI">
PKI
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PKI configuration for managing certificates on the device.</p>
</td>
</tr>
<tr>
<td>
<code>logging</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.Logging">
Logging
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Top-level logging configuration for the device.</p>
</td>
</tr>
<tr>
<td>
<code>snmp</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.SNMP">
SNMP
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SNMP global configuration.</p>
</td>
</tr>
<tr>
<td>
<code>users</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.*./api/v1alpha1.User">
[]*./api/v1alpha1.User
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of local users on the switch.</p>
</td>
</tr>
<tr>
<td>
<code>grpc</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.GRPC">
GRPC
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configuration for the gRPC server on the device.
Currently, only a single &ldquo;default&rdquo; gRPC server is supported.</p>
</td>
</tr>
<tr>
<td>
<code>banner</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.TemplateSource">
TemplateSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MOTD banner to display on login.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.DeviceStatus">
DeviceStatus
</a>
</em>
</td>
<td>
<p>Status of the resource. This is set and updated automatically.
Read-only.
More info: <a href="https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status">https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.DevicePhase">DevicePhase
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.DeviceStatus">DeviceStatus</a>)
</p>
<div>
<p>DevicePhase represents the current phase of the Device as it&rsquo;s being provisioned and managed by the operator.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Active&#34;</p></td>
<td><p>DevicePhaseActive indicates that the device has been successfully provisioned and is now ready for use.</p>
</td>
</tr><tr><td><p>&#34;Failed&#34;</p></td>
<td><p>DevicePhaseFailed indicates that the device provisioning has failed.</p>
</td>
</tr><tr><td><p>&#34;Pending&#34;</p></td>
<td><p>DevicePhasePending indicates that the device is pending and has not yet been provisioned.</p>
</td>
</tr><tr><td><p>&#34;Provisioning&#34;</p></td>
<td><p>DevicePhaseProvisioning indicates that the device is being provisioned.</p>
</td>
</tr></tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.DeviceSpec">DeviceSpec
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.Device">Device</a>)
</p>
<div>
<p>DeviceSpec defines the desired state of Device.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>endpoint</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.Endpoint">
Endpoint
</a>
</em>
</td>
<td>
<p>Endpoint contains the connection information for the device.</p>
</td>
</tr>
<tr>
<td>
<code>bootstrap</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.Bootstrap">
Bootstrap
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Bootstrap is an optional configuration for the device bootstrap process.
It can be used to provide initial configuration templates or scripts that are applied during the device provisioning.</p>
</td>
</tr>
<tr>
<td>
<code>dns</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.DNS">
DNS
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Top-level configuration for DNS / resolver.</p>
</td>
</tr>
<tr>
<td>
<code>ntp</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.NTP">
NTP
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configuration data for system-wide NTP process.</p>
</td>
</tr>
<tr>
<td>
<code>acl</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.*./api/v1alpha1.ACL">
[]*./api/v1alpha1.ACL
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Access Control Lists (ACLs) configuration.</p>
</td>
</tr>
<tr>
<td>
<code>pki</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.PKI">
PKI
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PKI configuration for managing certificates on the device.</p>
</td>
</tr>
<tr>
<td>
<code>logging</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.Logging">
Logging
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Top-level logging configuration for the device.</p>
</td>
</tr>
<tr>
<td>
<code>snmp</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.SNMP">
SNMP
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SNMP global configuration.</p>
</td>
</tr>
<tr>
<td>
<code>users</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.*./api/v1alpha1.User">
[]*./api/v1alpha1.User
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>List of local users on the switch.</p>
</td>
</tr>
<tr>
<td>
<code>grpc</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.GRPC">
GRPC
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configuration for the gRPC server on the device.
Currently, only a single &ldquo;default&rdquo; gRPC server is supported.</p>
</td>
</tr>
<tr>
<td>
<code>banner</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.TemplateSource">
TemplateSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>MOTD banner to display on login.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.DeviceStatus">DeviceStatus
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.Device">Device</a>)
</p>
<div>
<p>DeviceStatus defines the observed state of Device.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>phase</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.DevicePhase">
DevicePhase
</a>
</em>
</td>
<td>
<p>Phase represents the current phase of the Device.</p>
</td>
</tr>
<tr>
<td>
<code>conditions</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The conditions are a list of status objects that describe the state of the Device.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.Endpoint">Endpoint
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.DeviceSpec">DeviceSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>address</code><br/>
<em>
string
</em>
</td>
<td>
<p>Address is the management address of the device provided as <a href="ip:port">ip:port</a>.</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretreference-v1-core">
Kubernetes core/v1.SecretReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SecretRef is name of the authentication secret for the device containing the username and password.
The secret must be of type kubernetes.io/basic-auth and as such contain the following keys: &lsquo;username&rsquo; and &lsquo;password&rsquo;.</p>
</td>
</tr>
<tr>
<td>
<code>tls</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.TLS">
TLS
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Transport credentials for grpc connection to the switch.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.GNMI">GNMI
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.GRPC">GRPC</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>maxConcurrentCall</code><br/>
<em>
byte
</em>
</td>
<td>
<em>(Optional)</em>
<p>The maximum number of concurrent gNMI calls that can be made to the gRPC server on the switch for each VRF.
Configure a limit from 1 through 16. The default limit is 8.</p>
</td>
</tr>
<tr>
<td>
<code>keepAliveTimeout</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#duration-v1-meta">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configure the keepalive timeout for inactive or unauthorized connections.
The gRPC agent is expected to periodically send an empty response to the client, on which the client is expected to respond with an empty request.
If the client does not respond within the keepalive timeout, the gRPC agent should close the connection.
The default interval value is 10 minutes.</p>
</td>
</tr>
<tr>
<td>
<code>minSampleInterval</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#duration-v1-meta">
Kubernetes meta/v1.Duration
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Configure the minimum sample interval for the gNMI telemetry stream.
Once per stream sample interval, the switch sends the current values for all specified paths.
The default value is 10 seconds.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.GRPC">GRPC
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.DeviceSpec">DeviceSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>port</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>The TCP port on which the gRPC server should listen.
The range of port-id is from 1024 to 65535.
Port 9339 is the default.</p>
</td>
</tr>
<tr>
<td>
<code>certificateId</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Name of the certificate that is associated with the gRPC service.
The certificate is provisioned through other interfaces on the device,
such as e.g. the gNOI certificate management service.</p>
</td>
</tr>
<tr>
<td>
<code>networkInstance</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Enable the gRPC agent to accept incoming (dial-in) RPC requests from a given network instance.</p>
</td>
</tr>
<tr>
<td>
<code>gnmi</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.GNMI">
GNMI
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Additional gNMI configuration for the gRPC server.
This may not be supported by all devices.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.IPPrefix">IPPrefix
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.ACLEntry">ACLEntry</a>)
</p>
<div>
<p>IPPrefix represents an IP prefix in CIDR notation.
It is used to define a range of IP addresses in a network.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>-</code><br/>
<em>
<a href="https://pkg.go.dev/net/netip#Prefix">
net/netip.Prefix
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.Interface">Interface
</h3>
<div>
<p>Interface is the Schema for the interfaces API.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.InterfaceSpec">
InterfaceSpec
</a>
</em>
</td>
<td>
<p>Specification of the desired state of the resource.
More info: <a href="https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status">https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status</a></p>
<br/>
<br/>
<table>
<tr>
<td>
<code>deviceRef</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.LocalObjectReference">
LocalObjectReference
</a>
</em>
</td>
<td>
<p>DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
Immutable.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfigRef</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.TypedLocalObjectReference">
TypedLocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
This reference is used to link the Interface to its provider-specific configuration.</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the interface.</p>
</td>
</tr>
<tr>
<td>
<code>adminState</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.AdminState">
AdminState
</a>
</em>
</td>
<td>
<p>AdminState indicates whether the interface is administratively up or down.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description provides a human-readable description of the interface.</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.InterfaceType">
InterfaceType
</a>
</em>
</td>
<td>
<p>Type indicates the type of the interface.</p>
</td>
</tr>
<tr>
<td>
<code>mtu</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>MTU (Maximum Transmission Unit) specifies the size of the largest packet that can be sent over the interface.</p>
</td>
</tr>
<tr>
<td>
<code>switchport</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.Switchport">
Switchport
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Switchport defines the switchport configuration for the interface.
This is only applicable for interfaces that are switchports (e.g., Ethernet interfaces).</p>
</td>
</tr>
<tr>
<td>
<code>ipv4Addresses</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Ipv4Addresses is the list of IPv4 addresses assigned to the interface.
Each address should be given either in CIDR notation (e.g., &ldquo;10.0.0.<sup>1</sup>&frasl;<sub>32</sub>&rdquo;)
or as interface reference in the form of &ldquo;unnumbered:<source-interface>&rdquo; (e.g., &ldquo;unnumbered:lo0&rdquo;).</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status,omitempty,omitzero</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.InterfaceStatus">
InterfaceStatus
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Status of the resource. This is set and updated automatically.
Read-only.
More info: <a href="https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status">https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.InterfaceSpec">InterfaceSpec
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.Interface">Interface</a>)
</p>
<div>
<p>InterfaceSpec defines the desired state of Interface.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>deviceRef</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.LocalObjectReference">
LocalObjectReference
</a>
</em>
</td>
<td>
<p>DeviceName is the name of the Device this object belongs to. The Device object must exist in the same namespace.
Immutable.</p>
</td>
</tr>
<tr>
<td>
<code>providerConfigRef</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.TypedLocalObjectReference">
TypedLocalObjectReference
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>ProviderConfigRef is a reference to a resource holding the provider-specific configuration of this interface.
This reference is used to link the Interface to its provider-specific configuration.</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the interface.</p>
</td>
</tr>
<tr>
<td>
<code>adminState</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.AdminState">
AdminState
</a>
</em>
</td>
<td>
<p>AdminState indicates whether the interface is administratively up or down.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Description provides a human-readable description of the interface.</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.InterfaceType">
InterfaceType
</a>
</em>
</td>
<td>
<p>Type indicates the type of the interface.</p>
</td>
</tr>
<tr>
<td>
<code>mtu</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>MTU (Maximum Transmission Unit) specifies the size of the largest packet that can be sent over the interface.</p>
</td>
</tr>
<tr>
<td>
<code>switchport</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.Switchport">
Switchport
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Switchport defines the switchport configuration for the interface.
This is only applicable for interfaces that are switchports (e.g., Ethernet interfaces).</p>
</td>
</tr>
<tr>
<td>
<code>ipv4Addresses</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Ipv4Addresses is the list of IPv4 addresses assigned to the interface.
Each address should be given either in CIDR notation (e.g., &ldquo;10.0.0.<sup>1</sup>&frasl;<sub>32</sub>&rdquo;)
or as interface reference in the form of &ldquo;unnumbered:<source-interface>&rdquo; (e.g., &ldquo;unnumbered:lo0&rdquo;).</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.InterfaceStatus">InterfaceStatus
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.Interface">Interface</a>)
</p>
<div>
<p>InterfaceStatus defines the observed state of Interface.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>conditions</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#condition-v1-meta">
[]Kubernetes meta/v1.Condition
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The conditions are a list of status objects that describe the state of the Interface.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.InterfaceType">InterfaceType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.InterfaceSpec">InterfaceSpec</a>)
</p>
<div>
<p>InterfaceType represents the type of the interface.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Loopback&#34;</p></td>
<td><p>InterfaceTypeLoopback indicates that the interface is a loopback interface.</p>
</td>
</tr><tr><td><p>&#34;Physical&#34;</p></td>
<td><p>InterfaceTypePhysical indicates that the interface is a physical/ethernet interface.</p>
</td>
</tr></tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.LocalObjectReference">LocalObjectReference
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.InterfaceSpec">InterfaceSpec</a>)
</p>
<div>
<p>LocalObjectReference contains enough information to locate a
referenced object inside the same namespace.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the referent.
More info: <a href="https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names">https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names</a></p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.LogFacility">LogFacility
</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the log facility.</p>
</td>
</tr>
<tr>
<td>
<code>severity</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.Severity">
Severity
</a>
</em>
</td>
<td>
<p>The severity level of the log messages for this facility.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.LogServer">LogServer
</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>address</code><br/>
<em>
string
</em>
</td>
<td>
<p>IP address or hostname of the remote log server</p>
</td>
</tr>
<tr>
<td>
<code>severity</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.Severity">
Severity
</a>
</em>
</td>
<td>
<p>The servity level of the log messages sent to the server.</p>
</td>
</tr>
<tr>
<td>
<code>networkInstance</code><br/>
<em>
string
</em>
</td>
<td>
<p>The network instance used to reach the log server.</p>
</td>
</tr>
<tr>
<td>
<code>port</code><br/>
<em>
int64
</em>
</td>
<td>
<em>(Optional)</em>
<p>The destination port number for syslog UDP messages to
the server. The default is 514.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.Logging">Logging
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.DeviceSpec">DeviceSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>servers</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.*./api/v1alpha1.LogServer">
[]*./api/v1alpha1.LogServer
</a>
</em>
</td>
<td>
<p>Servers is a list of remote log servers to which the device will send logs.</p>
</td>
</tr>
<tr>
<td>
<code>facilities</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.*./api/v1alpha1.LogFacility">
[]*./api/v1alpha1.LogFacility
</a>
</em>
</td>
<td>
<p>Facilities is a list of log facilities to configure on the device.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.NTP">NTP
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.DeviceSpec">DeviceSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>srcIf</code><br/>
<em>
string
</em>
</td>
<td>
<p>Source interface for all NTP traffic.</p>
</td>
</tr>
<tr>
<td>
<code>servers</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.*./api/v1alpha1.NTPServer">
[]*./api/v1alpha1.NTPServer
</a>
</em>
</td>
<td>
<p>NTP servers.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.NTPServer">NTPServer
</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>address</code><br/>
<em>
string
</em>
</td>
<td>
<p>Hostname/IP address of the NTP server.</p>
</td>
</tr>
<tr>
<td>
<code>prefer</code><br/>
<em>
bool
</em>
</td>
<td>
<em>(Optional)</em>
<p>Indicates whether this server should be preferred or not.</p>
</td>
</tr>
<tr>
<td>
<code>networkInstance</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The network instance used to communicate with the NTP server.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.NameServer">NameServer
</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>address</code><br/>
<em>
string
</em>
</td>
<td>
<p>The Hostname or IP address of the DNS server.</p>
</td>
</tr>
<tr>
<td>
<code>networkInstance</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The network instance used to communicate with the DNS server.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.PKI">PKI
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.DeviceSpec">DeviceSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>certificates</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.*./api/v1alpha1.Certificate">
[]*./api/v1alpha1.Certificate
</a>
</em>
</td>
<td>
<p>Certificates is a list of certificates to be managed by the PKI.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.PasswordSource">PasswordSource
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.User">User</a>)
</p>
<div>
<p>PasswordSource represents a source for the value of a password.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>secretKeyRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretkeyselector-v1-core">
Kubernetes core/v1.SecretKeySelector
</a>
</em>
</td>
<td>
<p>Selects a key of a secret.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.SNMP">SNMP
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.DeviceSpec">DeviceSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>contact</code><br/>
<em>
string
</em>
</td>
<td>
<p>The contact information for the SNMP server.</p>
</td>
</tr>
<tr>
<td>
<code>location</code><br/>
<em>
string
</em>
</td>
<td>
<p>The location information for the SNMP server.</p>
</td>
</tr>
<tr>
<td>
<code>engineId</code><br/>
<em>
string
</em>
</td>
<td>
<p>The SNMP engine ID for the SNMP server.</p>
</td>
</tr>
<tr>
<td>
<code>srcIf</code><br/>
<em>
string
</em>
</td>
<td>
<p>Source interface to be used for sending out SNMP Trap/Inform notifications.</p>
</td>
</tr>
<tr>
<td>
<code>communities</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.*./api/v1alpha1.SNMPCommunity">
[]*./api/v1alpha1.SNMPCommunity
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>SNMP communities for SNMPv1 or SNMPv2c.</p>
</td>
</tr>
<tr>
<td>
<code>destinations</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.*./api/v1alpha1.SNMPDestination">
[]*./api/v1alpha1.SNMPDestination
</a>
</em>
</td>
<td>
<p>SNMP destinations for SNMP traps or informs.</p>
</td>
</tr>
<tr>
<td>
<code>traps</code><br/>
<em>
[]string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The list of trap groups to enable.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.SNMPCommunity">SNMPCommunity
</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Name of the community.</p>
</td>
</tr>
<tr>
<td>
<code>group</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Group to which the community belongs.</p>
</td>
</tr>
<tr>
<td>
<code>acl</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>ACL name to filter snmp requests.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.SNMPDestination">SNMPDestination
</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>address</code><br/>
<em>
string
</em>
</td>
<td>
<p>The Hostname or IP address of the SNMP host to send notifications to.</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Type of message to send to host. Default is traps.</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SNMP version. Default is v2c.</p>
</td>
</tr>
<tr>
<td>
<code>target</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>SNMP community or user name.</p>
</td>
</tr>
<tr>
<td>
<code>networkInstance</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>The network instance to use to source traffic.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.Severity">Severity
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.LogFacility">LogFacility</a>, <a href="#networking.cloud.sap/v1alpha1.LogServer">LogServer</a>)
</p>
<div>
<p>Severity represents the severity level of a log message.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Alert&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Critical&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Debug&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Emergency&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Error&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Info&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Notice&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;Warning&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.Switchport">Switchport
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.InterfaceSpec">InterfaceSpec</a>)
</p>
<div>
<p>Switchport defines the switchport configuration for an interface.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>mode</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.SwitchportMode">
SwitchportMode
</a>
</em>
</td>
<td>
<p>Mode defines the switchport mode, such as access or trunk.</p>
</td>
</tr>
<tr>
<td>
<code>accessVlan</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>AccessVlan specifies the VLAN ID for access mode switchports.
Only applicable when Mode is set to &ldquo;Access&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>nativeVlan</code><br/>
<em>
int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>NativeVlan specifies the native VLAN ID for trunk mode switchports.
Only applicable when Mode is set to &ldquo;Trunk&rdquo;.</p>
</td>
</tr>
<tr>
<td>
<code>allowedVlans</code><br/>
<em>
[]int32
</em>
</td>
<td>
<em>(Optional)</em>
<p>AllowedVlans is a list of VLAN IDs that are allowed on the trunk port.
Only applicable when Mode is set to &ldquo;Trunk&rdquo;.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.SwitchportMode">SwitchportMode
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.Switchport">Switchport</a>)
</p>
<div>
<p>SwitchportMode represents the switchport mode of an interface.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Access&#34;</p></td>
<td><p>SwitchportModeAccess indicates that the switchport is in access mode.</p>
</td>
</tr><tr><td><p>&#34;Trunk&#34;</p></td>
<td><p>SwitchportModeTrunk indicates that the switchport is in trunk mode.</p>
</td>
</tr></tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.TLS">TLS
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.Endpoint">Endpoint</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ca</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretkeyselector-v1-core">
Kubernetes core/v1.SecretKeySelector
</a>
</em>
</td>
<td>
<p>The CA certificate to verify the server&rsquo;s identity.</p>
</td>
</tr>
<tr>
<td>
<code>certificate</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.CertificateSource">
CertificateSource
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>The client certificate and private key to use for mutual TLS authentication.
Leave empty if mTLS is not desired.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.TemplateSource">TemplateSource
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.Bootstrap">Bootstrap</a>, <a href="#networking.cloud.sap/v1alpha1.DeviceSpec">DeviceSpec</a>)
</p>
<div>
<p>TemplateSource defines a source for template content.
It can be provided inline, or as a reference to a Secret or ConfigMap.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>inline</code><br/>
<em>
string
</em>
</td>
<td>
<em>(Optional)</em>
<p>Inline template content</p>
</td>
</tr>
<tr>
<td>
<code>secretRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#secretkeyselector-v1-core">
Kubernetes core/v1.SecretKeySelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Reference to a Secret containing the template</p>
</td>
</tr>
<tr>
<td>
<code>configMapRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.32/#configmapkeyselector-v1-core">
Kubernetes core/v1.ConfigMapKeySelector
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>Reference to a ConfigMap containing the template</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.TypedLocalObjectReference">TypedLocalObjectReference
</h3>
<p>
(<em>Appears on:</em><a href="#networking.cloud.sap/v1alpha1.InterfaceSpec">InterfaceSpec</a>)
</p>
<div>
<p>TypedLocalObjectReference contains enough information to locate a
typed referenced object inside the same namespace.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>kind</code><br/>
<em>
string
</em>
</td>
<td>
<p>Kind of the resource being referenced.
Kind must consist of alphanumeric characters or &lsquo;-&rsquo;, start with an alphabetic character, and end with an alphanumeric character.</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the resource being referenced.
Name must consist of lower case alphanumeric characters, &lsquo;-&rsquo; or &lsquo;.&rsquo;, and must start and end with an alphanumeric character.</p>
</td>
</tr>
<tr>
<td>
<code>apiVersion</code><br/>
<em>
string
</em>
</td>
<td>
<p>APIVersion is the api group version of the resource being referenced.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.cloud.sap/v1alpha1.User">User
</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Assigned username for this user.</p>
</td>
</tr>
<tr>
<td>
<code>password</code><br/>
<em>
<a href="#networking.cloud.sap/v1alpha1.PasswordSource">
PasswordSource
</a>
</em>
</td>
<td>
<p>The user password, supplied as cleartext.</p>
</td>
</tr>
<tr>
<td>
<code>role</code><br/>
<em>
string
</em>
</td>
<td>
<p>Role which the user is to be assigned to.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>
