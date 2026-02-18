// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"syscall"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	// Import all supported provider implementations.
	_ "github.com/ironcore-dev/network-operator/internal/provider/cisco/nxos"
	_ "github.com/ironcore-dev/network-operator/internal/provider/openconfig"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/clientutil"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

var (
	address      = flag.String("address", "", "API endpoint address (required)")
	username     = flag.String("username", "", "Username for authentication (required)")
	password     = flag.String("password", "", "Password for authentication (required)")
	file         = flag.String("file", "", "Path to Kubernetes resource manifest file (required)")
	providerName = flag.String("provider", "openconfig", "Provider implementation to use")
	refFiles     = flag.String("ref-files", "", "Comma-separated list of YAML files containing referenced resources")
)

// ReferenceStore holds referenced resources keyed by "namespace/name".
type ReferenceStore map[string]client.Object

// Get retrieves a resource from the store by namespace and name.
// If namespace is empty, it defaults to "default".
func (r ReferenceStore) Get(name, namespace string) client.Object {
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}
	key := namespace + "/" + name
	return r[key]
}

// Global reference store for resources loaded from --ref-files
var refStore = make(ReferenceStore)

// refStoreReader implements [client.Reader] interface for reading resources from refStore.
type refStoreReader struct {
	store ReferenceStore
}

func (r *refStoreReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	o := r.store.Get(key.Name, key.Namespace)
	if o == nil {
		return fmt.Errorf("resource %s/%s not found in reference files", key.Namespace, key.Name)
	}
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Pointer {
		return errors.New("obj must be a pointer")
	}
	rv := reflect.ValueOf(o)
	if rv.Type() != v.Type() {
		return fmt.Errorf("type mismatch: want %T, got %T", obj, o)
	}
	v.Elem().Set(rv.Elem())
	return nil
}

func (r *refStoreReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return errors.New("List operation not supported by refStoreReader")
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [flags] <create|delete>\n\n", os.Args[0]) // #nosec G705
	fmt.Fprintf(os.Stderr, "A debug tool for testing provider implementations.\n\n")
	fmt.Fprintf(os.Stderr, "This tool allows you to directly test provider implementations by creating or\n")
	fmt.Fprintf(os.Stderr, "deleting resources on network devices.\n\n")
	fmt.Fprintf(os.Stderr, "Arguments:\n")
	fmt.Fprintf(os.Stderr, "  create|delete    Operation to perform on the resource\n\n")
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nExample:\n")
	fmt.Fprintf(os.Stderr, "  %s -address=192.168.1.1:9339 -username=admin -password=secret -file=config/samples/v1alpha1_interface.yaml create\n", os.Args[0]) // #nosec G705
}

func validateFlags() error {
	if *address == "" {
		return errors.New("address flag is required")
	}
	if *username == "" {
		return errors.New("username flag is required")
	}
	if *password == "" {
		return errors.New("password flag is required")
	}
	if *file == "" {
		return errors.New("file flag is required")
	}
	return nil
}

func validatePositionalArgs() (string, error) {
	if len(flag.Args()) != 1 {
		return "", errors.New("exactly one positional argument (create|delete) is required")
	}

	operation := flag.Args()[0]
	if operation != "create" && operation != "delete" {
		return "", fmt.Errorf("positional argument must be either 'create' or 'delete', got: %s", operation)
	}

	return operation, nil
}

func loadAndUnmarshalResource(path string) (runtime.Object, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	if err = v1alpha1.AddToScheme(scheme.Scheme); err != nil {
		return nil, fmt.Errorf("failed to add scheme: %w", err)
	}

	decoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDeserializer()

	json, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}

	obj, _, err := decoder.Decode(json, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode resource: %w", err)
	}

	return obj, nil
}

