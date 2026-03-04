// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package provisioning

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/deviceutil"
)

var (
	testDevice = &v1alpha1.Device{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-device",
			Namespace: "default",
			Labels: map[string]string{
				v1alpha1.DeviceSerialLabel: "ABC123",
			},
		},
		Spec: v1alpha1.DeviceSpec{
			Endpoint: v1alpha1.Endpoint{
				Address: "192.168.1.100:22",
				SecretRef: &v1alpha1.SecretReference{
					Name: "test-device-connection",
				},
			},
			Provisioning: &v1alpha1.Provisioning{
				Image: v1alpha1.Image{
					URL: "http://example.com/image.bin",
				},
			},
		},
		Status: v1alpha1.DeviceStatus{
			SerialNumber: "ABC123",
			Provisioning: []v1alpha1.ProvisioningInfo{
				{
					Token:     "validtoken",
					StartTime: metav1.Now(),
				},
			},
		},
	}

	testSecret = &corev1.Secret{
		Type: corev1.SecretTypeBasicAuth,
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-device-connection",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("secret123"),
		},
	}
)

type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) HashProvisioningPassword(password string) (string, string, error) { //nolint:gocritic
	return "hashedpass", "sha256", nil
}

func (p *MockProvider) VerifyProvisioned(ctx context.Context, conn *deviceutil.Connection, device *v1alpha1.Device) bool {
	return true
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name         string
		setupRequest func(*http.Request)
		expectedIP   string
	}{
		{
			name: "extract IP from X-Forwarded-For header with single IP",
			setupRequest: func(req *http.Request) {
				req.Header.Set("X-Forwarded-For", "192.168.1.100")
			},
			expectedIP: "192.168.1.100",
		},
		{
			name: "extract first IP from X-Forwarded-For header with multiple IPs",
			setupRequest: func(req *http.Request) {
				req.Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.1, 172.16.0.1")
			},
			expectedIP: "192.168.1.100",
		},
		{
			name: "extract IP from X-Real-IP header",
			setupRequest: func(req *http.Request) {
				req.Header.Set("X-Real-IP", "192.168.1.200")
			},
			expectedIP: "192.168.1.200",
		},
		{
			name: "extract IP from RemoteAddr as fallback",
			setupRequest: func(req *http.Request) {
				req.RemoteAddr = "192.168.1.50:12345"
			},
			expectedIP: "192.168.1.50",
		},
		{
			name: "prioritize X-Forwarded-For over X-Real-IP",
			setupRequest: func(req *http.Request) {
				req.Header.Set("X-Forwarded-For", "192.168.1.100")
				req.Header.Set("X-Real-IP", "192.168.1.200")
			},
			expectedIP: "192.168.1.100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			tt.setupRequest(req)

			ip, err := getClientIP(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedIP, ip)
		})
	}
}

func TestGetBearerToken(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		expectedToken string
		expectedErr   error
	}{
		{
			name:          "extract valid bearer token",
			authorization: "Bearer abc123token",
			expectedToken: "abc123token",
		},
		{
			name:          "missing authorization header",
			authorization: "",
			expectedErr:   errMissingAuthorizationHeader,
		},
		{
			name:          "invalid format without Bearer prefix",
			authorization: "abc123token",
			expectedErr:   errInvalidAuthorizationFormat,
		},
		{
			name:          "wrong auth type",
			authorization: "Basic abc123token",
			expectedErr:   errInvalidAuthorizationFormat,
		},
		{
			name:          "too many parts",
			authorization: "Bearer abc123 extra",
			expectedErr:   errInvalidAuthorizationFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}

			token, err := getBearerToken(req)
			if tt.expectedErr != nil {
				require.ErrorIs(t, err, tt.expectedErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedToken, token)
		})
	}
}

