/*
 * @Author: Tomato
 * @Date: 2026-04-23 00:33:14
 */
package pipeline

import (
	"encoding/base64"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mysqlcrd/internal/controller"
	"github.com/mysqlcrd/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type ConfigMapStage struct {
	Files map[string]string
}

// 加载配置文件
func NewConfinMapStage(dir string) (*ConfigMapStage, error) {
	stage := &ConfigMapStage{
		Files: map[string]string{},
	}
	for _, name := range utils.FileNameArr {
		content, err := os.ReadFile(dir + name)
		if err != nil {
			return nil, fmt.Errorf("read file from %s%s failed, %w", dir, name, err)
		}
		stage.Files[name] = string(content)
	}
	return stage, nil
}

// 执行Reconcile, 创建
func (s *ConfigMapStage) Process(p *controller.StageParam) (res *ctrl.Result, err error) {
	defer func() {
		if err != nil {
			if setErr := p.Controller.SetCondition(p.Ctx, p.Cr, utils.ConfigReady, metav1.ConditionFalse, "Create failed", err.Error()); setErr != nil {
				p.Logger.Error(setErr, "set condition failed", "stage", s.Name())
			}
		}
	}()

	// create configmap
	if err := s.reconcileConfigmap(p); err != nil {
		return nil, err
	}

	// create secret
	if err := s.reconcileSecret(p); err != nil {
		return nil, err
	}

	// 标记config创建完成
	if err := p.Controller.SetCondition(p.Ctx, p.Cr, utils.ConfigReady, metav1.ConditionTrue, "Ready", ""); err != nil {
		p.Logger.Error(err, "set condition failed", "stage", s.Name())
	}

	return nil, nil
}

func (s *ConfigMapStage) reconcileSecret(p *controller.StageParam) (err error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.ResourceName(p.Cr.Name, utils.Secret),
			Namespace: p.Cr.Namespace,
		},
	}

	secretop, err := controllerutil.CreateOrUpdate(p.Ctx, p.Controller.Client, secret, func() error {
		if err := controllerutil.SetControllerReference(p.Cr, secret, p.Controller.Scheme); err != nil {
			return err
		}

		secret.Type = "Opaque"

		secret.Data = map[string][]byte{
			utils.EnvMysqlRootPassword:       []byte(base64.StdEncoding.EncodeToString([]byte(*p.Cr.Spec.Master.RootPassword))),
			utils.EnvMysqlMasterDumpUser:     []byte(base64.StdEncoding.EncodeToString([]byte(*p.Cr.Spec.Master.ReplicaAccount))),
			utils.EnvMysqlMasterDumpPassword: []byte(base64.StdEncoding.EncodeToString([]byte(*p.Cr.Spec.Master.ReplicaPassword))),
		}

		return nil
	})
	if err != nil {
		return err
	}

	p.Logger.Info("Secret reconciled", "operation", secretop)
	return nil
}

func (s *ConfigMapStage) reconcileConfigmap(p *controller.StageParam) (err error) {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.ResourceName(p.Cr.Name, utils.ConfigMap),
			Namespace: p.Cr.Namespace,
		},
	}

	cmop, err := controllerutil.CreateOrUpdate(p.Ctx, p.Controller.Client, cm, func() error {
		if err := controllerutil.SetControllerReference(p.Cr, cm, p.Controller.Scheme); err != nil {
			return err
		}

		cm.Data = s.Files

		return nil
	})
	if err != nil {
		return err
	}
	p.Logger.Info("ConfigMap reconciled", "operation", cmop)

	return nil
}

// 阶段名称
func (s *ConfigMapStage) Name() string {
	return "ConfigMapStage"
}
