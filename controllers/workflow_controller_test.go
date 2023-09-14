/*
Copyright 2022-2023 Hewlett Packard Enterprise Development LP.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	dwsv1alpha2 "github.com/DataWorkflowServices/dws/api/v1alpha2"
)

func ignoreExactTime(x, y *metav1.MicroTime) bool {
	// Don't compare times, just check that a time was set
	bothSet := !(reflect.ValueOf(x).IsNil() || reflect.ValueOf(y).IsNil())
	bothUnset := reflect.ValueOf(x).IsNil() && reflect.ValueOf(y).IsNil()
	return bothSet || bothUnset
}

var _ = Describe("Workflow Controller Test", func() {

	var (
		wf                     *dwsv1alpha2.Workflow
		key                    types.NamespacedName
		expectedDriverStatuses []dwsv1alpha2.WorkflowDriverStatus
	)

	BeforeEach(func() {
		wfid := uuid.NewString()[0:8]
		key = types.NamespacedName{
			Name:      "test-workflow-" + wfid,
			Namespace: corev1.NamespaceDefault,
		}

		wf = &dwsv1alpha2.Workflow{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: dwsv1alpha2.WorkflowSpec{
				DesiredState: dwsv1alpha2.StateProposal,
				WLMID:        "test",
				JobID:        intstr.FromString("wlm job 442"),
				UserID:       0,
				GroupID:      0,
				DWDirectives: []string{},
			},
		}
		Expect(k8sClient.Get(context.TODO(), key, wf)).ToNot(Succeed())
	})

	JustAfterEach(func() {
		Expect(k8sClient.Create(context.TODO(), wf)).To(Succeed())
		Eventually(func(g Gomega) []dwsv1alpha2.WorkflowDriverStatus {
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
		expectedDriverStatus := dwsv1alpha2.WorkflowDriverStatus{
			DriverID:     DRIVERID,
			DWDIndex:     0,
			WatchState:   dwsv1alpha2.StateProposal,
			Status:       dwsv1alpha2.StatusCompleted,
			Completed:    true,
			CompleteTime: &aTimeWasSet,
		}

		expectedDriverStatuses = []dwsv1alpha2.WorkflowDriverStatus{
			expectedDriverStatus}
	})

	DescribeTable("can set Workflow driver errors",
		func(severity string, expectedStatus string) {
			state := "Proposal"
			action := "error"
			message := "Test_error_message"
			dwLine := fmt.Sprintf("#DW %s action=%s message=%s", state, action, message)
			if severity != "" {
				dwLine = dwLine + fmt.Sprintf(" severity=%s", severity)
			}
			wf.Spec.DWDirectives = []string{dwLine}

			expectedDriverStatus := dwsv1alpha2.WorkflowDriverStatus{
				DriverID:   DRIVERID,
				DWDIndex:   0,
				WatchState: dwsv1alpha2.StateProposal,
				Status:     expectedStatus,
				Message:    "Reported error: " + message,
				// Errors are found on the #DW line with
				// underscores representing spaces, which allows the
				// #DW parser to be simple; the controller will swap
				// those back to spaces.
				Error: strings.ReplaceAll(message, "_", " "),
			}

			expectedDriverStatuses = []dwsv1alpha2.WorkflowDriverStatus{
				expectedDriverStatus,
			}
		},
		Entry("without a specified severity", "", dwsv1alpha2.StatusRunning),
		Entry("with a minor severity", string(dwsv1alpha2.SeverityMinor), dwsv1alpha2.StatusRunning),
		Entry("with a major severity", string(dwsv1alpha2.SeverityMajor), dwsv1alpha2.StatusTransientCondition),
		Entry("with a fatal severity", string(dwsv1alpha2.SeverityFatal), dwsv1alpha2.StatusError),
	)

	It("Can No-op Workflow driver statuses", func() {
		state := "Proposal"
		action := "wait"
		wf.Spec.DWDirectives = []string{
			fmt.Sprintf("#DW %s action=%s", state, action),
		}

		expectedDriverStatus := dwsv1alpha2.WorkflowDriverStatus{
			DriverID:   DRIVERID,
			DWDIndex:   0,
			WatchState: dwsv1alpha2.StateProposal,
			Status:     dwsv1alpha2.StatusPending,
			Completed:  false,
		}

		expectedDriverStatuses = []dwsv1alpha2.WorkflowDriverStatus{
			expectedDriverStatus,
		}
	})
})
