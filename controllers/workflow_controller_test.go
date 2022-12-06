package controllers

import (
	"context"

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

	It("Creates workflow", func() {
		Expect(k8sClient.Create(context.TODO(), wf)).To(Succeed())

		Eventually(func(g Gomega) string {
			g.Expect(k8sClient.Get(context.TODO(), client.ObjectKeyFromObject(wf), wf)).To(Succeed())
			return string(wf.Spec.DesiredState)
		}).Should(Equal(string(dwsv1alpha1.StateProposal)))

	})
})
