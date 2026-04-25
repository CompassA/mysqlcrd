/*
 * @Author: Tomato
 * @Date: 2026-04-25 22:09:30
 */
package pipeline

import (
	"fmt"
	"time"

	"github.com/mysqlcrd/internal/controller"
	"github.com/mysqlcrd/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ReplicaCreateStage struct{}

// 执行Reconcile
func (s *ReplicaCreateStage) Process(p *controller.StageParam) (res *ctrl.Result, err error) {
	// 没配置从库
	if p.Cr.Spec.Replica == nil {
		p.Logger.Info("no replica config, skip this stage", "stage", s.Name())
		return nil, nil
	}

	// 更新Controller状态
	defer func() {
		// 记录异常
		if err != nil {
			if setErr := p.Controller.SetCondition(p.Ctx, p.Cr, utils.ReplicaStsReady, metav1.ConditionFalse, "create replica failed", err.Error()); setErr != nil {
				p.Logger.Error(setErr, "set condition failed", "stage", s.Name())
			}
			return
		}
		// 什么都没返回代表当前阶段结束
		if res == nil {
			if setErr := p.Controller.SetCondition(p.Ctx, p.Cr, utils.ReplicaStsReady, metav1.ConditionTrue, "create replica failed", err.Error()); setErr != nil {
				p.Logger.Error(setErr, "set condition failed", "stage", s.Name())
			}
		} else {
			if setErr := p.Controller.SetCondition(p.Ctx, p.Cr, utils.ReplicaStsReady, metav1.ConditionFalse, "waiting for replica-creation to be completed", ""); setErr != nil {
				p.Logger.Error(setErr, "set condition failed", "stage", s.Name())
			}
		}
	}()

	// 从库POD标签
	label := map[string]string{
		utils.AppLabel:       utils.ResourceName(p.Cr.Name, utils.ReplicaPod),
		utils.MasterDNSLabel: utils.MasterServiceDNS(p.Cr.Name, p.Cr.Namespace),
	}

	// 创建从库 headless service
	if err := s.reconcileService(p, label); err != nil {
		return nil, err
	}

	// 创建从库 statefulset
	if err := s.reconcileStatefulSet(p, label); err != nil {
		return nil, err
	}

	// 检查从库statefulset创建状态
	if res, err := s.isReplicaStsReady(p); res != nil || err != nil {
		return res, err
	}

	return nil, nil
}

func (s *ReplicaCreateStage) reconcileService(p *controller.StageParam, label map[string]string) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.ResourceName(p.Cr.Name, utils.ReplicaService),
			Namespace: p.Cr.Namespace,
		},
	}

	if op, err := controllerutil.CreateOrUpdate(p.Ctx, p.Controller.Client, svc, func() error {
		if err := controllerutil.SetControllerReference(p.Cr, svc, p.Controller.Scheme); err != nil {
			return err
		}

		svc.Spec = corev1.ServiceSpec{
			Selector:  label,
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				{
					Name:     fmt.Sprintf("%s-mysql-replica-svc-port", p.Cr.Name),
					Protocol: corev1.ProtocolTCP,
					Port:     utils.MysqlServicePort,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: utils.MysqlPodPortName,
					},
				},
			},
		}

		return nil
	}); err != nil {
		return err
	} else {
		p.Logger.Info("replica service reconciled", "CreateOrUpdateRes", op)
	}

	return nil
}

func (s *ReplicaCreateStage) reconcileStatefulSet(p *controller.StageParam, label map[string]string) error {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.ResourceName(p.Cr.Name, utils.ReplicaStatefulSet),
			Namespace: p.Cr.Namespace,
		},
	}

	if op, err := controllerutil.CreateOrUpdate(p.Ctx, p.Controller.Client, sts, func() error {
		if err := controllerutil.SetControllerReference(p.Cr, sts, p.Controller.Scheme); err != nil {
			return err
		}

		crname := p.Cr.Name
		storage := p.Cr.Spec.Storage
		cpu := p.Cr.Spec.Cpu
		mem := p.Cr.Spec.Memory
		scn := p.Cr.Spec.StorageClassName

		pvcname, pvc := utils.CreatePVC(crname, scn, storage, utils.ReplicaPVC) // pvc
		volumns := createReplicaPodVolumns(p)                                   // volumn
		init := createReplicaInitContainer(p, label, pvcname)                   // 初始化容器
		mysql := utils.CreateMysqlContainer(crname, cpu, mem, pvcname)          // mysql容器
		xtrabackup := createReplicaSidecarContainer(p, label, pvcname)          // xtrabackup sidecar容器

		terminationGracePeriodSeconds := int64(60)

		sts.Spec = appsv1.StatefulSetSpec{
			ServiceName:          utils.ResourceName(crname, utils.ReplicaService), // 绑定headlessservice
			Replicas:             p.Cr.Spec.Replica.Size,                           // 从库pod数
			VolumeClaimTemplates: pvc,                                              // PVC

			// POD selector
			Selector: &metav1.LabelSelector{MatchLabels: label},

			// MysqlPOD
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: label},
				Spec: corev1.PodSpec{
					InitContainers:                []corev1.Container{*init},               // 初始化, 拷贝配置
					Containers:                    []corev1.Container{*mysql, *xtrabackup}, // mysql容器组
					Volumes:                       volumns,                                 // 数据卷
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
				},
			},
		}
		return nil
	}); err != nil {
		return err
	} else {
		p.Logger.Info("master statefulset reconciled", "CreateOrUpdateRes", op)
	}

	return nil
}

