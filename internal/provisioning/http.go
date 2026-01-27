// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package provisioning

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/clientutil"
	"github.com/ironcore-dev/network-operator/internal/conditions"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

func getClientIP(r *http.Request) (string, error) {
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
		return "", fmt.Errorf("Failed to parse remote address: %w", err)
	}
	return ip, nil
}

func getBearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("authorization header is missing")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", errors.New("invalid authorization header format")
	}

	return parts[1], nil
}

func (s *HTTPServer) findDeviceAndValidateToken(ctx context.Context, serial, token string) (*v1alpha1.Device, *v1alpha1.ProvisioningInfo, int, error) {
	device, err := deviceutil.GetDeviceBySerial(ctx, s.Client, "", serial)
	if err != nil {
		s.Logger.Error(err, "Failed to get device by serial", "serial", serial, "error", err)
		return nil, nil, http.StatusInternalServerError, fmt.Errorf("Failed to find device by serial: %w", err)
	}

	act := device.GetActiveProvisioning()

	if act == nil {
		s.Logger.Error(nil, "No active provisioning found for device", "device", device.Name)
		return nil, nil, http.StatusPreconditionFailed, fmt.Errorf("no active provisioning found for device: %s", device.Name)
	}
	if act.Token != token {
		return nil, nil, http.StatusUnauthorized, errors.New("unauthorized: invalid token")
	}
	return device, act, http.StatusOK, nil
}

type HTTPServer struct {
	Client           client.Client
	Logger           klog.Logger
	Mux              *http.ServeMux
	Recorder         record.EventRecorder
	ValidateSourceIP bool
	Provider         provider.ProvisioningProvider
}

func (s *HTTPServer) Start(port int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/provisioning/status-report", s.HandleStatusReport)
	mux.HandleFunc("/provisioning/config", s.HandleProvisioningRequest)
	mux.HandleFunc("/provisioning/device-certificate", s.GetDeviceCertificate)
	mux.HandleFunc("/provisioning/mtls-client-ca", s.GetMTLSClientCA)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	s.Logger.Info("Starting provisioning server", "port", port)

	err := httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

type StatusReport struct {
	Status v1alpha1.ProvisioningPhase `json:"status"`
	Detail string                     `json:"detail,omitempty"`
}

func (r *StatusReport) ToCondition() metav1.Condition {
	condition := metav1.ConditionFalse
	if r.Succeeded() {
		condition = metav1.ConditionTrue
	}
	return metav1.Condition{
		Type:    v1alpha1.ReadyCondition,
		Status:  condition,
		Reason:  string(r.Status),
		Message: r.Detail,
	}
}

func (r *StatusReport) Failed() bool {
	switch r.Status {
	case v1alpha1.ProvisioningScriptExecutionFailed,
		v1alpha1.ProvisioningUpgradeFailed,
		v1alpha1.ProvisioningImageDownloadFailed:
		return true
	default:
		return false
	}
}

func (r *StatusReport) Succeeded() bool {
	switch r.Status {
	case v1alpha1.ProvisioningExecutionFinishedWithoutReboot,
		v1alpha1.ProvisioningRebootingDevice:
		return true
	default:
		return false
	}
}

func (s *HTTPServer) HandleStatusReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	serial := r.URL.Query().Get("serial")
	if serial == "" {
		http.Error(w, "Serial parameter is required", http.StatusBadRequest)
		return
	}

	token, err := getBearerToken(r)
	if err != nil {
		http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
		return
	}

	var report StatusReport
	if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
		clientIP, ipErr := getClientIP(r)
		if ipErr != nil {
			clientIP = "unknown"
		}
		s.Logger.Error(err, "Invalid status report body", "IP", clientIP)
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	device, act, statusCode, err := s.findDeviceAndValidateToken(ctx, serial, token)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}
	conditions.Set(device, report.ToCondition())

	if report.Succeeded() {
		device.Status.Phase = v1alpha1.DevicePhaseProvisioned
		if report.Status == v1alpha1.ProvisioningRebootingDevice {
			act.RebootTime = metav1.Now()
		}
	}

	if report.Failed() {
		device.Status.Phase = v1alpha1.DevicePhaseFailed
		act.EndTime = metav1.Now()
		act.Error = report.Detail
	}

	s.Recorder.Eventf(device, "Normal", "Provisioning", "%s: %s", report.Status, report.Detail)

	if err := s.Client.Status().Update(ctx, device); err != nil {
		s.Logger.Error(err, "Failed to update device status", "device", device.Name)
		http.Error(w, "Failed to persist device status", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)

	if _, err := w.Write([]byte("OK")); err != nil {
		s.Logger.Error(err, "Failed to write response")
	}
}

