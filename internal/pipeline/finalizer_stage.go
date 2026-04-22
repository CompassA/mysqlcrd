/*
 * @Author: Tomato
 * @Date: 2026-04-23 00:18:50
 */
package pipeline

import (
	"github.com/mysqlcrd/internal/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const finalizer string = "delete-pvc-finalizer-mark"

type FinalizerStage struct{}

// 资源被删除后, 处理关联PVC的删除
func (s *FinalizerStage) Process(p *controller.StageParam) (*ctrl.Result, error) {
	// 没被删除时, 检查删除标记是否添加
	if p.Cr.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(p.Cr, finalizer) {
			controllerutil.AddFinalizer(p.Cr, finalizer)
			if err := p.R.Update(*p.Ctx, p.Cr); err != nil {
				return &reconcile.Result{}, err
			}
		}
		return nil, nil
	}

	// fixme TOMATO todo 删除CRD
	return nil, nil
}

// 阶段名称
func (s *FinalizerStage) Name() string {
	return "FinalizerStage"
}
