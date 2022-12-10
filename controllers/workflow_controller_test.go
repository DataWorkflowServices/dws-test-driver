package controllers

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dwsv1alpha1 "github.com/HewlettPackard/dws/api/v1alpha1"
)

var _ = Describe("Workflow Controller Test", func() {

	var (
		wf *dwsv1alpha1.Workflow
	)

	BeforeEach(func() {
		wfid := uuid.NewString()[0:8]
		wf = &dwsv1alpha1.Workflow{
			ObjectMeta: metav1.ObjectMeta{
				Name:      wfid,
				Namespace: corev1.NamespaceDefault,
			},
			Spec: dwsv1alpha1.WorkflowSpec{
				DesiredState: dwsv1alpha1.StateProposal,
				WLMID:        "test",
				JobID:        0,
				UserID:       0,
				GroupID:      0,
				DWDirectives: []string{},
			},
			Status: dwsv1alpha1.WorkflowStatus{
				State:  dwsv1alpha1.StateProposal,
				Ready:  false,
				Status: dwsv1alpha1.StatusDriverWait,
			},
		}
	})

	AfterEach(func() {
		if wf != nil {
			Expect(k8sClient.Delete(context.TODO(), wf)).To(Succeed())

			wfExpected := &dwsv1alpha1.Workflow{}
			Eventually(func() error { // Delete can still return the cached object. Wait until the object is no longer present.
				return k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(wf), wfExpected)
			}).ShouldNot(Succeed())
		}
	})

	It("Can complete Workflow driver states", func() {
		action := "complete"
		wf.Spec.DWDirectives = []string{
			fmt.Sprintf("#DW STATUS action=%s", action),
		}

		driverStatus := dwsv1alpha1.WorkflowDriverStatus{
			DriverID:   DRIVERID,
			DWDIndex:   0,
			WatchState: dwsv1alpha1.StateProposal,
		}

		// Set a bunch of other statuses that should remain unchanged
		statusWrongDriver := dwsv1alpha1.WorkflowDriverStatus{
			DriverID:   "notMyDriver",
			DWDIndex:   0,
			WatchState: dwsv1alpha1.StateProposal,
		}
		statusWrongWatchState := dwsv1alpha1.WorkflowDriverStatus{
			DriverID:   DRIVERID,
			DWDIndex:   0,
			WatchState: dwsv1alpha1.StateDataIn,
		}
		statusCompleted := dwsv1alpha1.WorkflowDriverStatus{
			DriverID:   DRIVERID,
			DWDIndex:   0,
			WatchState: dwsv1alpha1.StateProposal,
			Completed:  true,
		}
		statusError := dwsv1alpha1.WorkflowDriverStatus{
			DriverID:   DRIVERID,
			DWDIndex:   0,
			WatchState: dwsv1alpha1.StateProposal,
			Status:     dwsv1alpha1.StatusError,
		}
		wf.Status.Drivers = []dwsv1alpha1.WorkflowDriverStatus{
			driverStatus,
			statusWrongDriver,
			statusWrongWatchState,
			statusCompleted,
			statusError,
		}

		ct := metav1.NowMicro()
		expectedDriverStatuses := []dwsv1alpha1.WorkflowDriverStatus{
			{
				DriverID:     DRIVERID,
				DWDIndex:     0,
				WatchState:   dwsv1alpha1.StateProposal,
				Status:       dwsv1alpha1.StatusCompleted,
				Completed:    true,
				CompleteTime: &ct,
			},
			statusWrongDriver,
			statusWrongWatchState,
			statusCompleted,
			statusError,
		}

		Expect(k8sClient.Create(context.TODO(), wf)).To(Succeed())

		Eventually(func(g Gomega) []dwsv1alpha1.WorkflowDriverStatus {
			returnWf := &dwsv1alpha1.Workflow{}
			g.Expect(k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(wf), returnWf)).To(Succeed())
			driverStatus = returnWf.Status.Drivers[0]
			return returnWf.Status.Drivers
		}).Should(BeComparableTo(expectedDriverStatuses,
			cmp.Comparer(func(x, y *metav1.MicroTime) bool {
				// Don't compare times, just check for Nil
				bothSet := !(reflect.ValueOf(x).IsNil() || reflect.ValueOf(y).IsNil())
				bothUnset := reflect.ValueOf(x).IsNil() && reflect.ValueOf(y).IsNil()
				return bothSet || bothUnset
			}),
		))
	})

	It("Can set Workflow driver errors", func() {
		action := "error"
		message := "Test_error_message"
		wf.Spec.DWDirectives = []string{
			fmt.Sprintf("#DW STATUS action=%s message=%s", action, message),
		}

		driverStatus := dwsv1alpha1.WorkflowDriverStatus{
			DriverID:   DRIVERID,
			DWDIndex:   0,
			WatchState: dwsv1alpha1.StateProposal,
		}
		wf.Status.Drivers = []dwsv1alpha1.WorkflowDriverStatus{driverStatus}

		expectedDriverStatus := dwsv1alpha1.WorkflowDriverStatus{
			DriverID:   DRIVERID,
			DWDIndex:   0,
			WatchState: dwsv1alpha1.StateProposal,
			Status:     dwsv1alpha1.StatusError,
			Error:      "Test error message",
		}

		Expect(k8sClient.Create(context.TODO(), wf)).To(Succeed())

		Eventually(func(g Gomega) dwsv1alpha1.WorkflowDriverStatus {
			returnWf := &dwsv1alpha1.Workflow{}
			g.Expect(k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(wf), returnWf)).To(Succeed())
			return returnWf.Status.Drivers[0]
		}).Should(Equal(expectedDriverStatus))
	})

	It("Can No-op Workflow driver statuses", func() {
		action := "wait"
		wf.Spec.DWDirectives = []string{
			fmt.Sprintf("#DW STATUS action=%s", action),
		}

		driverStatus := dwsv1alpha1.WorkflowDriverStatus{
			DriverID:   DRIVERID,
			DWDIndex:   0,
			WatchState: dwsv1alpha1.StateProposal,
		}
		wf.Status.Drivers = []dwsv1alpha1.WorkflowDriverStatus{driverStatus}

		// The driver status should be unchanged
		expectedDriverStatus := dwsv1alpha1.WorkflowDriverStatus{
			DriverID:   DRIVERID,
			DWDIndex:   0,
			WatchState: dwsv1alpha1.StateProposal,
		}

		Expect(k8sClient.Create(context.TODO(), wf)).To(Succeed())

		Eventually(func(g Gomega) dwsv1alpha1.WorkflowDriverStatus {
			returnWf := &dwsv1alpha1.Workflow{}
			g.Expect(k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(wf), returnWf)).To(Succeed())
			return returnWf.Status.Drivers[0]
		}).Should(Equal(expectedDriverStatus))
	})
})