func TestHandleStatusReport(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		serial         string
		authorization  string
		body           any
		device         *v1alpha1.Device
		expectedStatus int
		expectedBody   string
		validateDevice func(*testing.T, *v1alpha1.Device)
	}{
		{
			name:           "reject non-PUT requests",
			method:         http.MethodGet,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed",
		},
		{
			name:           "reject requests without authorization header",
			method:         http.MethodPut,
			serial:         "ABC123",
			body:           StatusReport{Status: v1alpha1.ProvisioningScriptExecutionStarted},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized",
		},
		{
			name:           "reject requests with invalid JSON body",
			method:         http.MethodPut,
			authorization:  "Bearer validtoken",
			serial:         "ABC123",
			body:           "invalid json",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid JSON body",
		},
		{
			name:           "device not found",
			method:         http.MethodPut,
			authorization:  "Bearer validtoken",
			serial:         "NONEXISTENT",
			body:           StatusReport{Status: v1alpha1.ProvisioningScriptExecutionStarted},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to find device",
		},
		{
			name:          "no active provisioning found",
			method:        http.MethodPut,
			authorization: "Bearer validtoken",
			serial:        "ABC123",
			body:          StatusReport{Status: v1alpha1.ProvisioningScriptExecutionStarted},
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Status: v1alpha1.DeviceStatus{SerialNumber: "ABC123"},
			},
			expectedStatus: http.StatusPreconditionFailed,
			expectedBody:   "no active provisioning found",
		},
		{
			name:          "reject invalid token",
			method:        http.MethodPut,
			authorization: "Bearer wrongtoken",
			serial:        "ABC123",
			body:          StatusReport{Status: v1alpha1.ProvisioningDownloadingImage},
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Status: v1alpha1.DeviceStatus{
					SerialNumber: "ABC123",
					Provisioning: []v1alpha1.ProvisioningInfo{{Token: "correcttoken", StartTime: metav1.Now()}},
				},
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "unauthorized: invalid token",
		},
		{
			name:          "successfully update device status for successful provisioning",
			method:        http.MethodPut,
			authorization: "Bearer validtoken",
			serial:        "ABC123",
			body:          StatusReport{Status: v1alpha1.ProvisioningRebootingDevice, Detail: "Device is rebooting"},
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Status: v1alpha1.DeviceStatus{
					SerialNumber: "ABC123",
					Provisioning: []v1alpha1.ProvisioningInfo{{Token: "validtoken", StartTime: metav1.Now()}},
				},
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   "OK",
			validateDevice: func(t *testing.T, device *v1alpha1.Device) {
				assert.Equal(t, v1alpha1.DevicePhaseProvisioned, device.Status.Phase)
				assert.True(t, device.Status.Provisioning[0].EndTime.IsZero())
			},
		},
		{
			name:          "successfully update device status for failed provisioning",
			method:        http.MethodPut,
			authorization: "Bearer validtoken",
			serial:        "ABC123",
			body:          StatusReport{Status: v1alpha1.ProvisioningScriptExecutionFailed, Detail: "Script execution failed"},
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Status: v1alpha1.DeviceStatus{
					SerialNumber: "ABC123",
					Provisioning: []v1alpha1.ProvisioningInfo{{Token: "validtoken", StartTime: metav1.Now()}},
				},
			},
			expectedStatus: http.StatusCreated,
			validateDevice: func(t *testing.T, device *v1alpha1.Device) {
				assert.Equal(t, v1alpha1.DevicePhaseFailed, device.Status.Phase)
				assert.False(t, device.Status.Provisioning[0].EndTime.IsZero())
				assert.Equal(t, "Script execution failed", device.Status.Provisioning[0].Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != nil {
				switch v := tt.body.(type) {
				case string:
					body = bytes.NewBufferString(v)
				default:
					buf, err := json.Marshal(v)
					require.NoError(t, err)
					body = bytes.NewReader(buf)
				}
			}

			url := "/provisioning/status-report"
			if tt.serial != "" {
				url += "?serial=" + tt.serial
			}

			req := httptest.NewRequest(tt.method, url, body)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}

			clientBuilder := fake.NewClientBuilder().WithScheme(scheme.Scheme)
			if tt.device != nil {
				clientBuilder.WithObjects(tt.device).WithStatusSubresource(tt.device)
			}
			k8sClient := clientBuilder.Build()

			server := &HTTPServer{
				Client:   k8sClient,
				Logger:   klog.NewKlogr(),
				Recorder: record.NewFakeRecorder(10),
			}

			rr := httptest.NewRecorder()
			server.HandleStatusReport(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedBody)
			}

			if tt.validateDevice != nil {
				var device v1alpha1.Device
				err := k8sClient.Get(context.Background(), types.NamespacedName{Name: tt.device.Name, Namespace: tt.device.Namespace}, &device)
				require.NoError(t, err)
				tt.validateDevice(t, &device)
			}
		})
	}
}

