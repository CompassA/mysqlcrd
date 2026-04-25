/*
 * @Author: Tomato
 * @Date: 2026-04-23 22:04:21
 */
package utils

import (
	"fmt"
)

// Reconcile Condition Type
const (
	ReconcileProcessing = "ReconcileProcessing" // 处理中
	ConfigReady         = "ConfigReady"         // 创建ConfigMap
	MainStsReady        = "MainStsReady"        // 创建主库StatefulSet
	ReplicaStsReady     = "ReplicaStsReady"     // 创建从库StatefulSet
	Ready               = "Ready"               // 资源整体编排完成
)

// Operator会创建的资源类型
type CrdResourceType int

const (
	ConfigMap          CrdResourceType = iota // MySQL配置
	Secret                                    // 账户信息
	MasterService                             // 主库服务
	MasterStatefulSet                         // 主库
	MasterPod                                 // 主库Pod
	ReplicaService                            // 从库服务
	ReplicaStatefulSet                        // 从库
	ReplicaPod                                // 从库Pod
	MasterPVC                                 // 主库PVC
	ReplicaPVC                                // 从库PVC
)

// 根据MySQL资源的名称, 拼接子资源的名称
func ResourceName(crName string, tp CrdResourceType) string {
	switch tp {
	case ConfigMap:
		return fmt.Sprintf("%s-configmap", crName)
	case Secret:
		return fmt.Sprintf("%s-secret", crName)
	case MasterService:
		return fmt.Sprintf("%s-master-service", crName)
	case ReplicaService:
		return fmt.Sprintf("%s-replica-service", crName)
	case MasterStatefulSet:
		return fmt.Sprintf("%s-master-sts", crName)
	case ReplicaStatefulSet:
		return fmt.Sprintf("%s-replica-sts", crName)
	case MasterPod:
		return fmt.Sprintf("%s-master", crName)
	case ReplicaPod:
		return fmt.Sprintf("%s-replica", crName)
	case MasterPVC:
		return fmt.Sprintf("%s-master-pvc", crName)
	case ReplicaPVC:
		return fmt.Sprintf("%s-replica-pvc", crName)
	}
	return ""
}

const (
	EnvMysqlRootPassword       = "MYSQL_ROOT_PASSWORD"        // 环境变量 mysqlroot密码
	EnvMysqlMasterDumpUser     = "MYSQL_MASTER_DUMP_USER"     // 环境变量 mysql复制账号
	EnvMysqlMasterDumpPassword = "MYSQL_MASTER_DUMP_PASSWORD" // 环境变量 mysql复制账号密码

	AppLabel         = "app"               // pod "app"标签
	MysqlImage       = "mysql:8.4.7"       // mysql镜像版本
	MysqlServicePort = 3306                // mysql暴露的端口
	MysqlConfPath    = "/etc/mysql/conf.d" // mysql镜像配置文件地址
	MysqlDataPath    = "/var/lib/mysql"    // mysql镜像数据文件地址

	XtrabackupImage = "localhost/my-xtrabackup:8.4" // xtrabackup
	XtrabackupCpu   = "100m"                        // xtrabackup sidecar占用的CPU
	XtrabackupMem   = "128Mi"                       // xtrabackup sidecar占用的内存
	XtrabackupPort  = 3307                          // xtrabackup sidecar开放的端口

	DefaultStorageClass = "standard" // 默认的存储类型

	// 放入configmap的配置文件名
	FileCreateReplicaAccountProcedure = "create_replication_account_procedure.sql"
	FileStartReplicationProcedure     = "start_replication_procedure.sql"
	FileMasterSideCar                 = "master_sidecar.sh"
	FileReplicaSideCar                = "replica_sidecar.sh"
	FileReplicaInit                   = "replica_init.sh"
	FileMasterConf                    = "master.cnf"
	FileReplicaConf                   = "replica.cnf"
)

var FileNameArr = [...]string{
	FileCreateReplicaAccountProcedure,
	FileStartReplicationProcedure,
	FileMasterSideCar,
	FileReplicaSideCar,
	FileReplicaInit,
	FileMasterConf,
	FileReplicaConf,
}
