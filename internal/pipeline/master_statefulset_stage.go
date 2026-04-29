/*
 * @Author: Tomato
 * @Date: 2026-04-24 21:09:59
 */
package pipeline

import (
	"fmt"
	"strconv"
	"time"

	myctrl "github.com/mysqlcrd/internal/controller"
	"github.com/mysqlcrd/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type MasterCreateStage struct{}

// 执行Reconcile
func (s *MasterCreateStage) Process(p *myctrl.StageParam) (res *ctrl.Result, err error) {
	// 应用标记
	label := map[string]string{
		myctrl.AppLabel: myctrl.ResourceName(p.Cr.Name, myctrl.MasterPod),
	}

	// 创建Service
	if err := s.reconcileService(p, label); err != nil {
		return nil, err
	}

	// 创建主库statefulset
	if err := s.reconcileStatefulset(p, label); err != nil {
		return nil, err
	}

	// 查询主库statefulset创建状态, 未完成时结束本次reconcile, 等待下次reconcile
	if res, err := s.isStsReady(p); res != nil || err != nil {
		return res, err
	}

	return nil, nil
}

func (s *MasterCreateStage) reconcileService(p *myctrl.StageParam, label map[string]string) error {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      myctrl.ResourceName(p.Cr.Name, myctrl.MasterService),
			Namespace: p.Cr.Namespace,
		},
	}

	if op, err := controllerutil.CreateOrPatch(p.Ctx, p.Controller.Client, svc, func() error {
		if err := controllerutil.SetControllerReference(p.Cr, svc, p.Controller.Scheme); err != nil {
			return err
		}

		if svc.CreationTimestamp.IsZero() {
			svc.Spec = corev1.ServiceSpec{
				Selector:  label,
				ClusterIP: "None",
				Ports: []corev1.ServicePort{
					{
						Name:     fmt.Sprintf("%s-mysqlsvcport", p.Cr.Name),
						Protocol: corev1.ProtocolTCP,
						Port:     myctrl.MysqlServicePort,
						TargetPort: intstr.IntOrString{
							Type:   intstr.String,
							StrVal: myctrl.MysqlPodPortName,
						},
					},
				},
			}
		}

		return nil
	}); err != nil {
		return err
	} else {
		p.Logger.Info("master service reconciled", "CreateOrPatchRes", op)
	}

	return nil
}

func (s *MasterCreateStage) reconcileStatefulset(p *myctrl.StageParam, label map[string]string) error {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      myctrl.ResourceName(p.Cr.Name, myctrl.MasterStatefulSet),
			Namespace: p.Cr.Namespace,
		},
	}

	if op, err := controllerutil.CreateOrPatch(p.Ctx, p.Controller.Client, sts, func() error {
		if err := controllerutil.SetControllerReference(p.Cr, sts, p.Controller.Scheme); err != nil {
			return err
		}

		crname := p.Cr.Name
		storage := p.Cr.Spec.Storage
		cpu := p.Cr.Spec.Cpu
		mem := p.Cr.Spec.Memory
		scn := p.Cr.Spec.StorageClassName
		semisync := p.Cr.Spec.Master.Semisync

		pvcname, pvc := myctrl.CreatePVC(crname, scn, storage, myctrl.MasterPVC) // pvc
		mysql := myctrl.CreateMysqlContainer(crname, cpu, mem, pvcname)          // mysql容器
		xtrabackup := createSidecarContainer(p, pvcname, semisync)               // xtrabackup sidecar容器

		// 首次
		if sts.CreationTimestamp.IsZero() {
			replicas := int32(1)
			terminationGracePeriodSeconds := int64(60)
			volumns := createPodVolumns(p) // volumn
			init := createInitContainer()  // 初始化容器

			sts.Spec = appsv1.StatefulSetSpec{
				ServiceName:          myctrl.ResourceName(crname, myctrl.MasterService), // 绑定headlessservice
				Replicas:             &replicas,                                         // 主库pod数固定为1
				VolumeClaimTemplates: pvc,                                               // PVC
				Selector:             &metav1.LabelSelector{MatchLabels: label},         // POD selector
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: label},
					Spec: corev1.PodSpec{
						InitContainers:                []corev1.Container{*init},               // 初始化, 拷贝配置
						Containers:                    []corev1.Container{*mysql, *xtrabackup}, // mysql容器组
						Volumes:                       volumns,                                 // 数据卷
						TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					},
				},
				// PVC保留策略, statefulset删除/缩容后直接删除PVC
				PersistentVolumeClaimRetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
					WhenDeleted: appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
					WhenScaled:  appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
				},
			}
			return nil
		}

		// 更新
		for i, container := range sts.Spec.Template.Spec.Containers {
			switch container.Name {
			case mysql.Name:
				// 更新mysql资源配置
				sts.Spec.Template.Spec.Containers[i].Resources = mysql.Resources
			case xtrabackup.Name:
				// 更新半同步配置
				sts.Spec.Template.Spec.Containers[i].Env = xtrabackup.Env
			}
		}
		return nil
	}); err != nil {
		return err
	} else {
		p.Logger.Info("master statefulset reconciled", "CreateOrPatchRes", op)
	}

	return nil
}

