// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and IronCore contributors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/ironcore-dev/network-operator/api/core/v1alpha1"
	"github.com/ironcore-dev/network-operator/internal/objectstorage"
	"github.com/ironcore-dev/network-operator/internal/provider"
)

var _ = Describe("ConfigBackup Controller", func() {
	Context("When reconciling a resource", func() {
		var device *v1alpha1.Device

		var backup *v1alpha1.ConfigBackup

		BeforeEach(func() {
			By("Creating a Device resource")
			device = &v1alpha1.Device{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-configbackup-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.DeviceSpec{
					Endpoint: v1alpha1.Endpoint{Address: "192.168.10.2:9339"},
				},
			}
			Expect(k8sClient.Create(ctx, device)).To(Succeed())

			By("Waiting for the Device to become Running and Reachable")
			Eventually(func(g Gomega) {
				current := &v1alpha1.Device{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(device), current)).To(Succeed())
				g.Expect(current.Status.Phase).To(Equal(v1alpha1.DevicePhaseRunning))
				cond := meta.FindStatusCondition(current.Status.Conditions, v1alpha1.ReachableCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Resetting the test provider state")
			testProvider.Lock()
			testProvider.ConfigBackups = nil
			testProvider.StartupConfig = nil
			testProvider.Unlock()
		})

		AfterEach(func() {
			if backup != nil {
				By("Deleting the ConfigBackup resource")
				Expect(k8sClient.Delete(ctx, backup)).To(Succeed())
				backup = nil
			}
			By("Deleting the Device resource")
			Expect(k8sClient.Delete(ctx, device)).To(Succeed())
		})

		It("Should successfully reconcile a one-shot local backup", func() {
			By("Creating a Local ConfigBackup resource")
			backup = &v1alpha1.ConfigBackup{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-configbackup-local-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ConfigBackupSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					Type:      v1alpha1.ConfigBackupTypeLocal,
					Path:      "bootflash:///backups/",
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			By("Verifying the controller adds a finalizer")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ConfigBackup{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), resource)).To(Succeed())
				g.Expect(controllerutil.ContainsFinalizer(resource, v1alpha1.FinalizerName)).To(BeTrue())
			}).Should(Succeed())

			By("Verifying the controller adds the device label")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ConfigBackup{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), resource)).To(Succeed())
				g.Expect(resource.Labels).To(HaveKeyWithValue(v1alpha1.DeviceLabel, device.Name))
			}).Should(Succeed())

			By("Verifying the controller sets the device as owner reference")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ConfigBackup{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), resource)).To(Succeed())
				g.Expect(resource.OwnerReferences).To(HaveLen(1))
				g.Expect(resource.OwnerReferences[0].Kind).To(Equal("Device"))
				g.Expect(resource.OwnerReferences[0].Name).To(Equal(device.Name))
			}).Should(Succeed())

			By("Verifying the backup status is populated")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ConfigBackup{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), resource)).To(Succeed())
				g.Expect(resource.Status.LastBackup).NotTo(BeNil())
				g.Expect(resource.Status.LastBackup.Timestamp.IsZero()).To(BeFalse())
				g.Expect(resource.Status.LastBackup.Filepath).NotTo(BeEmpty())
				g.Expect(resource.Status.LastBackup.SizeBytes).NotTo(BeNil())
				g.Expect(resource.Status.LastAttemptTime.IsZero()).To(BeFalse())
				g.Expect(resource.Status.TotalBackups).NotTo(BeNil())
				g.Expect(*resource.Status.TotalBackups).To(Equal(int32(1)))
				g.Expect(resource.Status.Storage).NotTo(BeNil())
				g.Expect(resource.Status.Storage.TotalBytes).NotTo(BeNil())

				cond := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				g.Expect(cond.Reason).To(Equal(v1alpha1.BackupSuccessfulReason))
			}).Should(Succeed())
		})

		It("Should successfully reconcile a startup backup", func() {
			By("Creating a Startup ConfigBackup resource")
			backup = &v1alpha1.ConfigBackup{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-configbackup-startup-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ConfigBackupSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					Type:      v1alpha1.ConfigBackupTypeStartup,
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			By("Verifying the backup status is populated")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ConfigBackup{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), resource)).To(Succeed())
				g.Expect(resource.Status.LastBackup).NotTo(BeNil())
				g.Expect(resource.Status.LastBackup.Filepath).To(BeEmpty())
				g.Expect(resource.Status.LastBackup.SizeBytes).To(BeNil())
				g.Expect(resource.Status.TotalBackups).NotTo(BeNil())
				g.Expect(*resource.Status.TotalBackups).To(Equal(int32(1)))
				g.Expect(resource.Status.Storage).To(BeNil())

				cond := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				g.Expect(cond.Reason).To(Equal(v1alpha1.BackupSuccessfulReason))
			}).Should(Succeed())

			By("Verifying the provider received the startup backup")
			Eventually(func(g Gomega) {
				testProvider.Lock()
				defer testProvider.Unlock()
				g.Expect(testProvider.StartupConfig).NotTo(BeNil())
			}).Should(Succeed())
		})

		It("Should rotate old backups according to retention policy", func() {
			By("Pre-seeding the provider with existing backups")
			testProvider.Lock()
			size := int64(1024)
			testProvider.ConfigBackups = []*provider.ConfigBackupFile{
				{Path: "bootflash:///backups/configbackup-old-1", SizeBytes: &size, CreatedAt: time.Date(2026, time.April, 10, 2, 0, 0, 0, time.UTC)},
				{Path: "bootflash:///backups/configbackup-old-2", SizeBytes: &size, CreatedAt: time.Date(2026, time.April, 11, 2, 0, 0, 0, time.UTC)},
				{Path: "bootflash:///backups/configbackup-old-3", SizeBytes: &size, CreatedAt: time.Date(2026, time.April, 12, 2, 0, 0, 0, time.UTC)},
			}
			testProvider.Unlock()

			By("Creating a ConfigBackup with retention keepLast: 2")
			backup = &v1alpha1.ConfigBackup{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-configbackup-retention-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ConfigBackupSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					Type:      v1alpha1.ConfigBackupTypeLocal,
					Path:      "bootflash:///backups/",
					Retention: &v1alpha1.ConfigBackupRetention{KeepLast: 2},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			By("Verifying old backups are rotated")
			Eventually(func(g Gomega) {
				testProvider.Lock()
				defer testProvider.Unlock()
				// 3 pre-seeded + 1 new = 4, keepLast=2 means 2 oldest deleted → 2 remain
				g.Expect(testProvider.ConfigBackups).To(HaveLen(2))
			}).Should(Succeed())

			By("Verifying the status reflects the retained count")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ConfigBackup{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), resource)).To(Succeed())
				g.Expect(resource.Status.TotalBackups).NotTo(BeNil())
				g.Expect(*resource.Status.TotalBackups).To(Equal(int32(2)))
			}).Should(Succeed())
		})

		It("Should block backup when storage threshold is exceeded", func() {
			By("Pre-seeding the provider to simulate full storage")
			testProvider.Lock()
			size := int64(95)
			testProvider.ConfigBackups = []*provider.ConfigBackupFile{
				{Path: "bootflash:///backups/existing-file", SizeBytes: &size, CreatedAt: time.Now()},
			}
			testProvider.StorageTotal = 100
			testProvider.Unlock()

			By("Creating a ConfigBackup with a storage threshold")
			minFreeBytes := int64(10)
			backup = &v1alpha1.ConfigBackup{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-configbackup-threshold-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ConfigBackupSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					Type:      v1alpha1.ConfigBackupTypeLocal,
					Path:      "bootflash:///backups/",
					StorageThreshold: &v1alpha1.ConfigBackupStorageThreshold{
						MinFreeBytes: &minFreeBytes,
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			By("Verifying the backup is blocked")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ConfigBackup{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), resource)).To(Succeed())
				g.Expect(resource.Status.LastBackup).To(BeNil())

				cond := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(cond.Reason).To(Equal(v1alpha1.StorageThresholdExceededReason))
			}).Should(Succeed())
		})

		It("Should not re-run a one-shot backup after it succeeds", func() {
			By("Creating a Local ConfigBackup resource")
			backup = &v1alpha1.ConfigBackup{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-configbackup-oneshot-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ConfigBackupSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					Type:      v1alpha1.ConfigBackupTypeLocal,
					Path:      "bootflash:///backups/",
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			By("Waiting for the backup to succeed")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ConfigBackup{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), resource)).To(Succeed())
				g.Expect(resource.Status.LastBackup).NotTo(BeNil())
			}).Should(Succeed())

			By("Verifying the provider only has one backup (no re-runs)")
			Consistently(func(g Gomega) {
				testProvider.Lock()
				defer testProvider.Unlock()
				g.Expect(testProvider.ConfigBackups).To(HaveLen(1))
			}).Should(Succeed())
		})

		It("Should successfully reconcile a remote backup to S3", func() {
			By("Creating a Secret with S3 credentials")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "s3-creds-",
					Namespace:    metav1.NamespaceDefault,
				},
				Data: map[string][]byte{
					"accessKeyID":     []byte("EXAMPLEACCESSKEY"),
					"secretAccessKey": []byte("EXAMPLESECRETKEY"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			By("Creating a Remote ConfigBackup resource")
			backup = &v1alpha1.ConfigBackup{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "test-configbackup-remote-",
					Namespace:    metav1.NamespaceDefault,
				},
				Spec: v1alpha1.ConfigBackupSpec{
					DeviceRef: v1alpha1.LocalObjectReference{Name: device.Name},
					Type:      v1alpha1.ConfigBackupTypeRemote,
					Path:      "leaf-1/",
					S3: &v1alpha1.ConfigBackupS3{
						Endpoint:             "https://s3.example.com",
						Bucket:               "network-config-backups",
						CredentialsSecretRef: v1alpha1.SecretReference{Name: secret.Name},
					},
				},
			}
			Expect(k8sClient.Create(ctx, backup)).To(Succeed())

			By("Verifying the backup status is populated")
			Eventually(func(g Gomega) {
				resource := &v1alpha1.ConfigBackup{}
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), resource)).To(Succeed())
				g.Expect(resource.Status.LastBackup).NotTo(BeNil())
				g.Expect(resource.Status.LastBackup.Filepath).To(HavePrefix("s3://network-config-backups/leaf-1/configbackup-"))
				g.Expect(resource.Status.LastBackup.SizeBytes).NotTo(BeNil())
				g.Expect(*resource.Status.LastBackup.SizeBytes).To(BeNumerically(">", 0))
				g.Expect(resource.Status.LastAttemptTime.IsZero()).To(BeFalse())

				cond := meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.ReadyCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				g.Expect(cond.Reason).To(Equal(v1alpha1.BackupSuccessfulReason))

				cond = meta.FindStatusCondition(resource.Status.Conditions, v1alpha1.RemoteEndpointReadyCondition)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())

			By("Verifying the mock S3 server received the upload")
			Expect(testS3Store.Objects).To(HaveLen(1))
			for key, body := range testS3Store.Objects {
				Expect(key).To(HavePrefix("leaf-1/configbackup-"))
				Expect(body).NotTo(BeEmpty())
			}
		})
	})
})

