/*
 * @Author: Tomato
 * @Date: 2026-03-30 00:38:27
 */
/*
Copyright 2026.

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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	tomatov1 "github.com/mysqlcrd/api/v1"
	tomatopipe "github.com/mysqlcrd/internal/pipeline"
)

// MySQLReconciler reconciles a MySQL object
type MySQLReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Pipelines []tomatopipe.OperatorStage
}

// +kubebuilder:rbac:groups=tomato.github.com,resources=mysqls,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=tomato.github.com,resources=mysqls/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=tomato.github.com,resources=mysqls/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MySQL object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/reconcile
func (r *MySQLReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	// 执行具体逻辑
	for _, stage := range r.Pipelines {
		// stage返回了err时, 处理err, 流程中止
		// stage返回了result时, 直接将result作为本次Reconcile的结果, 流程中止
		// stage什么都没返回时, 继续执行
		result, err := stage.Process(ctx, &req)
		if err != nil {
			logger.Error(err, "operate stage failed", "stage", stage.Name())
			return *result, err
		}
		if result != nil {
			return *result, nil
		}
	}

	// todo 记录status

	// 返回
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MySQLReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tomatov1.MySQL{}).
		Named("mysql").
		Complete(r)
}