func TestHandleProvisioningRequest(t *testing.T) {
	tests := []struct {
		name             string
		serial           string
		remoteAddr       string
		device           *v1alpha1.Device
		secret           *corev1.Secret
		validateSourceIP bool
		expectedStatus   int
		expectedBody     string
		validateResponse func(*testing.T, *ProvisioningResponse)
	}{
		{
			name:           "reject requests without serial parameter",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Serial parameter is required",
		},
		{
			name:           "device not found",
			serial:         "NONEXISTENT",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to find device",
		},
		{
			name:       "reject request when source IP validation fails",
			serial:     "ABC123",
			remoteAddr: "192.168.1.100:12345",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: "192.168.1.200:22"},
				},
				Status: v1alpha1.DeviceStatus{SerialNumber: "ABC123"},
			},
			validateSourceIP: true,
			expectedStatus:   http.StatusForbidden,
			expectedBody:     "Source IP does not match device IP",
		},
		{
			name:       "return error when no active provisioning and the device is active",
			serial:     "ABC123",
			remoteAddr: "192.168.1.100:12345",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.1.100:22",
						SecretRef: &v1alpha1.SecretReference{
							Name: "test-secret",
						},
					},
				},
				Status: v1alpha1.DeviceStatus{SerialNumber: "ABC123", Phase: v1alpha1.DevicePhaseRunning},
			},
			secret: &corev1.Secret{
				Type: corev1.SecretTypeBasicAuth,
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"username": []byte("admin"),
					"password": []byte("secret"),
				},
			},
			validateSourceIP: true,
			expectedStatus:   http.StatusPreconditionRequired,
			expectedBody:     "Failed to create provisioning entry",
		},
		{
			name:             "successfully return provisioning configuration",
			serial:           "ABC123",
			remoteAddr:       "192.168.1.100:12345",
			device:           testDevice.DeepCopy(),
			secret:           testSecret.DeepCopy(),
			validateSourceIP: true,
			expectedStatus:   http.StatusOK,
			validateResponse: func(t *testing.T, res *ProvisioningResponse) {
				assert.Equal(t, "validtoken", res.ProvisioningToken)
				assert.Equal(t, "http://example.com/image.bin", res.Image.URL)
				assert.Equal(t, "test-device", res.Hostname)
				assert.Len(t, res.UserAccounts, 1)
				assert.Equal(t, "admin", res.UserAccounts[0].Username)
				assert.Equal(t, "hashedpass", res.UserAccounts[0].HashedPassword)
				assert.Equal(t, "sha256", res.UserAccounts[0].HashAlgorithm)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/provisioning/config"
			if tt.serial != "" {
				url += "?serial=" + tt.serial
			}

			req := httptest.NewRequest(http.MethodGet, url, http.NoBody)
			if tt.remoteAddr != "" {
				req.RemoteAddr = tt.remoteAddr
			}

			clientBuilder := fake.NewClientBuilder().WithScheme(scheme.Scheme)
			if tt.device != nil {
				clientBuilder.WithObjects(tt.device)
			}
			if tt.secret != nil {
				clientBuilder.WithObjects(tt.secret)
			}
			k8sClient := clientBuilder.Build()

			server := &HTTPServer{
				Client:           k8sClient,
				Logger:           klog.NewKlogr(),
				ValidateSourceIP: tt.validateSourceIP,
				Provider:         new(MockProvider),
			}

			rr := httptest.NewRecorder()
			server.HandleProvisioningRequest(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedBody)
			}

			if tt.validateResponse != nil {
				assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
				var res ProvisioningResponse
				err := json.Unmarshal(rr.Body.Bytes(), &res)
				require.NoError(t, err)
				tt.validateResponse(t, &res)
			}
		})
	}
}

