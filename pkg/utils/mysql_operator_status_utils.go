/*
 * @Author: Tomato
 * @Date: 2026-04-23 22:04:21
 */
package utils

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/resource"
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
func ResourceName(crname string, tp CrdResourceType) string {
	switch tp {
	case ConfigMap:
		return fmt.Sprintf("%s-configmap", crname)
	case Secret:
		return fmt.Sprintf("%s-secret", crname)
	case MasterService:
		return fmt.Sprintf("%s-master-service", crname)
	case ReplicaService:
		return fmt.Sprintf("%s-replica-service", crname)
	case MasterStatefulSet:
		return fmt.Sprintf("%s-master-sts", crname)
	case ReplicaStatefulSet:
		return fmt.Sprintf("%s-replica-sts", crname)
	case MasterPod:
		return fmt.Sprintf("%s-master", crname)
	case ReplicaPod:
		return fmt.Sprintf("%s-replica", crname)
	case MasterPVC:
		return fmt.Sprintf("%s-master-pvc", crname)
	case ReplicaPVC:
		return fmt.Sprintf("%s-replica-pvc", crname)
	}
	return ""
}

const (
	EnvMysqlRootPassword       = "MYSQL_ROOT_PASSWORD"        // 环境变量 mysqlroot密码
	EnvMysqlMasterDumpUser     = "MYSQL_MASTER_DUMP_USER"     // 环境变量 mysql复制账号
	EnvMysqlMasterDumpPassword = "MYSQL_MASTER_DUMP_PASSWORD" // 环境变量 mysql复制账号密码
	EnvPodName                 = "PODNAME"                    // pod名称
	EnvPodNamespace            = "POD_NAMESPACE"              // podnamspace
	EnvMasterDNS               = "MASTER_SERVICE"             // 主库headless service DNS
	EnvReplicaServiceName      = "REPLICA_SERVICE_NAME"       // 从库headless service的名称

	AppLabel       = "app"                            // pod "app"标签
	MasterDNSLabel = "tomatocrd/mysql/master-service" // 主库service

	MysqlImage           = "mysql:8.4.7"       // mysql镜像版本
	MysqlServicePort     = 3306                // mysql暴露的端口
	MysqlPodPortName     = "mysqlport"         // mysql pod port配置名称
	MysqlConfPath        = "/etc/mysql/conf.d" // mysql镜像配置文件地址
	MysqlDataPath        = "/var/lib/mysql"    // mysql镜像数据文件地址
	MysqlConfVolumn      = "conf"              // mysql配置目录数据卷名称
	MysqlConfigMapVolumn = "mysql-config"      // mysql configmap数据卷名称
	MysqlConfigMapPath   = "/mnt/configmap"    // mysql configmap挂载目录

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

func EnvSecretRef(secretName string, envKeys []string) []corev1.EnvVar {
	res := make([]corev1.EnvVar, len(envKeys))

	for i, envKey := range envKeys {
		res[i] = corev1.EnvVar{
			Name: envKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: envKey,
				},
			},
		}
	}

	return res
}

func CreatePVC(crname string, scn *string, storage *resource.Quantity, tp CrdResourceType) (string, []corev1.PersistentVolumeClaim) {
	pvcname := ResourceName(crname, tp)

	strorageClassName := DefaultStorageClass
	if scn != nil {
		strorageClassName = *scn
	}

	return pvcname, []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{Name: pvcname},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: &strorageClassName,
				AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: *storage,
					},
				},
			},
		},
	}
}

func CreateMysqlContainer(crname string, cpu *resource.Quantity, mem *resource.Quantity, pvcname string) *corev1.Container {
	// root密码
	env := EnvSecretRef(ResourceName(crname, Secret), []string{EnvMysqlRootPassword})

	return &corev1.Container{
		Name:  "mysql",
		Image: MysqlImage,
		Env:   env,
		Ports: []corev1.ContainerPort{
			{
				Name:          MysqlPodPortName,
				ContainerPort: MysqlServicePort,
			},
		},
		LivenessProbe: &corev1.Probe{
			InitialDelaySeconds: 30,
			PeriodSeconds:       20,
			TimeoutSeconds:      10,
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"mysqladmin", "ping"},
				},
			},
		},
		ReadinessProbe: &corev1.Probe{
			InitialDelaySeconds: 30,
			PeriodSeconds:       20,
			TimeoutSeconds:      10,
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"bash", "\"-c\"",
						fmt.Sprintf("mysql -h 127.0.0.1 -uroot -p$%s -e \"SELECT 1\"", EnvMysqlRootPassword),
					},
				},
			},
		},

		// CPU与内存限制
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    *cpu,
				corev1.ResourceMemory: *mem,
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    *cpu,
				corev1.ResourceMemory: *mem,
			},
		},

		// 数据卷
		VolumeMounts: []corev1.VolumeMount{
			// 配置文件
			{Name: MysqlConfVolumn, MountPath: MysqlConfPath},
			// ConfigMap配置
			{Name: MysqlConfigMapVolumn, MountPath: MysqlConfigMapPath},
			// 绑定pvc
			{Name: pvcname, MountPath: MysqlDataPath},
		},
	}
}

func MasterServiceDNS(crname, crnamespace string) string {
	return fmt.Sprintf("%s-0.%s.%s",
		ResourceName(crname, MasterPod), ResourceName(crname, MasterService), crnamespace)
}
