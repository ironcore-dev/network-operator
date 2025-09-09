// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ironcore-dev/network-operator/api/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/clientutil"
)

const (
	DeviceIPLabel = "networking.cloud.sap/device-ip"
	// Future enhancement: DeviceSerialLabel = "networking.cloud.sap/device-serial"
)

type HTTPServer struct {
	client           client.Client
	port             int
	validateSourceIP bool
	logger           klog.Logger
}

func NewHTTPServer(client client.Client, port int, validateSourceIP bool) *HTTPServer {
	return &HTTPServer{
		client:           client,
		port:             port,
		validateSourceIP: validateSourceIP,
		logger:           klog.NewKlogr().WithName("bootstrap-http"),
	}
}

func (s *HTTPServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/bootstrap", s.handleBootstrap)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: mux,
	}

	s.logger.Info("Starting bootstrap server", "port", s.port, "validateSourceIP", s.validateSourceIP)

	err := httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *HTTPServer) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := s.logger.WithValues("method", r.Method, "path", r.URL.Path)

	serial := r.URL.Query().Get("serial")
	if serial == "" {
		log.Error(nil, "Serial parameter is required")
		http.Error(w, "Serial parameter is required", http.StatusBadRequest)
		return
	}

	s.logger.Info("Bootstrap request received", "serial", serial)

	var deviceList v1alpha1.DeviceList
	selector := labels.SelectorFromSet(labels.Set{v1alpha1.DeviceSerialLabel: serial})

	if err := s.client.List(ctx, &deviceList, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		s.logger.Error(err, "Failed to list devices", "serial", serial)
		http.Error(w, "Failed to list devices", http.StatusInternalServerError)
		return
	}

	s.logger.Info("Devices found", "count", len(deviceList.Items), "serial", serial)

	if len(deviceList.Items) > 1 {
		err := fmt.Errorf("multiple devices found with the same serial: %q", serial)
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	if len(deviceList.Items) == 0 {
		s.logger.Info("No device found with the given serial", "serial", serial)
		http.Error(w, "No device found with the given serial", http.StatusNotFound)
		return
	}

	device := deviceList.Items[0]
	s.logger.Info("Found device for serial", "device", deviceList.Items[0].Name, "serial", serial)

	if s.validateSourceIP {
		clientIP, err := s.getClientIP(r)
		if err != nil {
			log.Error(err, "Failed to get client IP for validation")
			http.Error(w, "Failed to determine client IP", http.StatusBadRequest)
			return
		}

		deviceIP := strings.Split(device.Spec.Endpoint.Address, ":")[0]
		if deviceIP != clientIP {
			log.Error(nil, "Source IP validation failed", "clientIP", clientIP, "deviceIP", deviceIP)
			http.Error(w, "Source IP does not match device IP", http.StatusForbidden)
			return
		}

		log.Info("Source IP validation passed", "clientIP", clientIP, "device", device.Name)
	}

	c := clientutil.NewClient(s.client, device.Namespace)
	content, err := c.Template(ctx, device.Spec.Bootstrap.Template)
	if err != nil {
		s.logger.Error(err, "Failed to render bootstrap template", "device", device.Name)
		http.Error(w, "Failed to render bootstrap template", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(content)
}

func (s *HTTPServer) getClientIP(r *http.Request) (string, error) {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// Take the first IP if there are multiple
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0]), nil
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP, nil
	}

	// Use RemoteAddr as fallback
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", fmt.Errorf("failed to parse remote address: %w", err)
	}
	return ip, nil
}