func printResourceInfo(obj runtime.Object) {
	switch resource := obj.(type) {
	case *v1alpha1.AccessControlList:
		fmt.Printf("Loaded AccessControlList: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  ACL Name: %s\n", resource.Spec.Name)
		fmt.Printf("  Entries: %d\n", len(resource.Spec.Entries))
	case *v1alpha1.Banner:
		fmt.Printf("Loaded Banner: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Type: %s\n", resource.Spec.Type)
	case *v1alpha1.BGP:
		fmt.Printf("Loaded BGP: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  AS Number: %s\n", resource.Spec.ASNumber.String())
		fmt.Printf("  Router ID: %s\n", resource.Spec.RouterID)
	case *v1alpha1.BGPPeer:
		fmt.Printf("Loaded BGPPeer: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Address: %s\n", resource.Spec.Address)
		fmt.Printf("  AS Number: %s\n", resource.Spec.ASNumber.String())
	case *v1alpha1.Certificate:
		fmt.Printf("Loaded Certificate: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  ID: %s\n", resource.Spec.ID)
	case *v1alpha1.DNS:
		fmt.Printf("Loaded DNS: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Admin State: %s\n", resource.Spec.AdminState)
		fmt.Printf("  Domain: %s\n", resource.Spec.Domain)
		fmt.Printf("  Servers: %v\n", resource.Spec.Servers)
		fmt.Printf("  Source Interface: %v\n", resource.Spec.SourceInterfaceName)
	case *v1alpha1.EVPNInstance:
		fmt.Printf("Loaded EVPNInstance: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Type: %v\n", resource.Spec.Type)
		fmt.Printf("  VNI: %d\n", resource.Spec.VNI)
	case *v1alpha1.Interface:
		fmt.Printf("Loaded Interface: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Interface Name: %s\n", resource.Spec.Name)
		fmt.Printf("  Admin State: %s\n", resource.Spec.AdminState)
	case *v1alpha1.ISIS:
		fmt.Printf("Loaded ISIS: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Instance: %s\n", resource.Spec.Instance)
		fmt.Printf("  NET: %s\n", resource.Spec.NetworkEntityTitle)
	case *v1alpha1.ManagementAccess:
		fmt.Printf("Loaded ManagementAccess: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
	case *v1alpha1.NTP:
		fmt.Printf("Loaded NTP: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Admin State: %s\n", resource.Spec.AdminState)
		fmt.Printf("  Servers: %v\n", resource.Spec.Servers)
		fmt.Printf("  Source Interface: %v\n", resource.Spec.SourceInterfaceName)
	case *v1alpha1.OSPF:
		fmt.Printf("Loaded OSPF: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Admin State: %s\n", resource.Spec.AdminState)
		fmt.Printf("  Instance: %s\n", resource.Spec.Instance)
	case *v1alpha1.PIM:
		fmt.Printf("Loaded PIM: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Admin State: %s\n", resource.Spec.AdminState)
	case *v1alpha1.PrefixSet:
		fmt.Printf("Loaded PrefixSet: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Prefix Set Name: %s\n", resource.Spec.Name)
	case *v1alpha1.RoutingPolicy:
		fmt.Printf("Loaded RoutingPolicy: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Policy Name: %s\n", resource.Spec.Name)
	case *v1alpha1.SNMP:
		fmt.Printf("Loaded SNMP: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Source Interface: %s\n", resource.Spec.SourceInterfaceName)
	case *v1alpha1.Syslog:
		fmt.Printf("Loaded Syslog: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Servers: %v\n", resource.Spec.Servers)
		fmt.Printf("  Facilities: %v\n", resource.Spec.Facilities)
	case *v1alpha1.User:
		fmt.Printf("Loaded User: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Username: %s\n", resource.Spec.Username)
		fmt.Printf("  Roles: %v\n", resource.Spec.Roles)
	case *v1alpha1.VLAN:
		fmt.Printf("Loaded VLAN: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  Admin State: %s\n", resource.Spec.AdminState)
		fmt.Printf("  VLAN ID: %d\n", resource.Spec.ID)
	case *v1alpha1.VRF:
		fmt.Printf("Loaded VRF: %s\n", resource.Name)
		fmt.Printf("  Namespace: %s\n", resource.Namespace)
		fmt.Printf("  VRF Name: %s\n", resource.Spec.Name)
	default:
		fmt.Printf("Loaded resource of unknown type: %T\n", resource)
	}
}

// loadReferenceFiles loads referenced resources from comma-separated YAML files
// into the global refStore.
func loadReferenceFiles(files string) error {
	if files == "" {
		return nil
	}

	for path := range strings.SplitSeq(files, ",") {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}

		obj, err := loadAndUnmarshalResource(path)
		if err != nil {
			return fmt.Errorf("failed to load reference file %s: %w", path, err)
		}

		o, ok := obj.(client.Object)
		if !ok {
			return fmt.Errorf("resource in %s is not a client.Object", path)
		}

		ns := o.GetNamespace()
		if ns == "" {
			ns = metav1.NamespaceDefault
		}

		key := ns + "/" + o.GetName()
		refStore[key] = o
	}

	return nil
}

