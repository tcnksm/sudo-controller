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

	"github.com/go-logr/logr"
	sudocontrollerv1 "github.com/tcnksm/sudo-controller/api/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TemporaryClusterRoleBindingReconciler reconciles a TemporaryClusterRoleBinding object
type TemporaryClusterRoleBindingReconciler struct {
	client.Client
	Log    logr.Logger
	scheme *runtime.Scheme
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
	logger.V(1).Info("get temporaryclusterrolebinding")
	if err := r.Get(ctx, req.NamespacedName, &temporaryClusterRoleBinding); err != nil {
		if !apierrs.IsNotFound(err) {
			logger.Error(err, "unable to fetch TemporaryClusterRoleBinding")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	var clusterRoleBinding rbacv1.ClusterRoleBinding
	bindingFound := true
	logger.V(1).Info("get clusterrolebinding")
	if err := r.Get(ctx, req.NamespacedName, &clusterRoleBinding); err != nil {
		if !apierrs.IsNotFound(err) {
			logger.Error(err, "unable to fetch ClusterRoleBinding")
			return ctrl.Result{}, err
		}
		bindingFound = false
	}

	if bindingFound {
		// annotations := clusterRoleBinding.Annotations
		// expiredAt, ok := annotations["expiredAt"]
		return ctrl.Result{}, nil
	}

	// TODO(tcnksm): Add approvement process

	// Create ClusterRoleBinding
	templ := temporaryClusterRoleBinding.DeepCopy()
	clusterRoleBinding.Name = req.Name
	clusterRoleBinding.Namespace = req.Namespace
	clusterRoleBinding.Annotations = map[string]string{
		"expiredAt": "Yo",
	}
	clusterRoleBinding.Subjects = templ.Subjects
	clusterRoleBinding.RoleRef = templ.RoleRef

	// SetControllerReference sets owner as a Controller OwnerReference on owned.
	// This is used for garbage collection of the owned object and for reconciling
	// the owner object on changes to owned (with a Watch + EnqueueRequestForOwner).
	// Since only one OwnerReference can be a controller, it returns an error
	// if there is another OwnerReference with Controller flag set.
	logger.V(1).Info("set controller references")
	if err := ctrl.SetControllerReference(&temporaryClusterRoleBinding, &clusterRoleBinding, r.scheme); err != nil {
		logger.Error(err, "unable to set clusterrolebinding owner reference")
		return ctrl.Result{}, err
	}

	logger.V(1).Info("create clusterrolebinding")
	if err := r.Create(ctx, &clusterRoleBinding); err != nil {
		logger.Error(err, "unable to create ClusterRoleBinding")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *TemporaryClusterRoleBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.scheme = mgr.GetScheme()
	return ctrl.NewControllerManagedBy(mgr).
		For(&sudocontrollerv1.TemporaryClusterRoleBinding{}).
		Owns(&rbacv1.ClusterRoleBinding{}).
		Complete(r)
}

func ignoreNotFound(err error) error {
	if apierrs.IsNotFound(err) {
		return nil
	}
	return err
}
