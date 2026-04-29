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
	"encoding/json"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	tomatov1 "github.com/mysqlcrd/api/v1"
	v1 "github.com/mysqlcrd/api/v1"

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
	Process(p *StageParam) (*ctrl.Result, error)

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
// +kubebuilder:rbac:groups="",resources=pods;services;configmaps;secrets;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch;create;update;patch;delete

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

	crjson, _ := json.Marshal(*cr)
	logger.Info("mysql reconsile start", "cr", string(crjson))

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
			return ctrl.Result{RequeueAfter: time.Minute}, nil
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
	latest := &tomatov1.MySQL{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(cr), latest); err != nil {
		return err
	}

	newCond := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
		ObservedGeneration: latest.Generation,
	}

	cur := meta.FindStatusCondition(latest.Status.Conditions, condType)
	if cur != nil && cur.Status == status && cur.Reason == reason && cur.Message == message {
		return nil
	}

	patch := client.MergeFrom(latest.DeepCopy())
	meta.SetStatusCondition(&latest.Status.Conditions, newCond)
	return r.Status().Patch(ctx, latest, patch)
}