func main() {
	flag.Usage = usage

	for _, arg := range os.Args[1:] {
		if arg == "-h" || arg == "--help" {
			flag.Usage()
			os.Exit(0)
		}
	}

	flag.Parse()

	if err := validateFlags(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		flag.Usage()
		os.Exit(1)
	}

	operation, err := validatePositionalArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
		flag.Usage()
		os.Exit(1)
	}

	resource, err := loadAndUnmarshalResource(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading resource: %v\n", err)
		os.Exit(1)
	}

	obj, ok := resource.(client.Object)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: resource is not a client.Object\n")
		os.Exit(1)
	}
	if obj.GetNamespace() == "" {
		obj.SetNamespace(metav1.NamespaceDefault)
	}

	if *refFiles != "" {
		fmt.Printf("=== Loading Reference Files ===\n")
		fmt.Printf("Reference Files: %s\n\n", *refFiles)
		err = loadReferenceFiles(*refFiles)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading reference files: %v\n", err)
			os.Exit(1)
		}
	}

	c := clientutil.NewClient(&refStoreReader{store: refStore}, obj.GetNamespace())

	fmt.Printf("=== Debug Tool Configuration ===\n")
	fmt.Printf("Address: %s\n", *address)
	fmt.Printf("Username: %s\n", *username)
	fmt.Printf("Password: %s\n", "[REDACTED]")
	fmt.Printf("Resource File: %s\n", *file)
	fmt.Printf("Provider: %s\n", *providerName)
	fmt.Printf("Operation: %s\n", operation)
	fmt.Printf("\n=== Resource Information ===\n")
	printResourceInfo(resource)

	fn, err := provider.Get(*providerName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting provider: %v\n", err)
		os.Exit(1)
	}

	prov := fn()

	conn := &deviceutil.Connection{
		Address:  *address,
		Username: *username,
		Password: *password,
		// #nosec 204
		TLS: &tls.Config{InsecureSkipVerify: true}, // For testing purposes only
	}

	ctx, cancel := context.WithCancel(context.Background())

	ch := make(chan os.Signal, 2)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		cancel()
		<-ch
		os.Exit(1)
	}()

	err = prov.Connect(ctx, conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to provider: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if disconnectErr := prov.Disconnect(ctx, conn); disconnectErr != nil {
			fmt.Fprintf(os.Stderr, "Error disconnecting from provider: %v\n", disconnectErr)
			os.Exit(1)
		}
	}()

	fmt.Printf("\n=== Operation Status ===\n")
	err = performOperation(ctx, prov, obj, operation, c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error performing operation: %v\n", err)
		return
	}

	fmt.Printf("Provider tool completed successfully.\n")
}

func performOperation(ctx context.Context, prov provider.Provider, obj client.Object, operation string, c *clientutil.Client) error {
	switch operation {
	case "create":
		return performCreate(ctx, prov, obj, c)
	case "delete":
		return performDelete(ctx, prov, obj)
	default:
		return fmt.Errorf("unknown operation: %s", operation)
	}
}

