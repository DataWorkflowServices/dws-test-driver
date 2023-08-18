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
	"strings"

	dwsv1alpha2 "github.com/HewlettPackard/dws/api/v1alpha2"
	dwdparse "github.com/HewlettPackard/dws/utils/dwdparse"
	"github.com/HewlettPackard/dws/utils/updater"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const DRIVERID string = "tester"

// WorkflowReconciler reconciles a Workflow object
type WorkflowReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=dws.cray.hpe.com,resources=workflows,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Workflow object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *WorkflowReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, err error) {
	log := r.Log.WithValues("Workflow", req.NamespacedName)
	log.Info("Reconciling Workflow")

	// Fetch the Workflow workflow
	workflow := &dwsv1alpha2.Workflow{}
	if err := r.Get(ctx, req.NamespacedName, workflow); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// The workflow is being deleted. Nothing to do
	if !workflow.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, nil
	}

	// Nothing to do
	if workflow.Status.Ready {
		return ctrl.Result{}, nil
	}

	// Transitioning states. Nothing to do
	if workflow.Status.State != workflow.Spec.DesiredState {
		return ctrl.Result{}, nil
	}

	log.Info("Reconciling Workflow Driver Statuses")
	desiredState := workflow.Spec.DesiredState
	directives := workflow.Spec.DWDirectives

	// Create a status updater that handles the call to r.Update() if any of the fields
	// in workflow.Status{} change. This is necessary since Status is not a subresource
	// of the workflow.
	statusUpdater := updater.NewStatusUpdater[*dwsv1alpha2.WorkflowStatus](workflow)
	defer func() { err = statusUpdater.CloseWithUpdate(ctx, r, err) }()

	// Check workflow for test driver entries
	for driverStatusIndex, driverStatus := range workflow.Status.Drivers {

		// Skip driverStatus entries of other drivers
		if DRIVERID != driverStatus.DriverID {
			continue
		}

		// Skip driverStatus entries that aren't relevant
		if desiredState != driverStatus.WatchState {
			continue
		}

		// Skip driverStatus entries that have already completed
		if driverStatus.Completed {
			continue
		}

		// Skip driverStatus entries with recorded errors
		if dwsv1alpha2.StatusError == driverStatus.Status {
			continue
		}

		var directive = directives[driverStatus.DWDIndex]
		args, err := dwdparse.BuildArgsMap(directive)
		if err != nil {
			log.Error(err, "Could not parse driver args from directive", "directive", directive)
			return ctrl.Result{}, err
		}

		switch {
		case args["action"] == "complete":
			log.Info("Completing workflow")
			driverStatus.Completed = true
			driverStatus.Status = dwsv1alpha2.StatusCompleted
			ct := metav1.NowMicro()
			driverStatus.CompleteTime = &ct
		case args["action"] == "wait":
			// The driver status will be marked complete by external process
			// Nothing to do
			log.Info("Driver waiting on external completion", "desired_state", desiredState)
			continue
		case args["action"] == "error":
			log.Info("Failing workflow")
			driverStatus.Message = "Reported error: " + args["message"]
			// Errors are found on the #DW line with
			// underscores representing spaces, which allows the
			// #DW parser to be simple; the controller will swap
			// those back to spaces.
			driverStatus.Error = strings.ReplaceAll(args["message"], "_", " ")

			var severity string
			var present bool
			severity, present = args["severity"]
			if !present {
				severity = ""
			}
			status, err := dwsv1alpha2.SeverityStringToStatus(severity)
			if err != nil {
				driverStatus.Status = dwsv1alpha2.StatusError
				driverStatus.Message = "Internal error: " + err.Error()
				driverStatus.Error = err.Error()
			} else {
				driverStatus.Status = status
			}

		default:
			log.Error(err, "Unsupported action in directive", "directive", directive)
			return ctrl.Result{}, err
		}

		workflow.Status.Drivers[driverStatusIndex] = driverStatus
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkflowReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dwsv1alpha2.Workflow{}).
		Complete(r)
}
