---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
    name: 'Network Operator'
    text: 'Cloud Native Network Device Provisioning'
    tagline: 'Network Operator is an open-source platform designed to empower operators to automate, operate and observe multi-vendor data-center networks'
    image:
        src: https://raw.githubusercontent.com/ironcore-dev/network-operator/refs/heads/main/docs/assets/network-operator-logo.png
        alt: Network Operator
    actions:
        - theme: brand
          text: Overview
          link: /overview/
        - theme: alt
          text: API Reference
          link: /api-reference/

features:
    - title: 🔌 Multi-Vendor Network Automation
      details: Provisioning and configuration for network devices using Kubernetes-native CRDs, with pluggable providers for OpenConfig, Cisco NX-OS and many more.
    - title: 📄 Declarative Day-2 Operations
      details: Manage the entire lifecycle of network devices declaratively, from Zero Touch Provisioning to Operating System Upgrades.
    - title: 🧰 Comprehensive Resource Management
      details: Manage a wide range of network resources including Interfaces (Loopback, Ethernet, Port-Channel), Routing Protocols (BGP, IS-IS, OSPF), ACLs and device-level settings.
    - title: ☸️ Kubernetes-Native Design
      details: Built with Kubebuilder and controller-runtime for seamless integration and robust, reliable operation within your Kubernetes environment.
    - title: 📡 gNMI-Powered Communication
      details: Utilizes the gRPC Network Management Interface (gNMI) for robust, real-time configuration and streaming telemetry from network devices.
    - title: ✅ Rigorously Tested
      details: A comprehensive suite of unit, end-to-end, and lab tests ensures reliability and correctness for production environments.
---