func createPodVolumns(p *myctrl.StageParam) []corev1.Volume {
	return []corev1.Volume{
		{ // 配置数据卷
			Name: myctrl.MysqlConfVolumn,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
		{ // ConfigMap数据卷
			Name: myctrl.MysqlConfigMapVolumn,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: myctrl.ResourceName(p.Cr.Name, myctrl.ConfigMap),
					},
					Items: []corev1.KeyToPath{
						// 主库配置文件
						{Key: myctrl.FileMasterConf, Path: myctrl.FileMasterConf},
						// 创建主从复制账号的脚本
						{Key: myctrl.FileCreateReplicaAccountProcedure, Path: myctrl.FileCreateReplicaAccountProcedure},
						// 启动xtrabackup sidecar的脚本
						{Key: myctrl.FileMasterSideCar, Path: myctrl.FileMasterSideCar},
					},
				},
			},
		},
	}
}

func createInitContainer() *corev1.Container {
	// 将cofigmap中的配置文件, 拷贝到mysql conf目录下
	srcconf := fmt.Sprintf("%s/%s", myctrl.MysqlConfigMapPath, myctrl.FileMasterConf)
	dstconf := fmt.Sprintf("%s/my.cnf", myctrl.MysqlConfPath)
	return &corev1.Container{
		Name:    "init-conf",
		Image:   myctrl.MysqlImage,
		Command: []string{"cp", srcconf, dstconf},
		VolumeMounts: []corev1.VolumeMount{
			{Name: myctrl.MysqlConfVolumn, MountPath: myctrl.MysqlConfPath},
			{Name: myctrl.MysqlConfigMapVolumn, MountPath: myctrl.MysqlConfigMapPath},
		},
	}
}

func createSidecarContainer(p *myctrl.StageParam, masterPVCName string, semisync *int32) *corev1.Container {
	// root密码 主从复制账号密码
	env := myctrl.EnvSecretRef(myctrl.ResourceName(p.Cr.Name, myctrl.Secret),
		[]string{myctrl.EnvMysqlRootPassword, myctrl.EnvMysqlMasterDumpUser, myctrl.EnvMysqlMasterDumpPassword})
	env = append(env, corev1.EnvVar{
		Name:  myctrl.EnvSemisync,
		Value: strconv.Itoa(int(*semisync)),
	})

	cpu := resource.MustParse(myctrl.XtrabackupCpu)
	mem := resource.MustParse(myctrl.XtrabackupMem)

	sidecarsh := fmt.Sprintf("%s/%s", myctrl.MysqlConfigMapPath, myctrl.FileMasterSideCar)

	return &corev1.Container{
		Name:  "xtrabackup",
		Image: myctrl.XtrabackupImage,
		Env:   env,
		Ports: []corev1.ContainerPort{
			{Name: "xtrabackup", ContainerPort: myctrl.XtrabackupPort},
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
			{Name: myctrl.MysqlConfVolumn, MountPath: myctrl.MysqlConfPath},
			{Name: myctrl.MysqlConfigMapVolumn, MountPath: myctrl.MysqlConfigMapPath},
			{Name: masterPVCName, MountPath: myctrl.MysqlDataPath},
		},

		// 启动sidecar脚本
		Command: []string{"bash", sidecarsh},
	}
}

func (*MasterCreateStage) isStsReady(p *myctrl.StageParam) (*ctrl.Result, error) {
	sts := &appsv1.StatefulSet{}
	if err := p.Controller.Client.Get(p.Ctx, types.NamespacedName{
		Namespace: p.Cr.Namespace,
		Name:      myctrl.ResourceName(p.Cr.Name, myctrl.MasterStatefulSet),
	}, sts); err != nil {
		if apierrors.IsNotFound(err) {
			return &ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
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
	return "CreateMySQLMaster"
}