type ProvisioningResponse struct {
	ProvisioningToken string         `json:"provisioningToken"`
	Image             v1alpha1.Image `json:"image"`
	UserAccounts      []UserAccount  `json:"userAccounts"`
	Hostname          string         `json:"hostname"`
}

type UserAccount struct {
	Username       string `json:"username"`
	HashedPassword string `json:"hashedPassword"`
	HashAlgorithm  string `json:"hashAlgorithm"`
}

func (s *HTTPServer) HandleProvisioningRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	serial := r.URL.Query().Get("serial")
	if serial == "" {
		http.Error(w, "Serial parameter is required", http.StatusBadRequest)
		return
	}

	s.Logger.Info("provisioning request received", "serial", serial)
	device, err := deviceutil.GetDeviceBySerial(ctx, s.Client, "", serial)
	if err != nil {
		s.Logger.Error(err, "Failed to find device by serial", "serial", serial, "error", err)
		http.Error(w, "Failed to find device by serial", http.StatusInternalServerError)
		return
	}

	if s.ValidateSourceIP {
		clientIP, err := getClientIP(r)
		if err != nil {
			s.Logger.Error(err, "Failed to get client IP for validation")
			http.Error(w, "Failed to determine client IP", http.StatusBadRequest)
			return
		}

		deviceIP := strings.Split(device.Spec.Endpoint.Address, ":")[0]
		if deviceIP != clientIP {
			s.Logger.Error(nil, "Source IP validation failed", "clientIP", clientIP, "deviceIP", deviceIP)
			http.Error(w, "Source IP does not match device IP", http.StatusForbidden)
			return
		}
		s.Logger.Info("Source IP validation passed", "clientIP", clientIP, "device", device.Name)
	}

	act := device.GetActiveProvisioning()
	if act == nil {
		act, err = device.CreateProvisioningEntry()
		if err != nil {
			s.Logger.Error(err, "Failed to create provisioning entry", "device", device.Name)
			http.Error(w, "Failed to create provisioning entry", http.StatusPreconditionRequired)
			return
		}
		if err := s.Client.Status().Update(ctx, device); err != nil {
			s.Logger.Error(err, "Failed to update device status", "device", device.Name)
			http.Error(w, "Failed to update device status", http.StatusInternalServerError)
			return
		}
	}

	conn, err := deviceutil.GetDeviceConnection(ctx, s.Client, device)
	if err != nil {
		s.Logger.Error(err, "Failed to get user accounts", "device", device.Name)
		http.Error(w, "Failed to get user accounts", http.StatusInternalServerError)
		return
	}

	hashedPassword, hashAlgorithm, err := s.Provider.HashProvisioningPassword(conn.Password)
	if err != nil {
		s.Logger.Error(err, "Failed to hash provisioning password", "device", device.Name)
		http.Error(w, "Failed to hash provisioning password", http.StatusInternalServerError)
		return
	}

	ua := UserAccount{
		Username:       conn.Username,
		HashedPassword: hashedPassword,
		HashAlgorithm:  hashAlgorithm,
	}

	response := ProvisioningResponse{
		ProvisioningToken: act.Token,
		Image:             device.Spec.Provisioning.Image,
		UserAccounts:      []UserAccount{ua},
		Hostname:          device.Name,
	}

	content, err := json.Marshal(response)
	if err != nil {
		s.Logger.Error(err, "Failed to marshal provisioning response", "device", device.Name)
		http.Error(w, "Failed to marshal provisioning response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(content); err != nil {
		s.Logger.Error(err, "Failed to write response")
	}
}

