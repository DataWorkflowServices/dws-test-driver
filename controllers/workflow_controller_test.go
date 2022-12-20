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
	"k8s.io/apimachinery/pkg/types"

	dwsv1alpha1 "github.com/HewlettPackard/dws/api/v1alpha1"
)

func ignoreExactTime(x, y *metav1.MicroTime) bool {
	// Don't compare times, just check that a time was set
	bothSet := !(reflect.ValueOf(x).IsNil() || reflect.ValueOf(y).IsNil())
	bothUnset := reflect.ValueOf(x).IsNil() && reflect.ValueOf(y).IsNil()
	return bothSet || bothUnset
}

var _ = Describe("Workflow Controller Test", func() {

	var (
		wf                     *dwsv1alpha1.Workflow
		key                    types.NamespacedName
		expectedDriverStatuses []dwsv1alpha1.WorkflowDriverStatus
	)

	BeforeEach(func() {
		wfid := uuid.NewString()[0:8]
		key = types.NamespacedName{
			Name:      "test-workflow-" + wfid,
			Namespace: corev1.NamespaceDefault,
		}

		wf = &dwsv1alpha1.Workflow{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: dwsv1alpha1.WorkflowSpec{
				DesiredState: dwsv1alpha1.StateProposal,
				WLMID:        "test",
				JobID:        0,
				UserID:       0,
				GroupID:      0,
				DWDirectives: []string{},
			},
		}
		Expect(k8sClient.Get(context.TODO(), key, wf)).ToNot(Succeed())
	})

	JustAfterEach(func() {
		Expect(k8sClient.Create(context.TODO(), wf)).To(Succeed())
		Eventually(func(g Gomega) []dwsv1alpha1.WorkflowDriverStatus {
			g.Expect(k8sClient.Get(context.TODO(), key, wf)).To(Succeed())
			return wf.Status.Drivers
		}).Should(BeComparableTo(expectedDriverStatuses,
			cmp.Comparer(ignoreExactTime),
		))
	})

	AfterEach(func() {
		if wf != nil {
			Expect(k8sClient.Delete(context.TODO(), wf)).To(Succeed())
			Eventually(func() error { // Delete can still return the cached object. Wait until the object is no longer present.
				return k8sClient.Get(context.TODO(), key, wf)
			}).ShouldNot(Succeed())
		}
	})

	It("Can complete Workflow driver states", func() {
		state := "Proposal"
		action := "complete"
		wf.Spec.DWDirectives = []string{
			fmt.Sprintf("#DW %s action=%s", state, action),
		}

		aTimeWasSet := metav1.NowMicro()
		expectedDriverStatus := dwsv1alpha1.WorkflowDriverStatus{
			DriverID:     DRIVERID,
			DWDIndex:     0,
			WatchState:   dwsv1alpha1.StateProposal,
			Status:       dwsv1alpha1.StatusCompleted,
			Completed:    true,
			CompleteTime: &aTimeWasSet,
		}

		expectedDriverStatuses = []dwsv1alpha1.WorkflowDriverStatus{
			expectedDriverStatus,
		}
	})

	It("Can set Workflow driver errors", func() {
		state := "Proposal"
		action := "error"
		message := "Test_error_message"
		wf.Spec.DWDirectives = []string{
			fmt.Sprintf("#DW %s action=%s message=%s", state, action, message),
		}

		expectedDriverStatus := dwsv1alpha1.WorkflowDriverStatus{
			DriverID:   DRIVERID,
			DWDIndex:   0,
			WatchState: dwsv1alpha1.StateProposal,
			Status:     dwsv1alpha1.StatusError,
			Error:      "Test error message",
		}

		expectedDriverStatuses = []dwsv1alpha1.WorkflowDriverStatus{
			expectedDriverStatus,
		}
	})

	It("Can No-op Workflow driver statuses", func() {
		state := "Proposal"
		action := "wait"
		wf.Spec.DWDirectives = []string{
			fmt.Sprintf("#DW %s action=%s", state, action),
		}

		expectedDriverStatus := dwsv1alpha1.WorkflowDriverStatus{
			DriverID:   DRIVERID,
			DWDIndex:   0,
			WatchState: dwsv1alpha1.StateProposal,
			Status:     dwsv1alpha1.StatusPending,
			Completed:  false,
		}

		expectedDriverStatuses = []dwsv1alpha1.WorkflowDriverStatus{
			expectedDriverStatus,
		}
	})
})