func TestGetDeviceCertificate(t *testing.T) {
	tests := []struct {
		name             string
		method           string
		serial           string
		authorization    string
		device           *v1alpha1.Device
		certificates     []*v1alpha1.Certificate
		certSecrets      []*corev1.Secret
		expectedStatus   int
		expectedBody     string
		validateResponse func(*testing.T, *DeviceCertificateResponse)
	}{
		{
			name:           "reject non-GET requests",
			method:         http.MethodPost,
			serial:         "ABC123",
			authorization:  "Bearer validtoken",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed",
		},
		{
			name:           "reject requests without authorization header",
			method:         http.MethodGet,
			serial:         "ABC123",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized",
		},
		{
			name:           "reject requests without serial parameter",
			method:         http.MethodGet,
			authorization:  "Bearer validtoken",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Serial parameter is required",
		},
		{
			name:           "device not found",
			method:         http.MethodGet,
			serial:         "NONEXISTENT",
			authorization:  "Bearer validtoken",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to find device",
		},
		{
			name:          "no active provisioning found",
			method:        http.MethodGet,
			serial:        "ABC123",
			authorization: "Bearer validtoken",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Status: v1alpha1.DeviceStatus{SerialNumber: "ABC123"},
			},
			expectedStatus: http.StatusPreconditionFailed,
			expectedBody:   "no active provisioning found",
		},
		{
			name:          "invalid token",
			method:        http.MethodGet,
			serial:        "ABC123",
			authorization: "Bearer wrongtoken",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Status: v1alpha1.DeviceStatus{
					SerialNumber: "ABC123",
					Provisioning: []v1alpha1.ProvisioningInfo{{Token: "validtoken", StartTime: metav1.Now()}},
				},
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "unauthorized: invalid token",
		},
		{
			name:          "no certificate found for device",
			method:        http.MethodGet,
			serial:        "ABC123",
			authorization: "Bearer validtoken",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Status: v1alpha1.DeviceStatus{
					SerialNumber: "ABC123",
					Provisioning: []v1alpha1.ProvisioningInfo{{Token: "validtoken", StartTime: metav1.Now()}},
				},
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "No certificate found for device",
		},
		{
			name:          "multiple certificates found for device",
			method:        http.MethodGet,
			serial:        "ABC123",
			authorization: "Bearer validtoken",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Status: v1alpha1.DeviceStatus{
					SerialNumber: "ABC123",
					Provisioning: []v1alpha1.ProvisioningInfo{{Token: "validtoken", StartTime: metav1.Now()}},
				},
			},
			certificates: []*v1alpha1.Certificate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-device-cert-1",
						Namespace: "default",
						Labels:    map[string]string{v1alpha1.DeviceLabel: "test-device"},
					},
					Spec: v1alpha1.CertificateSpec{
						SecretRef: v1alpha1.SecretReference{Name: "test-device-cert-secret-1"},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-device-cert-2",
						Namespace: "default",
						Labels:    map[string]string{v1alpha1.DeviceLabel: "test-device"},
					},
					Spec: v1alpha1.CertificateSpec{
						SecretRef: v1alpha1.SecretReference{Name: "test-device-cert-secret-2"},
					},
				},
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Multiple certificates found for device",
		},
		{
			name:          "certificate secret not found",
			method:        http.MethodGet,
			serial:        "ABC123",
			authorization: "Bearer validtoken",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Status: v1alpha1.DeviceStatus{
					SerialNumber: "ABC123",
					Provisioning: []v1alpha1.ProvisioningInfo{{Token: "validtoken", StartTime: metav1.Now()}},
				},
			},
			certificates: []*v1alpha1.Certificate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-device-cert",
						Namespace: "default",
						Labels:    map[string]string{v1alpha1.DeviceLabel: "test-device"},
					},
					Spec: v1alpha1.CertificateSpec{
						SecretRef: v1alpha1.SecretReference{Name: "nonexistent-secret"},
					},
				},
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to get certificate secret",
		},
		{
			name:          "incomplete certificate data - missing private key",
			method:        http.MethodGet,
			serial:        "ABC123",
			authorization: "Bearer validtoken",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Status: v1alpha1.DeviceStatus{
					SerialNumber: "ABC123",
					Provisioning: []v1alpha1.ProvisioningInfo{{Token: "validtoken", StartTime: metav1.Now()}},
				},
			},
			certificates: []*v1alpha1.Certificate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-device-cert",
						Namespace: "default",
						Labels:    map[string]string{v1alpha1.DeviceLabel: "test-device"},
					},
					Spec: v1alpha1.CertificateSpec{
						SecretRef: v1alpha1.SecretReference{Name: "test-device-cert-secret"},
					},
				},
			},
			certSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-device-cert-secret",
						Namespace: "default",
					},
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						"tls.crt": []byte("-----BEGIN CERTIFICATE-----\ntest-cert\n-----END CERTIFICATE-----"),
					},
				},
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to get certificate secret",
		},
		{
			name:          "successfully return device certificate with all fields",
			method:        http.MethodGet,
			serial:        "ABC123",
			authorization: "Bearer validtoken",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Status: v1alpha1.DeviceStatus{
					SerialNumber: "ABC123",
					Provisioning: []v1alpha1.ProvisioningInfo{{Token: "validtoken", StartTime: metav1.Now()}},
				},
			},
			certificates: []*v1alpha1.Certificate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-device-cert",
						Namespace: "default",
						Labels:    map[string]string{v1alpha1.DeviceLabel: "test-device"},
					},
					Spec: v1alpha1.CertificateSpec{
						SecretRef: v1alpha1.SecretReference{Name: "test-device-cert-secret"},
					},
				},
			},
			certSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-device-cert-secret",
						Namespace: "default",
					},
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						"tls.crt": []byte("-----BEGIN CERTIFICATE-----\ntest-cert\n-----END CERTIFICATE-----"),
						"tls.key": []byte("-----BEGIN PRIVATE KEY-----\ntest-key\n-----END PRIVATE KEY-----"),
						"ca.crt":  []byte("-----BEGIN CERTIFICATE-----\ntest-ca\n-----END CERTIFICATE-----"),
					},
				},
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, response *DeviceCertificateResponse) {
				assert.Equal(t, "-----BEGIN CERTIFICATE-----\ntest-cert\n-----END CERTIFICATE-----", response.Certificate)
				assert.Equal(t, "-----BEGIN PRIVATE KEY-----\ntest-key\n-----END PRIVATE KEY-----", response.PrivateKey)
				assert.Equal(t, "-----BEGIN CERTIFICATE-----\ntest-ca\n-----END CERTIFICATE-----", response.CACertificate)
			},
		},
		{
			name:          "successfully return device certificate without CA certificate",
			method:        http.MethodGet,
			serial:        "ABC123",
			authorization: "Bearer validtoken",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Status: v1alpha1.DeviceStatus{
					SerialNumber: "ABC123",
					Provisioning: []v1alpha1.ProvisioningInfo{{Token: "validtoken", StartTime: metav1.Now()}},
				},
			},
			certificates: []*v1alpha1.Certificate{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-device-cert",
						Namespace: "default",
						Labels:    map[string]string{v1alpha1.DeviceLabel: "test-device"},
					},
					Spec: v1alpha1.CertificateSpec{
						SecretRef: v1alpha1.SecretReference{Name: "test-device-cert-secret"},
					},
				},
			},
			certSecrets: []*corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-device-cert-secret",
						Namespace: "default",
					},
					Type: corev1.SecretTypeTLS,
					Data: map[string][]byte{
						"tls.crt": []byte("-----BEGIN CERTIFICATE-----\ntest-cert\n-----END CERTIFICATE-----"),
						"tls.key": []byte("-----BEGIN PRIVATE KEY-----\ntest-key\n-----END PRIVATE KEY-----"),
					},
				},
			},
			expectedStatus: http.StatusOK,
			validateResponse: func(t *testing.T, response *DeviceCertificateResponse) {
				assert.Equal(t, "-----BEGIN CERTIFICATE-----\ntest-cert\n-----END CERTIFICATE-----", response.Certificate)
				assert.Equal(t, "-----BEGIN PRIVATE KEY-----\ntest-key\n-----END PRIVATE KEY-----", response.PrivateKey)
				assert.Empty(t, response.CACertificate)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/provisioning/device-certificate"
			if tt.serial != "" {
				url += "?serial=" + tt.serial
			}

			req := httptest.NewRequest(tt.method, url, http.NoBody)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}

			clientBuilder := fake.NewClientBuilder().WithScheme(scheme.Scheme)
			if tt.device != nil {
				clientBuilder.WithObjects(tt.device).WithStatusSubresource(tt.device)
			}
			for _, cert := range tt.certificates {
				clientBuilder.WithObjects(cert)
			}
			for _, secret := range tt.certSecrets {
				clientBuilder.WithObjects(secret)
			}
			k8sClient := clientBuilder.Build()

			server := &HTTPServer{
				Client: k8sClient,
				Logger: klog.NewKlogr(),
			}

			rr := httptest.NewRecorder()
			server.GetDeviceCertificate(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedBody)
			}

			if tt.validateResponse != nil {
				assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
				var response DeviceCertificateResponse
				err := json.Unmarshal(rr.Body.Bytes(), &response)
				require.NoError(t, err)
				tt.validateResponse(t, &response)
			}
		})
	}
}

