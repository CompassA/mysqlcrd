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

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	tomatov1 "github.com/mysqlcrd/api/v1"
	v1 "github.com/mysqlcrd/api/v1"
	"github.com/mysqlcrd/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MySQLReconciler reconciles a MySQL object
type MySQLReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	Pipelines []OperatorStage
}

// 执行阶段
type OperatorStage interface {
	// 执行Reconcile
	Process(param *StageParam) (*ctrl.Result, error)

	// 阶段名称
	Name() string
}

type StageParam struct {
	Controller *MySQLReconciler
	Ctx        context.Context
	Req        *ctrl.Request
	Cr         *v1.MySQL
	Logger     *logr.Logger
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
func (r *MySQLReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := logf.FromContext(ctx)

	// 获取CR
	cr := &tomatov1.MySQL{}
	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 设置状态
	if err := r.SetCondition(ctx, cr, utils.ReconcileProcessing, metav1.ConditionTrue, "Reconciling", ""); err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		// 出现错误
		if err != nil {
			if setErr := r.SetCondition(ctx, cr, utils.ReconcileProcessing, metav1.ConditionTrue, "error", err.Error()); setErr != nil {
				logger.Error(setErr, "set condition failed")
			}
			if setErr := r.SetCondition(ctx, cr, utils.ConfigReady, metav1.ConditionFalse, "error", err.Error()); setErr != nil {
				logger.Error(setErr, "set condition failed")
			}
			return
		}

		// 等待资源就绪
		if result.RequeueAfter > 0 {
			if setErr := r.SetCondition(ctx, cr, utils.ReconcileProcessing, metav1.ConditionTrue, "Retrying", ""); setErr != nil {
				logger.Error(setErr, "set condition failed")
			}

			if setErr := r.SetCondition(ctx, cr, utils.ConfigReady, metav1.ConditionFalse, "Retrying", ""); setErr != nil {
				logger.Error(setErr, "set condition failed")
			}
			return
		}

		// Reconcile完成
		if setErr := r.SetCondition(ctx, cr, utils.ReconcileProcessing, metav1.ConditionFalse, "succeed", ""); setErr != nil {
			logger.Error(setErr, "set condition failed")
		}
		if setErr := r.SetCondition(ctx, cr, utils.ConfigReady, metav1.ConditionTrue, "Ready", ""); setErr != nil {
			logger.Error(setErr, "set condition failed")
		}
	}()

	// 执行具体逻辑
	p := &StageParam{
		Controller: r,
		Ctx:        ctx,
		Req:        &req,
		Cr:         cr,
		Logger:     &logger,
	}
	for _, stage := range r.Pipelines {
		// stage返回了err时, 处理err, 流程中止
		// stage返回了result时, 直接将result作为本次Reconcile的结果, 流程中止
		// stage什么都没返回时, 继续执行
		result, err := stage.Process(p)
		if err != nil {
			logger.Error(err, "operate stage failed", "stage", stage.Name())
			return *result, err
		}
		if result != nil {
			return *result, nil
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MySQLReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tomatov1.MySQL{}).
		Named("mysql").
		Complete(r)
}

// 设置Condition
func (r *MySQLReconciler) SetCondition(ctx context.Context, cr *tomatov1.MySQL, condType string, status metav1.ConditionStatus, reason string, message string) error {
	newCond := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: cr.Generation,
	}

	cur := meta.FindStatusCondition(cr.Status.Conditions, condType)
	if cur != nil && cur.Status == status && cur.Reason == reason && cur.Message == message {
		return nil
	}

	meta.SetStatusCondition(&cr.Status.Conditions, newCond)

	return r.Status().Update(ctx, cr)
}
