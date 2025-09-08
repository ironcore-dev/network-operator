# -*- mode: Python -*-
# SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
# SPDX-License-Identifier: Apache-2.0

# Don't track us.
analytics_settings(False)

update_settings(k8s_upsert_timeout_secs=60)

allow_k8s_contexts(['minikube', 'kind-network'])

load('ext://cert_manager', 'deploy_cert_manager')
deploy_cert_manager(version='v1.18.2')

docker_build('controller:latest', '.', ignore=['*/*/zz_generated.deepcopy.go', 'config/crd/bases/*'], only=[
    'api/', 'cmd/', 'hack/', 'internal/', 'go.mod', 'go.sum', 'Makefile',
])

local_resource('controller-gen', 'make generate', ignore=['*/*/zz_generated.deepcopy.go', 'config/crd/bases/*'], deps=[
    'api/', 'cmd/', 'hack/', 'internal/', 'go.mod', 'go.sum', 'Makefile',
])

k8s_yaml(kustomize('config/default'))
k8s_resource('network-operator-controller-manager', resource_deps=['controller-gen'])

# Sample resources with manual trigger mode
k8s_yaml('./config/samples/v1alpha1_device.yaml')
k8s_resource(new_name='leaf1', objects=['leaf1:device', 'secret-basic-auth:secret'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

k8s_yaml('./config/samples/v1alpha1_interface.yaml')
k8s_resource(new_name='lo0', objects=['lo0:interface'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)
k8s_resource(new_name='lo1', objects=['lo1:interface'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)
k8s_resource(new_name='eth1-1', objects=['eth1-1:interface'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)
k8s_resource(new_name='eth1-2', objects=['eth1-2:interface'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)
k8s_resource(new_name='eth1-10', objects=['eth1-10:interface'], trigger_mode=TRIGGER_MODE_MANUAL, auto_init=False)

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

print('🚀 network-operator development environment')
print('👉 Edit the code inside the api/, cmd/, or internal/ directories')
print('👉 Tilt will automatically rebuild and redeploy when changes are detected')
# vim: ft=tiltfile syn=python