func createReplicaPodVolumns(p *controller.StageParam) []corev1.Volume {
	return []corev1.Volume{
		{ // 配置数据卷
			Name: utils.MysqlConfVolumn,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{ // ConfigMap数据卷
			Name: utils.MysqlConfigMapVolumn,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: utils.ResourceName(p.Cr.Name, utils.ConfigMap),
					},
					Items: []corev1.KeyToPath{
						// 从库配置文件
						{Key: utils.FileReplicaConf, Path: utils.FileReplicaConf},
						// 从库初始化镜像的脚本
						{Key: utils.FileReplicaInit, Path: utils.FileReplicaInit},
						// 启动主从复制脚本
						{Key: utils.FileStartReplicationProcedure, Path: utils.FileStartReplicationProcedure},
						// 从库xtrabackup sidecar脚本
						{Key: utils.FileReplicaSideCar, Path: utils.FileReplicaSideCar},
					},
				},
			},
		},
	}
}

func createReplicaInitContainer(p *controller.StageParam, label map[string]string, pvcname string) *corev1.Container {
	// root密码 主从复制账号密码 主库serviceDNS 从库Service名称
	env := utils.EnvSecretRef(utils.ResourceName(p.Cr.Name, utils.Secret), []string{utils.EnvMysqlRootPassword})
	env = append(env, corev1.EnvVar{
		Name: utils.EnvPodName,
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	}, corev1.EnvVar{
		Name: utils.EnvPodNamespace,
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	}, corev1.EnvVar{
		Name:  utils.EnvMasterDNS,
		Value: label[utils.MasterDNSLabel],
	}, corev1.EnvVar{
		Name:  utils.EnvReplicaServiceName,
		Value: utils.ResourceName(p.Cr.Name, utils.ReplicaService),
	})

	return &corev1.Container{
		Name:  "init-dump",
		Image: utils.XtrabackupImage,
		VolumeMounts: []corev1.VolumeMount{
			// mysql配置目录
			{Name: utils.MysqlConfVolumn, MountPath: utils.MysqlConfPath},
			// configmap配置文件
			{Name: utils.MysqlConfigMapVolumn, MountPath: utils.MysqlConfigMapPath},
			// mysql数据文件
			{Name: pvcname, MountPath: utils.MysqlDataPath},
		},
		Env: env,

		// 从库初始化脚本
		Command: []string{"bash", fmt.Sprintf("%s/%s", utils.MysqlConfigMapPath, utils.FileReplicaInit)},
	}
}

func createReplicaSidecarContainer(p *controller.StageParam, label map[string]string, pvcname string) *corev1.Container {
	// root密码 主从复制账号密码 主库url
	env := utils.EnvSecretRef(utils.ResourceName(p.Cr.Name, utils.Secret),
		[]string{utils.EnvMysqlRootPassword, utils.EnvMysqlMasterDumpUser, utils.EnvMysqlMasterDumpPassword})
	env = append(env, corev1.EnvVar{
		Name:  utils.EnvMasterDNS,
		Value: label[utils.MasterDNSLabel],
	})

	cpu := resource.MustParse(utils.XtrabackupCpu)
	mem := resource.MustParse(utils.XtrabackupMem)

	return &corev1.Container{
		Name:  "xtrabackup",
		Image: utils.XtrabackupImage,
		Env:   env,
		Ports: []corev1.ContainerPort{
			{Name: "xtrabackup", ContainerPort: utils.XtrabackupPort},
		},

		// xtrabackup 占用的资源
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    cpu,
				corev1.ResourceMemory: mem,
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    cpu,
				corev1.ResourceMemory: mem,
			},
		},

		// 绑定数据卷
		VolumeMounts: []corev1.VolumeMount{
			{Name: utils.MysqlConfVolumn, MountPath: utils.MysqlConfPath},
			{Name: utils.MysqlConfigMapVolumn, MountPath: utils.MysqlConfigMapPath},
			{Name: pvcname, MountPath: utils.MysqlDataPath},
		},

		// 启动sidecar脚本
		Command: []string{"bash", fmt.Sprintf("%s/%s", utils.MysqlConfigMapPath, utils.FileReplicaSideCar)},
	}
}

func (s *ReplicaCreateStage) isReplicaStsReady(p *controller.StageParam) (*ctrl.Result, error) {
	sts := &appsv1.StatefulSet{}
	if err := p.Controller.Client.Get(p.Ctx, types.NamespacedName{
		Namespace: p.Cr.Namespace,
		Name:      utils.ResourceName(p.Cr.Name, utils.ReplicaStatefulSet),
	}, sts); err != nil {
		return nil, err
	}

	if ready, msg := utils.StatefulSetReady(sts); !ready {
		p.Logger.Info("wait for replica-stateful-set to be completed", "message", msg)
		return &ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	return nil, nil
}

// 阶段名称
func (s *ReplicaCreateStage) Name() string {
	return "CreateMysqlReplica"
}
