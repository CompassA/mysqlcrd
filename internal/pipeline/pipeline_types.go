/*
 * @Author: Tomato
 * @Date: 2026-04-21 01:14:07
 */
package pipeline

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
)

// 处理资源创建
type OperatorStage interface {
	// 执行Reconcile
	Process(ctx context.Context, req *ctrl.Request) (*ctrl.Result, error)

	// 阶段名称
	Name() string
}
