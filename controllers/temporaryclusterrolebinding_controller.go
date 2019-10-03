/*

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
	"errors"
	"time"

	"github.com/go-logr/logr"
	sudocontrollerv1 "github.com/tcnksm/sudo-controller/api/v1"
	"github.com/tcnksm/sudo-controller/notify"
	"github.com/tcnksm/sudo-controller/slack"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TemporaryClusterRoleBindingReconciler reconciles a TemporaryClusterRoleBinding object
type TemporaryClusterRoleBindingReconciler struct {
	client.Client
	SlackClient   slack.Slack
	ApprovementCh chan notify.Approvement
	Log           logr.Logger
	Scheme        *runtime.Scheme
	Clock
}

// +kubebuilder:rbac:groups=sudocontroller.deeeet.com,resources=temporaryclusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sudocontroller.deeeet.com,resources=temporaryclusterrolebindings/status,verbs=get;update;patch

// What reconcile for TemporaryClusterRoleBinding does is:
//  1. Get TemporaryClusterRoleBinding
//  2. Get ClusterRoleBinding
//  3. If ClusterRoleBinding exists
//    a.Clean up TemporaryClusterRoleBinding if it's expired
//    b. Does nothing and returned if it's not expired
//  4. If ClusterRoleBinding does not exist
//    a. Ask approver
//    b. Create ClusterRoleBinding if it's approved
//    c. Delete TemporaryClusterRoleBinding if it's not approved
func (r *TemporaryClusterRoleBindingReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	logger := r.Log.WithValues("temporaryclusterrolebinding", req.NamespacedName)

	var temporaryClusterRoleBinding sudocontrollerv1.TemporaryClusterRoleBinding
	logger.V(0).Info("get temporaryclusterrolebinding")
	if err := r.Get(ctx, req.NamespacedName, &temporaryClusterRoleBinding); err != nil {
		if !apierrs.IsNotFound(err) {
			logger.Error(err, "unable to fetch TemporaryClusterRoleBinding")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	var clusterRoleBinding rbacv1.ClusterRoleBinding
	bindingFound := true
	logger.V(0).Info("get clusterrolebinding")
	if err := r.Get(ctx, req.NamespacedName, &clusterRoleBinding); err != nil {
		if !apierrs.IsNotFound(err) {
			logger.Error(err, "unable to fetch ClusterRoleBinding")
			return ctrl.Result{}, err
		}
		bindingFound = false
	}

	if bindingFound {
		logger.V(0).Info("check ClusterRoleBinding expiration")
		annotations := clusterRoleBinding.Annotations
		expiredAtStr, ok := annotations["expiredAt"]
		if !ok {
			return ctrl.Result{}, nil
		}

		expiredAt, err := time.Parse(time.RFC3339, expiredAtStr)
		if err != nil {
			logger.Error(err, "failed to parse expiredAt string to Time")
			return ctrl.Result{}, err
		}

		if r.Clock.Now().After(expiredAt) {
			logger.V(0).Info("delete TemporaryClusterRoleBinding")
			if err := r.Delete(ctx, &temporaryClusterRoleBinding); err != nil {
				logger.Error(err, "unable to delete TemporaryClusterRoleBinding")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ask aprovement
	if len(temporaryClusterRoleBinding.Subjects) > 1 {
		err := errors.New("invalid request")
		logger.Error(err, "only one subject can be binded")
		return ctrl.Result{}, err
	}

	// Get approvers
	var clusterApprover sudocontrollerv1.ClusterApprover
	logger.V(0).Info("get temporaryclusterrolebinding")
	if err := r.Get(ctx, types.NamespacedName{Name: temporaryClusterRoleBinding.Approver}, &clusterApprover); err != nil {
		return ctrl.Result{}, err
	}

	slackChannelID := clusterApprover.Spec.ChannelID
	approver := clusterApprover.Spec.ApproverIDs[0]
	applicant := temporaryClusterRoleBinding.Subjects[0].Name
	role := temporaryClusterRoleBinding.RoleRef.Name
	cluster := "gke_mercari-p-tcnksm_asia-northeast1-a_tcnksm-playground" // TODO(tcnksm): Get this dynamically
	if err := r.SlackClient.AskApprovement(req.Name, slackChannelID, approver, applicant, role, cluster); err != nil {
		logger.Error(err, "failed to ask approvement on slack")
		return ctrl.Result{}, err
	}

	var approvement notify.Approvement
Loop:
	for {
		select {
		case approvement = <-r.ApprovementCh:
			if approvement.Name != req.Name {
				r.ApprovementCh <- approvement
				<-time.After(1 * time.Second)
				break
			}
			break Loop
		}
	}

	// If it's not approved, delete temporary role binding itself
	if !approvement.Approve {
		if err := r.Delete(ctx, &temporaryClusterRoleBinding); err != nil {
			logger.Error(err, "unable to delete TemporaryClusterRoleBinding")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	expiredAfter, err := time.ParseDuration(temporaryClusterRoleBinding.TTL)
	if err != nil {
		logger.Error(err, "failed to parse TTL")
		return ctrl.Result{}, err
	}

	templ := temporaryClusterRoleBinding.DeepCopy()

	clusterRoleBinding.Name = req.Name
	clusterRoleBinding.Namespace = req.Namespace

	clusterRoleBinding.Subjects = templ.Subjects
	clusterRoleBinding.RoleRef = templ.RoleRef

	expiredAt := r.Clock.Now().Add(expiredAfter)
	clusterRoleBinding.Annotations = map[string]string{
		"expiredAt": expiredAt.Format(time.RFC3339),
	}

	// SetControllerReference sets owner as a Controller OwnerReference on owned.
	// This is used for garbage collection of the owned object and for reconciling
	// the owner object on changes to owned (with a Watch + EnqueueRequestForOwner).
	// Since only one OwnerReference can be a controller, it returns an error
	// if there is another OwnerReference with Controller flag set.
	logger.V(0).Info("set controller references")
	if err := ctrl.SetControllerReference(&temporaryClusterRoleBinding, &clusterRoleBinding, r.Scheme); err != nil {
		logger.Error(err, "unable to set clusterrolebinding owner reference")
		return ctrl.Result{}, err
	}

	logger.V(0).Info("create clusterrolebinding")
	if err := r.Create(ctx, &clusterRoleBinding); err != nil {
		logger.Error(err, "unable to create ClusterRoleBinding")
		return ctrl.Result{}, err
	}

	return ctrl.Result{
		RequeueAfter: expiredAfter,
	}, nil
}

func (r *TemporaryClusterRoleBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Scheme = mgr.GetScheme()
	r.Clock = &realClock{}
	return ctrl.NewControllerManagedBy(mgr).
		For(&sudocontrollerv1.TemporaryClusterRoleBinding{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Complete(r)
}

type realClock struct{}

func (_ realClock) Now() time.Time { return time.Now() }

// clock knows how to get the current time.
// It can be used to fake out timing for testing.
type Clock interface {
	Now() time.Time
}