func TestGetMTLSClientCA(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		serial         string
		authorization  string
		device         *v1alpha1.Device
		caSecret       *corev1.Secret
		expectedStatus int
		expectedBody   string
		validateCA     func(*testing.T, []byte)
	}{
		{
			name:           "reject non-GET requests",
			method:         http.MethodPost,
			serial:         "ABC123",
			authorization:  "Bearer validtoken",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedBody:   "Method not allowed",
		},
		{
			name:           "reject requests without authorization header",
			method:         http.MethodGet,
			serial:         "ABC123",
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized",
		},
		{
			name:           "reject requests without serial parameter",
			method:         http.MethodGet,
			authorization:  "Bearer validtoken",
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Serial parameter is required",
		},
		{
			name:           "device not found",
			method:         http.MethodGet,
			serial:         "NONEXISTENT",
			authorization:  "Bearer validtoken",
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to find device",
		},
		{
			name:          "no active provisioning found",
			method:        http.MethodGet,
			serial:        "ABC123",
			authorization: "Bearer validtoken",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Status: v1alpha1.DeviceStatus{SerialNumber: "ABC123"},
			},
			expectedStatus: http.StatusPreconditionFailed,
			expectedBody:   "no active provisioning found",
		},
		{
			name:          "invalid token",
			method:        http.MethodGet,
			serial:        "ABC123",
			authorization: "Bearer wrongtoken",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Status: v1alpha1.DeviceStatus{
					SerialNumber: "ABC123",
					Provisioning: []v1alpha1.ProvisioningInfo{{Token: "validtoken", StartTime: metav1.Now()}},
				},
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "unauthorized: invalid token",
		},
		{
			name:          "device has no MTLS configuration",
			method:        http.MethodGet,
			serial:        "ABC123",
			authorization: "Bearer validtoken",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.1.100:22",
					},
				},
				Status: v1alpha1.DeviceStatus{
					SerialNumber: "ABC123",
					Provisioning: []v1alpha1.ProvisioningInfo{{Token: "validtoken", StartTime: metav1.Now()}},
				},
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Device has no TLS configuration",
		},
		{
			name:          "device has no MTLS configured but a CA for server validation",
			method:        http.MethodGet,
			serial:        "ABC123",
			authorization: "Bearer validtoken",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.1.100:22",
						TLS: &v1alpha1.TLS{
							CA: v1alpha1.SecretKeySelector{
								SecretReference: v1alpha1.SecretReference{
									Name: "operator-ca-secret",
								},
								Key: "ca.crt",
							},
						},
					},
				},
				Status: v1alpha1.DeviceStatus{
					SerialNumber: "ABC123",
					Provisioning: []v1alpha1.ProvisioningInfo{{Token: "validtoken", StartTime: metav1.Now()}},
				},
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Device has no MTLS certificate configured",
		},
		{
			name:          "CA secret not found",
			method:        http.MethodGet,
			serial:        "ABC123",
			authorization: "Bearer validtoken",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.1.100:22",
						TLS: &v1alpha1.TLS{
							Certificate: &v1alpha1.CertificateSource{
								SecretRef: v1alpha1.SecretReference{
									Name: "device-cert-secret",
								},
							},
						},
					},
				},
				Status: v1alpha1.DeviceStatus{
					SerialNumber: "ABC123",
					Provisioning: []v1alpha1.ProvisioningInfo{{Token: "validtoken", StartTime: metav1.Now()}},
				},
			},
			expectedStatus: http.StatusNotFound,
			expectedBody:   "Failed to get device certificate secret",
		},
		{
			name:          "successfully return MTLS client CA certificate",
			method:        http.MethodGet,
			serial:        "ABC123",
			authorization: "Bearer validtoken",
			device: &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-device",
					Namespace: "default",
					Labels:    map[string]string{v1alpha1.DeviceSerialLabel: "ABC123"},
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{
						Address: "192.168.1.100:22",
						TLS: &v1alpha1.TLS{
							Certificate: &v1alpha1.CertificateSource{
								SecretRef: v1alpha1.SecretReference{
									Name: "operator-ca-secret",
								},
							},
						},
					},
				},
				Status: v1alpha1.DeviceStatus{
					SerialNumber: "ABC123",
					Provisioning: []v1alpha1.ProvisioningInfo{{Token: "validtoken", StartTime: metav1.Now()}},
				},
			},
			caSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "operator-ca-secret",
					Namespace: "default",
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{
					"tls.crt": []byte("placeholder"),
					"tls.key": []byte("placeholder"),
					"ca.crt":  []byte("-----BEGIN CERTIFICATE-----\noperator-ca-cert\n-----END CERTIFICATE-----"),
				},
			},
			expectedStatus: http.StatusOK,
			validateCA: func(t *testing.T, ca []byte) {
				assert.Equal(t, []byte("-----BEGIN CERTIFICATE-----\noperator-ca-cert\n-----END CERTIFICATE-----"), ca)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/provisioning/mtls-client-ca"
			if tt.serial != "" {
				url += "?serial=" + tt.serial
			}

			req := httptest.NewRequest(tt.method, url, http.NoBody)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}

			clientBuilder := fake.NewClientBuilder().WithScheme(scheme.Scheme)
			if tt.device != nil {
				clientBuilder.WithObjects(tt.device).WithStatusSubresource(tt.device)
			}
			if tt.caSecret != nil {
				clientBuilder.WithObjects(tt.caSecret)
			}
			k8sClient := clientBuilder.Build()

			server := &HTTPServer{
				Client: k8sClient,
				Logger: klog.NewKlogr(),
			}

			rr := httptest.NewRecorder()
			server.GetMTLSClientCA(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedBody != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedBody)
			}

			if tt.validateCA != nil {
				assert.Equal(t, "application/x-pem-file", rr.Header().Get("Content-Type"))
				tt.validateCA(t, rr.Body.Bytes())
			}
		})
	}
}

func init() {
	utilruntime.Must(v1alpha1.AddToScheme(scheme.Scheme))
}