func (s *HTTPServer) GetMTLSClientCA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token, err := getBearerToken(r)
	if err != nil {
		http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
		return
	}

	serial := r.URL.Query().Get("serial")
	if serial == "" {
		http.Error(w, "Serial parameter is required", http.StatusBadRequest)
		return
	}

	device, _, statusCode, err := s.findDeviceAndValidateToken(ctx, serial, token)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	c := clientutil.NewClient(s.Client, device.Namespace)

	// The CA the operator uses to connect to the device, should be trusted by the device for MTLS
	if device.Spec.Endpoint.TLS == nil {
		http.Error(w, "Device has no TLS configuration", http.StatusNotFound)
		return
	}

	if device.Spec.Endpoint.TLS.Certificate == nil || device.Spec.Endpoint.TLS.Certificate.SecretRef.Name == "" {
		http.Error(w, "Device has no MTLS certificate configured", http.StatusNotFound)
		return
	}
	namespace := device.Namespace
	if device.Spec.Endpoint.SecretRef != nil && device.Spec.Endpoint.SecretRef.Namespace != "" {
		namespace = device.Spec.Endpoint.SecretRef.Namespace
	}
	certRef := client.ObjectKey{Name: device.Spec.Endpoint.TLS.Certificate.SecretRef.Name, Namespace: namespace}
	certSecret := corev1.Secret{}
	err = c.Get(ctx, certRef, &certSecret)
	if err != nil || certSecret.Data == nil {
		s.Logger.Error(err, "Failed to get device certificate secret", "device", device.Name)
		http.Error(w, "Failed to get device certificate secret", http.StatusNotFound)
		return
	}

	if certSecret.Data["ca.crt"] == nil {
		s.Logger.Error(nil, "CA certificate not found in secret", "device", device.Name)
		http.Error(w, "CA certificate not found", http.StatusInternalServerError)
		return
	}

	b64val := certSecret.Data["ca.crt"]
	operatorCA, err := base64.StdEncoding.DecodeString(string(b64val))
	if err != nil {
		s.Logger.Error(err, "Failed to decode CA certificate", "device", device.Name)
		http.Error(w, "Failed to decode CA certificate", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(operatorCA); err != nil {
		s.Logger.Error(err, "Failed to write response")
	}
}

type DeviceCertificateResponse struct {
	Certificate   []byte `json:"certificate"`
	PrivateKey    []byte `json:"privateKey"`
	CACertificate []byte `json:"caCertificate"`
}

func (s *HTTPServer) GetDeviceCertificate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token, err := getBearerToken(r)
	if err != nil {
		http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
		return
	}

	serial := r.URL.Query().Get("serial")
	if serial == "" {
		http.Error(w, "Serial parameter is required", http.StatusBadRequest)
		return
	}

	device, _, statusCode, err := s.findDeviceAndValidateToken(ctx, serial, token)
	if err != nil {
		http.Error(w, err.Error(), statusCode)
		return
	}

	c := clientutil.NewClient(s.Client, device.Namespace)
	certList := v1alpha1.CertificateList{}

	if err = c.List(ctx, &certList, client.InNamespace(device.Namespace), client.MatchingLabels{v1alpha1.DeviceLabel: device.Name}); err != nil {
		s.Logger.Error(err, "Failed to list certificates", "device", device.Name)
		http.Error(w, "Failed to list certificates", http.StatusInternalServerError)
		return
	}

	if len(certList.Items) == 0 {
		http.Error(w, "No certificate found for device", http.StatusNotFound)
		return
	}

	if len(certList.Items) > 1 {
		s.Logger.Error(nil, "Multiple certificates found for device", "device", device.Name)
		http.Error(w, "Multiple certificates found for device", http.StatusInternalServerError)
		return
	}
	certSecret := corev1.Secret{}
	certRef := client.ObjectKey{Name: certList.Items[0].Spec.SecretRef.Name, Namespace: device.Namespace}
	err = c.Get(ctx, certRef, &certSecret)
	if err != nil {
		s.Logger.Error(err, "Failed to get certificate secret", "device", device.Name)
		http.Error(w, "Failed to get certificate secret", http.StatusInternalServerError)
		return
	}
	response := DeviceCertificateResponse{}
	if b64val, ok := certSecret.Data["tls.crt"]; ok {
		certificate, err := base64.StdEncoding.DecodeString(string(b64val))
		if err != nil {
			s.Logger.Error(err, "Failed to decode certificate", "device", device.Name)
			http.Error(w, "Failed to decode certificate", http.StatusInternalServerError)
			return
		}
		response.Certificate = certificate
	}
	if b64val, ok := certSecret.Data["tls.key"]; ok {
		privateKey, err := base64.StdEncoding.DecodeString(string(b64val))
		if err != nil {
			s.Logger.Error(err, "Failed to decode private key", "device", device.Name)
			http.Error(w, "Failed to decode private key", http.StatusInternalServerError)
			return
		}
		response.PrivateKey = privateKey
	}
	if b64val, ok := certSecret.Data["ca.crt"]; ok {
		caCertificate, err := base64.StdEncoding.DecodeString(string(b64val))
		if err != nil {
			s.Logger.Error(err, "Failed to decode CA certificate", "device", device.Name)
			http.Error(w, "Failed to decode CA certificate", http.StatusInternalServerError)
			return
		}
		response.CACertificate = caCertificate
	}

	if len(response.Certificate) == 0 || len(response.PrivateKey) == 0 {
		s.Logger.Error(nil, "Incomplete certificate data in secret", "device", device.Name)
		http.Error(w, "Incomplete certificate data in secret", http.StatusInternalServerError)
		return
	}

	content, err := json.Marshal(response)
	if err != nil {
		s.Logger.Error(err, "Failed to marshal device certificate response", "device", device.Name)
		http.Error(w, "Failed to marshal device certificate response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if _, err := w.Write(content); err != nil {
		s.Logger.Error(err, "Failed to write response")
	}
}
