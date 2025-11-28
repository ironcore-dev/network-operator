# -*- mode: Python -*-
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0

# Don't track us.
analytics_settings(False)

update_settings(k8s_upsert_timeout_secs=60)

allow_k8s_contexts(['minikube', 'kind-network'])

load('ext://cert_manager', 'deploy_cert_manager')
deploy_cert_manager(version='v1.18.2')

docker_build('controller:latest', '.', ignore=['**/*/zz_generated.deepcopy.go', 'config/crd/bases/*'], only=[
    'api/', 'cmd/', 'hack/', 'internal/', 'go.mod', 'go.sum', 'Makefile',
])

local_resource('controller-gen', 'make generate', ignore=['**/*/zz_generated.deepcopy.go', 'config/crd/bases/*'], deps=[
    'api/', 'cmd/', 'hack/', 'internal/', 'go.mod', 'go.sum', 'Makefile',
])

docker_build('ghcr.io/ironcore-dev/gnmi-test-server:latest', './test/gnmi')

provider = os.getenv('PROVIDER', 'openconfig')

manager = kustomize('config/develop')
manager = str(manager).replace('--provider=openconfig', '--provider={}'.format(provider))

k8s_yaml(blob(manager))
k8s_resource('network-operator-controller-manager', resource_deps=['controller-gen'])

# Sample resources with manual trigger mode
def device_yaml():
    decoded = read_yaml_stream('./config/samples/v1alpha1_device.yaml')
    ip = str(local("docker run --rm busybox:1.37.0 nslookup -type=a host.docker.internal 2>/dev/null | grep 'Address:' | tail -n 1 | awk '{print $2}' || echo ''", quiet=True)).rstrip('\n')
    if len(ip) > 0:
        decoded[0]['spec']['endpoint']['address'] = ip+':9339'
    return encode_yaml_stream(decoded)

k8s_yaml(device_yaml())
k8s_resource(new_name='leaf1', objects=['leaf1:device', 'secret-basic-auth:secret'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_interface.yaml')
k8s_resource(new_name='lo0', objects=['lo0:interface'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)
k8s_resource(new_name='lo1', objects=['lo1:interface'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)
k8s_resource(new_name='eth1-1', objects=['eth1-1:interface'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)
k8s_resource(new_name='eth1-2', objects=['eth1-2:interface'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)
k8s_resource(new_name='eth1-10', objects=['eth1-10:interface'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)
k8s_resource(new_name='po10', objects=['po-10:interface'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)
k8s_resource(new_name='svi-10', objects=['svi-10:interface'], resource_deps=['vlan-10'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)
k8s_resource(new_name='eth1-30', objects=['eth1-30:interface'], resource_deps=['vrf-vpc-keepalive'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_banner.yaml')
k8s_resource(new_name='banner', objects=['banner:banner'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_user.yaml')
k8s_resource(new_name='user', objects=['user:user', 'user-password:secret', 'user-ssh-key:secret'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_dns.yaml')
k8s_resource(new_name='dns', objects=['dns:dns'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_ntp.yaml')
k8s_resource(new_name='ntp', objects=['ntp:ntp'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_acl.yaml')
k8s_resource(new_name='acl', objects=['acl:accesscontrollist'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_certificate.yaml')
k8s_resource(new_name='trustpoint', objects=['network-operator:issuer', 'network-operator-ca:certificate', 'trustpoint:certificate'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_snmp.yaml')
k8s_resource(new_name='snmp', objects=['snmp:snmp'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_syslog.yaml')
k8s_resource(new_name='syslog', objects=['syslog:syslog'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_managementaccess.yaml')
k8s_resource(new_name='managementaccess', objects=['managementaccess:managementaccess'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_isis.yaml')
k8s_resource(new_name='isis-underlay', objects=['underlay:isis'], resource_deps=['lo0', 'lo1', 'eth1-1', 'eth1-2'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_vrf.yaml')
k8s_resource(new_name='vrf-admin', objects=['vrf-cc-admin:vrf'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)
k8s_resource(new_name='vrf-001', objects=['vrf-cc-prod-001:vrf'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)
k8s_resource(new_name='vrf-vpc-keepalive', objects=['vpc-keepalive:vrf'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_pim.yaml')
k8s_resource(new_name='pim', objects=['pim:pim'], resource_deps=['lo0', 'lo1', 'eth1-1', 'eth1-2'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_bgp.yaml')
k8s_resource(new_name='bgp', objects=['bgp:bgp'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_bgppeer.yaml')
k8s_resource(new_name='peer-spine1', objects=['leaf1-spine1:bgppeer'], resource_deps=['bgp', 'lo0'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)
k8s_resource(new_name='peer-spine2', objects=['leaf1-spine2:bgppeer'], resource_deps=['bgp', 'lo0'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_ospf.yaml')
k8s_resource(new_name='ospf-underlay', objects=['underlay:ospf'], resource_deps=['lo0', 'lo1', 'eth1-1', 'eth1-2'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_vlan.yaml')
k8s_resource(new_name='vlan-10', objects=['vlan-10:vlan'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_evi.yaml')
k8s_resource(new_name='vxlan-100010', objects=['vxlan-100010:evpninstance'], resource_deps=['vlan-10'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

print('🚀 network-operator development environment')
print('👉 Edit the code inside the api/, cmd/, or internal/ directories')
print('👉 Tilt will automatically rebuild and redeploy when changes are detected')
# vim: ft=tiltfile syn=python
