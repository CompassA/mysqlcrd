/*
 * @Author: Tomato
 * @Date: 2026-04-24 21:09:59
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

type MasterCreateStage struct{}

const (
	svcPortName          = "mysqlsvc"
	podPortName          = "mysql"
	mysqlConfVolumn      = "conf"
	mysqlConfigMapVolumn = "mysql-master-config"
	mysqlConfigMapPath   = "/mnt/configmap"
)

// 执行Reconcile
func (s *MasterCreateStage) Process(p *controller.StageParam) (res *ctrl.Result, err error) {
	// 更新状态
	defer func() {
		// 记录异常
		if err != nil {
			if setErr := p.Controller.SetCondition(p.Ctx, p.Cr, utils.MainStsReady, metav1.ConditionFalse, "create master failed", err.Error()); setErr != nil {
				p.Logger.Error(setErr, "set condition failed", "stage", s.Name())
			}
			return
		}
		// 什么都没返回代表当前阶段结束
		if res == nil {
			if setErr := p.Controller.SetCondition(p.Ctx, p.Cr, utils.MainStsReady, metav1.ConditionTrue, "create master failed", err.Error()); setErr != nil {
				p.Logger.Error(setErr, "set condition failed", "stage", s.Name())
			}
		} else {
			if setErr := p.Controller.SetCondition(p.Ctx, p.Cr, utils.MainStsReady, metav1.ConditionFalse, "waiting for master-creation to be completed", ""); setErr != nil {
				p.Logger.Error(setErr, "set condition failed", "stage", s.Name())
			}
		}
	}()

	// 创建Service
	if err := s.reconcileService(p); err != nil {
		return nil, err
	}

	// 创建主库statefulset
	if err := s.reconcileStatefulset(p); err != nil {
		return nil, err
	}

	// 查询主库statefulset创建状态, 未完成时结束本次reconcile, 等待下次reconcile
	if res, err := s.isStsReady(p); res != nil && err != nil {
		return res, err
	}

	return nil, nil
}

func (s *MasterCreateStage) reconcileService(p *controller.StageParam) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.ResourceName(p.Cr.Name, utils.MasterService),
			Namespace: p.Cr.Namespace,
		},
	}

	if op, err := controllerutil.CreateOrUpdate(p.Ctx, p.Controller.Client, svc, func() error {
		if err := controllerutil.SetControllerReference(p.Cr, svc, p.Controller.Scheme); err != nil {
			return err
		}

		svc.Spec = corev1.ServiceSpec{
			Selector: map[string]string{
				utils.AppLabel: utils.ResourceName(p.Cr.Name, utils.MasterPod),
			},
			ClusterIP: "None",
			Ports: []corev1.ServicePort{
				{
					Name:     fmt.Sprintf("%s-%s", p.Cr.Name, svcPortName),
					Protocol: corev1.ProtocolTCP,
					Port:     utils.MysqlServicePort,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: podPortName,
					},
				},
			},
		}

		return nil
	}); err != nil {
		return err
	} else {
		p.Logger.Info("master service reconciled", "CreateOrUpdateRes", op)
	}

	return nil
}

func (s *MasterCreateStage) reconcileStatefulset(p *controller.StageParam) error {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.ResourceName(p.Cr.Name, utils.MasterStatefulSet),
			Namespace: p.Cr.Namespace,
		},
	}

	if op, err := controllerutil.CreateOrUpdate(p.Ctx, p.Controller.Client, sts, func() error {
		if err := controllerutil.SetControllerReference(p.Cr, sts, p.Controller.Scheme); err != nil {
			return err
		}

		masterPVCName, masterPVC := createMasterPVC(p)         // pvc
		volumns := createPodVolumns(p, masterPVCName)          // volumn
		init := createInitContainer()                          // 初始化容器
		mysql := createMysqlContainer(p, masterPVCName)        // mysql容器
		xtrabackup := createSidecarContainer(p, masterPVCName) // xtrabackup sidecar容器

		replicas := int32(1)
		terminationGracePeriodSeconds := int64(60)
		masterPodName := utils.ResourceName(p.Cr.Name, utils.MasterPod)
		headlessSvcName := utils.ResourceName(p.Cr.Name, utils.MasterService)

		// POD应用标记
		podLabel := map[string]string{
			utils.AppLabel: masterPodName,
		}

		sts.Spec = appsv1.StatefulSetSpec{
			ServiceName:          headlessSvcName, // 绑定headlessservice
			Replicas:             &replicas,       // 主库pod数固定为1
			VolumeClaimTemplates: masterPVC,       // PVC

			// POD selector
			Selector: &metav1.LabelSelector{
				MatchLabels: podLabel,
			},

			// MysqlPOD
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabel,
				},
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

func createMasterPVC(p *controller.StageParam) (string, []corev1.PersistentVolumeClaim) {
	masterPVCName := utils.ResourceName(p.Cr.Name, utils.MasterPVC)

	strorageClassName := utils.DefaultStorageClass
	if p.Cr.Spec.StorageClassName != nil {
		strorageClassName = *p.Cr.Spec.StorageClassName
	}

	return masterPVCName, []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: masterPVCName,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				StorageClassName: &strorageClassName,
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: *p.Cr.Spec.Storage,
					},
				},
			},
		},
	}
}

func createPodVolumns(p *controller.StageParam, masterPVCName string) []corev1.Volume {
	cmName := utils.ResourceName(p.Cr.Name, utils.ConfigMap)
	return []corev1.Volume{
		// 配置数据卷
		{
			Name: mysqlConfVolumn,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},

		// ConfigMap数据卷
		{
			Name: mysqlConfigMapVolumn,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cmName,
					},
					Items: []corev1.KeyToPath{
						// 主库配置文件
						{
							Key:  utils.FileMasterConf,
							Path: utils.FileMasterConf,
						},
						// 创建主从复制账号的脚本
						{
							Key:  utils.FileCreateReplicaAccountProcedure,
							Path: utils.FileCreateReplicaAccountProcedure,
						},
						// 启动xtrabackup sidecar的脚本
						{
							Key:  utils.FileMasterSideCar,
							Path: utils.FileMasterSideCar,
						},
					},
				},
			},
		},
	}
}

func createInitContainer() *corev1.Container {
	return &corev1.Container{
		Name:  "init-conf",
		Image: utils.MysqlImage,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      mysqlConfVolumn,
				MountPath: utils.MysqlConfPath,
			},
			{
				Name:      mysqlConfigMapVolumn,
				MountPath: mysqlConfigMapPath,
			},
		},

		// 将cofigmap中的配置文件, 拷贝到mysql conf目录下
		Command: []string{
			"cp",
			fmt.Sprintf("%s/%s", mysqlConfigMapPath, utils.FileMasterConf),
			fmt.Sprintf("%s/my.cnf", utils.MysqlConfPath),
		},
	}
}

func createMysqlContainer(p *controller.StageParam, masterPVCName string) *corev1.Container {
	// root密码
	env := utils.EnvSecretRef(utils.ResourceName(p.Cr.Name, utils.Secret), []string{utils.EnvMysqlRootPassword})

	return &corev1.Container{
		Name:  "mysql",
		Image: utils.MysqlImage,
		Env:   env,
		Ports: []corev1.ContainerPort{
			{
				Name:          podPortName,
				ContainerPort: utils.MysqlServicePort,
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
						fmt.Sprintf("mysql -h 127.0.0.1 -uroot -p$%s -e \"SELECT 1\"", utils.EnvMysqlRootPassword),
					},
				},
			},
		},

		// 获取CR中配置的CPU与内存限制
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    *p.Cr.Spec.Cpu,
				corev1.ResourceMemory: *p.Cr.Spec.Memory,
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    *p.Cr.Spec.Cpu,
				corev1.ResourceMemory: *p.Cr.Spec.Memory,
			},
		},

		// 数据卷
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      mysqlConfVolumn,
				MountPath: utils.MysqlConfPath,
			},
			{
				Name:      masterPVCName,
				MountPath: utils.MysqlDataPath,
			},
			{
				Name:      mysqlConfigMapVolumn,
				MountPath: mysqlConfigMapPath,
			},
		},
	}
}

func createSidecarContainer(p *controller.StageParam, masterPVCName string) *corev1.Container {
	// root密码 主从复制账号密码
	env := utils.EnvSecretRef(utils.ResourceName(p.Cr.Name, utils.Secret),
		[]string{utils.EnvMysqlRootPassword, utils.EnvMysqlMasterDumpUser, utils.EnvMysqlMasterDumpPassword})

	return &corev1.Container{
		Name:  "xtrabackup",
		Image: utils.XtrabackupImage,
		Env:   env,
		Ports: []corev1.ContainerPort{
			{
				Name:          "xtrabackup",
				ContainerPort: utils.XtrabackupPort,
			},
		},

		// xtrabackup 占用的资源
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(utils.XtrabackupCpu),
				corev1.ResourceMemory: resource.MustParse(utils.XtrabackupMem),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse(utils.XtrabackupCpu),
				corev1.ResourceMemory: resource.MustParse(utils.XtrabackupMem),
			},
		},

		// 绑定数据卷
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      mysqlConfVolumn,
				MountPath: utils.MysqlConfPath,
			},
			{
				Name:      masterPVCName,
				MountPath: utils.MysqlDataPath,
			},
			{
				Name:      mysqlConfigMapVolumn,
				MountPath: mysqlConfigMapPath,
			},
		},
	}
}

func (*MasterCreateStage) isStsReady(p *controller.StageParam) (*ctrl.Result, error) {
	sts := &appsv1.StatefulSet{}
	if err := p.Controller.Client.Get(p.Ctx, types.NamespacedName{
		Namespace: p.Cr.Namespace,
		Name:      utils.ResourceName(p.Cr.Name, utils.MasterStatefulSet),
	}, sts); err != nil {
		return nil, err
	}

	if ready, msg := utils.StatefulSetReady(sts); !ready {
		p.Logger.Info("wait for master-stateful-set to be completed", "message", msg)
		return &ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	return nil, nil
}

// 阶段名称
func (s *MasterCreateStage) Name() string {
	return "CreateMasterStage"
}