func performCreate(ctx context.Context, prov provider.Provider, obj client.Object, c *clientutil.Client) error {
	switch res := obj.(type) {
	case *v1alpha1.AccessControlList:
		ap, ok := prov.(provider.ACLProvider)
		if !ok {
			return errors.New("provider does not implement ACLProvider")
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return ap.EnsureACL(ctx, &provider.EnsureACLRequest{
			ACL:            res,
			ProviderConfig: cfg,
		})

	case *v1alpha1.Banner:
		bp, ok := prov.(provider.BannerProvider)
		if !ok {
			return errors.New("provider does not implement BannerProvider")
		}
		if res.Spec.Message.Inline == nil {
			return errors.New("banner message from Secret/ConfigMap is not supported")
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return bp.EnsureBanner(ctx, &provider.EnsureBannerRequest{
			Message:        *res.Spec.Message.Inline,
			Type:           res.Spec.Type,
			ProviderConfig: cfg,
		})

	case *v1alpha1.BGP:
		bp, ok := prov.(provider.BGPProvider)
		if !ok {
			return errors.New("provider does not implement BGPProvider")
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return bp.EnsureBGP(ctx, &provider.EnsureBGPRequest{
			BGP:            res,
			ProviderConfig: cfg,
		})

	case *v1alpha1.BGPPeer:
		bpp, ok := prov.(provider.BGPPeerProvider)
		if !ok {
			return errors.New("provider does not implement BGPPeerProvider")
		}

		sourceInterface := ""
		if res.Spec.LocalAddress != nil && res.Spec.LocalAddress.InterfaceRef.Name != "" {
			if len(refStore) == 0 {
				return errors.New("bgppeer resource references interface but no reference files provided (use --ref-files)")
			}
			obj := refStore.Get(res.Spec.LocalAddress.InterfaceRef.Name, res.Namespace)
			if obj == nil {
				return fmt.Errorf("referenced interface %s not found in reference files", res.Spec.LocalAddress.InterfaceRef.Name)
			}
			iface, ok := obj.(*v1alpha1.Interface)
			if !ok {
				return fmt.Errorf("referenced resource %s is not an Interface", res.Spec.LocalAddress.InterfaceRef.Name)
			}
			sourceInterface = iface.Spec.Name
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return bpp.EnsureBGPPeer(ctx, &provider.EnsureBGPPeerRequest{
			BGPPeer:         res,
			SourceInterface: sourceInterface,
			ProviderConfig:  cfg,
		})

	case *v1alpha1.Certificate:
		cp, ok := prov.(provider.CertificateProvider)
		if !ok {
			return errors.New("provider does not implement CertificateProvider")
		}

		cert, err := c.Certificate(ctx, &res.Spec.SecretRef)
		if err != nil {
			return err
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return cp.EnsureCertificate(ctx, &provider.EnsureCertificateRequest{
			ID:             res.Spec.ID,
			Certificate:    cert,
			ProviderConfig: cfg,
		})

	case *v1alpha1.DNS:
		dp, ok := prov.(provider.DNSProvider)
		if !ok {
			return errors.New("provider does not implement DNSProvider")
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return dp.EnsureDNS(ctx, &provider.EnsureDNSRequest{
			DNS:            res,
			ProviderConfig: cfg,
		})

	case *v1alpha1.EVPNInstance:
		ep, ok := prov.(provider.EVPNInstanceProvider)
		if !ok {
			return errors.New("provider does not implement EVPNInstanceProvider")
		}

		var vlan *v1alpha1.VLAN
		if res.Spec.VLANRef != nil && res.Spec.VLANRef.Name != "" {
			if len(refStore) == 0 {
				return errors.New("evpninstance resource references vlan but no reference files provided (use --ref-files)")
			}
			obj := refStore.Get(res.Spec.VLANRef.Name, res.Namespace)
			if obj == nil {
				return fmt.Errorf("referenced vlan %s not found in reference files", res.Spec.VLANRef.Name)
			}
			v, ok := obj.(*v1alpha1.VLAN)
			if !ok {
				return fmt.Errorf("referenced resource %s is not a VLAN", res.Spec.VLANRef.Name)
			}
			vlan = v
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return ep.EnsureEVPNInstance(ctx, &provider.EVPNInstanceRequest{
			EVPNInstance:   res,
			VLAN:           vlan,
			ProviderConfig: cfg,
		})

	case *v1alpha1.Interface:
		ip, ok := prov.(provider.InterfaceProvider)
		if !ok {
			return errors.New("provider does not implement InterfaceProvider")
		}

		var vlan *v1alpha1.VLAN
		if res.Spec.VlanRef != nil && res.Spec.VlanRef.Name != "" {
			if len(refStore) == 0 {
				return errors.New("interface resource references vlan but no reference files provided (use --ref-files)")
			}
			obj := refStore.Get(res.Spec.VlanRef.Name, res.Namespace)
			if obj == nil {
				return fmt.Errorf("referenced vlan %s not found in reference files", res.Spec.VlanRef.Name)
			}
			v, ok := obj.(*v1alpha1.VLAN)
			if !ok {
				return fmt.Errorf("referenced resource %s is not a VLAN", res.Spec.VlanRef.Name)
			}
			vlan = v
		}

		var vrf *v1alpha1.VRF
		if res.Spec.VrfRef != nil && res.Spec.VrfRef.Name != "" {
			if len(refStore) == 0 {
				return errors.New("interface resource references vrf but no reference files provided (use --ref-files)")
			}
			obj := refStore.Get(res.Spec.VrfRef.Name, res.Namespace)
			if obj == nil {
				return fmt.Errorf("referenced vrf %s not found in reference files", res.Spec.VrfRef.Name)
			}
			v, ok := obj.(*v1alpha1.VRF)
			if !ok {
				return fmt.Errorf("referenced resource %s is not a VRF", res.Spec.VrfRef.Name)
			}
			vrf = v
		}

		var ipv4 provider.IPv4
		if res.Spec.IPv4 != nil {
			if res.Spec.IPv4.Unnumbered != nil && res.Spec.IPv4.Unnumbered.InterfaceRef.Name != "" {
				if len(refStore) == 0 {
					return errors.New("interface resource references interface for unnumbered ipv4 but no reference files provided (use --ref-files)")
				}
				obj := refStore.Get(res.Spec.IPv4.Unnumbered.InterfaceRef.Name, res.Namespace)
				if obj == nil {
					return fmt.Errorf("referenced interface %s not found in reference files for unnumbered ipv4", res.Spec.IPv4.Unnumbered.InterfaceRef.Name)
				}
				intf, ok := obj.(*v1alpha1.Interface)
				if !ok {
					return fmt.Errorf("referenced resource %s is not an Interface", res.Spec.IPv4.Unnumbered.InterfaceRef.Name)
				}
				ipv4 = provider.IPv4Unnumbered{SourceInterface: intf.Spec.Name}
			} else if len(res.Spec.IPv4.Addresses) > 0 {
				addrs := make([]netip.Prefix, len(res.Spec.IPv4.Addresses))
				for i, addr := range res.Spec.IPv4.Addresses {
					addrs[i] = addr.Prefix
				}
				ipv4 = provider.IPv4AddressList(addrs)
			}
		}

		var members []*v1alpha1.Interface
		if res.Spec.Type == v1alpha1.InterfaceTypeAggregate && len(res.Spec.Aggregation.MemberInterfaceRefs) > 0 {
			if len(refStore) == 0 {
				return errors.New("interface resource references member interfaces but no reference files provided (use --ref-files)")
			}
			for _, ref := range res.Spec.Aggregation.MemberInterfaceRefs {
				obj := refStore.Get(ref.Name, res.Namespace)
				if obj == nil {
					return fmt.Errorf("referenced member interface %s not found in reference files", ref.Name)
				}
				member, ok := obj.(*v1alpha1.Interface)
				if !ok {
					return fmt.Errorf("referenced resource %s is not an Interface", ref.Name)
				}
				members = append(members, member)
			}
		}

		var multiChassisID *int16
		if res.Spec.Aggregation != nil && res.Spec.Aggregation.MultiChassis != nil {
			multiChassisID = &res.Spec.Aggregation.MultiChassis.ID
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return ip.EnsureInterface(ctx, &provider.EnsureInterfaceRequest{
			Interface:      res,
			IPv4:           ipv4,
			Members:        members,
			MultiChassisID: multiChassisID,
			VLAN:           vlan,
			VRF:            vrf,
			ProviderConfig: cfg,
		})

	case *v1alpha1.ISIS:
		ip, ok := prov.(provider.ISISProvider)
		if !ok {
			return errors.New("provider does not implement ISISProvider")
		}

		if len(refStore) == 0 {
			return fmt.Errorf("isis resource references %d interfaces but no reference files provided (use --ref-files)", len(res.Spec.InterfaceRefs))
		}

		var interfaces []*v1alpha1.Interface
		for _, ref := range res.Spec.InterfaceRefs {
			obj := refStore.Get(ref.Name, res.Namespace)
			if obj == nil {
				return fmt.Errorf("referenced interface %s not found in reference files", ref.Name)
			}
			intf, ok := obj.(*v1alpha1.Interface)
			if !ok {
				return fmt.Errorf("referenced resource %s is not an Interface", ref.Name)
			}
			interfaces = append(interfaces, intf)
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return ip.EnsureISIS(ctx, &provider.EnsureISISRequest{
			ISIS:           res,
			Interfaces:     interfaces,
			ProviderConfig: cfg,
		})

	case *v1alpha1.ManagementAccess:
		map_, ok := prov.(provider.ManagementAccessProvider)
		if !ok {
			return errors.New("provider does not implement ManagementAccessProvider")
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return map_.EnsureManagementAccess(ctx, &provider.EnsureManagementAccessRequest{
			ManagementAccess: res,
			ProviderConfig:   cfg,
		})

	case *v1alpha1.NTP:
		np, ok := prov.(provider.NTPProvider)
		if !ok {
			return errors.New("provider does not implement NTPProvider")
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return np.EnsureNTP(ctx, &provider.EnsureNTPRequest{
			NTP:            res,
			ProviderConfig: cfg,
		})

	case *v1alpha1.OSPF:
		op, ok := prov.(provider.OSPFProvider)
		if !ok {
			return errors.New("provider does not implement OSPFProvider")
		}

		if len(refStore) == 0 {
			return fmt.Errorf("ospf resource references %d interfaces but no reference files provided (use --ref-files)", len(res.Spec.InterfaceRefs))
		}

		var interfaces []provider.OSPFInterface
		for _, ref := range res.Spec.InterfaceRefs {
			obj := refStore.Get(ref.Name, res.Namespace)
			if obj == nil {
				return fmt.Errorf("referenced interface %s not found in reference files", ref.Name)
			}
			iface, ok := obj.(*v1alpha1.Interface)
			if !ok {
				return fmt.Errorf("referenced resource %s is not an Interface", ref.Name)
			}
			interfaces = append(interfaces, provider.OSPFInterface{
				Interface: iface,
				Area:      ref.Area,
				Passive:   ref.Passive,
			})
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return op.EnsureOSPF(ctx, &provider.EnsureOSPFRequest{
			OSPF:           res,
			Interfaces:     interfaces,
			ProviderConfig: cfg,
		})

	case *v1alpha1.PIM:
		pp, ok := prov.(provider.PIMProvider)
		if !ok {
			return errors.New("provider does not implement PIMProvider")
		}

		if len(refStore) == 0 {
			return fmt.Errorf("pim resource references %d interfaces but no reference files provided (use --ref-files)", len(res.Spec.InterfaceRefs))
		}

		var interfaces []provider.PIMInterface
		for _, ref := range res.Spec.InterfaceRefs {
			obj := refStore.Get(ref.Name, res.Namespace)
			if obj == nil {
				return fmt.Errorf("referenced interface %s not found in reference files", ref.Name)
			}
			iface, ok := obj.(*v1alpha1.Interface)
			if !ok {
				return fmt.Errorf("referenced resource %s is not an Interface", ref.Name)
			}
			interfaces = append(interfaces, provider.PIMInterface{
				Interface: iface,
				Mode:      ref.Mode,
			})
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return pp.EnsurePIM(ctx, &provider.EnsurePIMRequest{
			PIM:            res,
			Interfaces:     interfaces,
			ProviderConfig: cfg,
		})

	case *v1alpha1.PrefixSet:
		psp, ok := prov.(provider.PrefixSetProvider)
		if !ok {
			return errors.New("provider does not implement PrefixSetProvider")
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return psp.EnsurePrefixSet(ctx, &provider.PrefixSetRequest{
			PrefixSet:      res,
			ProviderConfig: cfg,
		})

	case *v1alpha1.RoutingPolicy:
		rpp, ok := prov.(provider.RoutingPolicyProvider)
		if !ok {
			return errors.New("provider does not implement RoutingPolicyProvider")
		}

		var statements []provider.PolicyStatement
		for _, stmt := range res.Spec.Statements {
			providerStmt := provider.PolicyStatement{
				Sequence: stmt.Sequence,
				Actions:  stmt.Actions,
			}

			if stmt.Conditions != nil && stmt.Conditions.MatchPrefixSet != nil {
				prefixSetName := stmt.Conditions.MatchPrefixSet.PrefixSetRef.Name
				if len(refStore) == 0 {
					return fmt.Errorf("routingpolicy statement %d references prefixset but no reference files provided (use --ref-files)", stmt.Sequence)
				}
				obj := refStore.Get(prefixSetName, res.Namespace)
				if obj == nil {
					return fmt.Errorf("referenced prefixset %s not found in reference files for statement %d", prefixSetName, stmt.Sequence)
				}
				ps, ok := obj.(*v1alpha1.PrefixSet)
				if !ok {
					return fmt.Errorf("referenced resource %s is not a PrefixSet for statement %d", prefixSetName, stmt.Sequence)
				}
				providerStmt.Conditions = []provider.PolicyCondition{
					provider.MatchPrefixSetCondition{PrefixSet: ps},
				}
			}

			statements = append(statements, providerStmt)
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return rpp.EnsureRoutingPolicy(ctx, &provider.EnsureRoutingPolicyRequest{
			Name:           res.Spec.Name,
			Statements:     statements,
			ProviderConfig: cfg,
		})

	case *v1alpha1.SNMP:
		sp, ok := prov.(provider.SNMPProvider)
		if !ok {
			return errors.New("provider does not implement SNMPProvider")
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return sp.EnsureSNMP(ctx, &provider.EnsureSNMPRequest{
			SNMP:           res,
			ProviderConfig: cfg,
		})

	case *v1alpha1.Syslog:
		slp, ok := prov.(provider.SyslogProvider)
		if !ok {
			return errors.New("provider does not implement SyslogProvider")
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return slp.EnsureSyslog(ctx, &provider.EnsureSyslogRequest{
			Syslog:         res,
			ProviderConfig: cfg,
		})

	case *v1alpha1.User:
		up, ok := prov.(provider.UserProvider)
		if !ok {
			return errors.New("provider does not implement UserProvider")
		}

		pwd, err := c.Secret(ctx, &res.Spec.Password.SecretKeyRef)
		if err != nil {
			return err
		}

		var sshKey string
		if res.Spec.SSHPublicKey != nil {
			sshKeyBytes, err := c.Secret(ctx, &res.Spec.SSHPublicKey.SecretKeyRef)
			if err != nil {
				return err
			}
			sshKey = string(sshKeyBytes)
		}

		roles := make([]string, len(res.Spec.Roles))
		for i, role := range res.Spec.Roles {
			roles[i] = role.Name
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return up.EnsureUser(ctx, &provider.EnsureUserRequest{
			Username:       res.Spec.Username,
			Password:       string(pwd),
			SSHKey:         sshKey,
			Roles:          roles,
			ProviderConfig: cfg,
		})

	case *v1alpha1.VLAN:
		vp, ok := prov.(provider.VLANProvider)
		if !ok {
			return errors.New("provider does not implement VLANProvider")
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return vp.EnsureVLAN(ctx, &provider.VLANRequest{
			VLAN:           res,
			ProviderConfig: cfg,
		})

	case *v1alpha1.VRF:
		vp, ok := prov.(provider.VRFProvider)
		if !ok {
			return errors.New("provider does not implement VRFProvider")
		}

		var cfg *provider.ProviderConfig
		if res.Spec.ProviderConfigRef != nil {
			var err error
			cfg, err = provider.GetProviderConfig(ctx, c, c.DefaultNamespace, res.Spec.ProviderConfigRef)
			if err != nil {
				return err
			}
		}

		return vp.EnsureVRF(ctx, &provider.VRFRequest{
			VRF:            res,
			ProviderConfig: cfg,
		})

	default:
		return fmt.Errorf("unsupported resource type: %T", res)
	}
}

func performDelete(ctx context.Context, prov provider.Provider, obj client.Object) error {
	switch resource := obj.(type) {
	case *v1alpha1.AccessControlList:
		ap, ok := prov.(provider.ACLProvider)
		if !ok {
			return errors.New("provider does not implement ACLProvider")
		}
		return ap.DeleteACL(ctx, &provider.DeleteACLRequest{
			Name: resource.Spec.Name,
		})

	case *v1alpha1.Banner:
		bp, ok := prov.(provider.BannerProvider)
		if !ok {
			return errors.New("provider does not implement BannerProvider")
		}
		return bp.DeleteBanner(ctx, &provider.DeleteBannerRequest{
			Type: resource.Spec.Type,
		})

	case *v1alpha1.BGP:
		bp, ok := prov.(provider.BGPProvider)
		if !ok {
			return errors.New("provider does not implement BGPProvider")
		}
		return bp.DeleteBGP(ctx, &provider.DeleteBGPRequest{
			BGP: resource,
		})

	case *v1alpha1.BGPPeer:
		bpp, ok := prov.(provider.BGPPeerProvider)
		if !ok {
			return errors.New("provider does not implement BGPPeerProvider")
		}
		return bpp.DeleteBGPPeer(ctx, &provider.DeleteBGPPeerRequest{
			BGPPeer: resource,
		})

	case *v1alpha1.Certificate:
		cp, ok := prov.(provider.CertificateProvider)
		if !ok {
			return errors.New("provider does not implement CertificateProvider")
		}
		return cp.DeleteCertificate(ctx, &provider.DeleteCertificateRequest{
			ID: resource.Spec.ID,
		})

	case *v1alpha1.DNS:
		dp, ok := prov.(provider.DNSProvider)
		if !ok {
			return errors.New("provider does not implement DNSProvider")
		}
		return dp.DeleteDNS(ctx)

	case *v1alpha1.EVPNInstance:
		ep, ok := prov.(provider.EVPNInstanceProvider)
		if !ok {
			return errors.New("provider does not implement EVPNInstanceProvider")
		}

		var vlan *v1alpha1.VLAN
		if resource.Spec.VLANRef != nil && resource.Spec.VLANRef.Name != "" {
			if len(refStore) == 0 {
				return errors.New("evpninstance resource references vlan but no reference files provided (use --ref-files)")
			}
			obj := refStore.Get(resource.Spec.VLANRef.Name, resource.Namespace)
			if obj == nil {
				return fmt.Errorf("referenced vlan %s not found in reference files", resource.Spec.VLANRef.Name)
			}
			v, ok := obj.(*v1alpha1.VLAN)
			if !ok {
				return fmt.Errorf("referenced resource %s is not a VLAN", resource.Spec.VLANRef.Name)
			}
			vlan = v
		}

		return ep.DeleteEVPNInstance(ctx, &provider.EVPNInstanceRequest{
			EVPNInstance: resource,
			VLAN:         vlan,
		})

	case *v1alpha1.Interface:
		ip, ok := prov.(provider.InterfaceProvider)
		if !ok {
			return errors.New("provider does not implement InterfaceProvider")
		}
		return ip.DeleteInterface(ctx, &provider.InterfaceRequest{
			Interface: resource,
		})

	case *v1alpha1.ISIS:
		ip, ok := prov.(provider.ISISProvider)
		if !ok {
			return errors.New("provider does not implement ISISProvider")
		}
		return ip.DeleteISIS(ctx, &provider.DeleteISISRequest{
			ISIS: resource,
		})

	case *v1alpha1.ManagementAccess:
		ma, ok := prov.(provider.ManagementAccessProvider)
		if !ok {
			return errors.New("provider does not implement ManagementAccessProvider")
		}
		return ma.DeleteManagementAccess(ctx)

	case *v1alpha1.NTP:
		np, ok := prov.(provider.NTPProvider)
		if !ok {
			return errors.New("provider does not implement NTPProvider")
		}
		return np.DeleteNTP(ctx)

	case *v1alpha1.OSPF:
		op, ok := prov.(provider.OSPFProvider)
		if !ok {
			return errors.New("provider does not implement OSPFProvider")
		}
		return op.DeleteOSPF(ctx, &provider.DeleteOSPFRequest{
			OSPF: resource,
		})

	case *v1alpha1.PIM:
		pp, ok := prov.(provider.PIMProvider)
		if !ok {
			return errors.New("provider does not implement PIMProvider")
		}
		return pp.DeletePIM(ctx, &provider.DeletePIMRequest{
			PIM: resource,
		})

	case *v1alpha1.PrefixSet:
		psp, ok := prov.(provider.PrefixSetProvider)
		if !ok {
			return errors.New("provider does not implement PrefixSetProvider")
		}
		return psp.DeletePrefixSet(ctx, &provider.PrefixSetRequest{
			PrefixSet: resource,
		})

	case *v1alpha1.RoutingPolicy:
		rpp, ok := prov.(provider.RoutingPolicyProvider)
		if !ok {
			return errors.New("provider does not implement RoutingPolicyProvider")
		}
		return rpp.DeleteRoutingPolicy(ctx, &provider.DeleteRoutingPolicyRequest{
			Name: resource.Spec.Name,
		})

	case *v1alpha1.SNMP:
		sp, ok := prov.(provider.SNMPProvider)
		if !ok {
			return errors.New("provider does not implement SNMPProvider")
		}
		return sp.DeleteSNMP(ctx, &provider.DeleteSNMPRequest{})

	case *v1alpha1.Syslog:
		slp, ok := prov.(provider.SyslogProvider)
		if !ok {
			return errors.New("provider does not implement SyslogProvider")
		}
		return slp.DeleteSyslog(ctx)

	case *v1alpha1.User:
		up, ok := prov.(provider.UserProvider)
		if !ok {
			return errors.New("provider does not implement UserProvider")
		}
		return up.DeleteUser(ctx, &provider.DeleteUserRequest{
			Username: resource.Spec.Username,
		})

	case *v1alpha1.VLAN:
		vp, ok := prov.(provider.VLANProvider)
		if !ok {
			return errors.New("provider does not implement VLANProvider")
		}
		return vp.DeleteVLAN(ctx, &provider.VLANRequest{
			VLAN: resource,
		})

	case *v1alpha1.VRF:
		vp, ok := prov.(provider.VRFProvider)
		if !ok {
			return errors.New("provider does not implement VRFProvider")
		}
		return vp.DeleteVRF(ctx, &provider.VRFRequest{
			VRF: resource,
		})

	default:
		return fmt.Errorf("unsupported resource type: %T", resource)
	}
}
