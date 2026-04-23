/*
 * @Author: Tomato
 * @Date: 2026-04-23 22:04:21
 */
package utils

// Reconcile Condition Type
const (
	ReconcileProcessing = "ReconcileProcessing" // 处理中
	ConfigReady         = "ConfigReady"         // 创建ConfigMap
	MainStsReady        = "MainStsReady"        // 创建主库StatefulSet
	ReplicaStsReady     = "ReplicaStsReady"     // 创建从库StatefulSet
	Ready               = "Ready"               // 资源整体编排完成
)