// mockObjectStorage is an in-memory fake implementing the ObjectStorage interface for testing.
type mockObjectStorage struct {
	sync.Mutex

	Objects map[string][]byte // key → body
}

func NewMockObjectStorage() *mockObjectStorage {
	return &mockObjectStorage{Objects: make(map[string][]byte)}
}

func (m *mockObjectStorage) HeadBucket(_ context.Context, _ string) error {
	return nil
}

func (m *mockObjectStorage) PutObject(_ context.Context, obj *objectstorage.Object) error {
	m.Lock()
	defer m.Unlock()
	m.Objects[obj.Key] = obj.Body
	return nil
}

func (m *mockObjectStorage) ListObjects(_ context.Context, _, prefix string) ([]objectstorage.Object, error) {
	m.Lock()
	defer m.Unlock()
	var result []objectstorage.Object
	for k, v := range m.Objects {
		if strings.HasPrefix(k, prefix) {
			result = append(result, objectstorage.Object{
				Key:          k,
				Size:         int64(len(v)),
				LastModified: time.Now().UTC(),
			})
		}
	}
	return result, nil
}

func (m *mockObjectStorage) DeleteObjects(_ context.Context, _ string, keys ...string) error {
	m.Lock()
	defer m.Unlock()
	for _, key := range keys {
		delete(m.Objects, key)
	}
	return nil
}
