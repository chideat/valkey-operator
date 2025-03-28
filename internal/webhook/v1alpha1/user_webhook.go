/*
Copyright 2024 chideat.

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

package v1alpha1

import (
	"context"
	"fmt"
	"slices"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/api/v1alpha1"
	valkeybufredv1alpha1 "github.com/chideat/valkey-operator/api/v1alpha1"
	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/config"
	"github.com/chideat/valkey-operator/internal/util"
	security "github.com/chideat/valkey-operator/pkg/security/password"
	"github.com/chideat/valkey-operator/pkg/types/user"
)

// nolint:unused
// log is for logging in this package.
var logger = logf.Log.WithName("user-resource")

// SetupUserWebhookWithManager registers the webhook for User in the manager.
func SetupUserWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&valkeybufredv1alpha1.User{}).
		WithValidator(&UserCustomValidator{}).
		WithDefaulter(&UserCustomDefaulter{}).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-valkey-buf-red-v1alpha1-user,mutating=true,failurePolicy=fail,sideEffects=None,groups=valkey.buf.red,resources=users,verbs=create;update;delete,versions=v1alpha1,name=muser-v1alpha1.kb.io,admissionReviewVersions=v1

// UserCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind User when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type UserCustomDefaulter struct {
	mgrClient client.Client
}

var _ webhook.CustomDefaulter = &UserCustomDefaulter{}

func (d *UserCustomDefaulter) Default(ctx context.Context, obj runtime.Object) (err error) {
	inst, ok := obj.(*valkeybufredv1alpha1.User)

	if !ok {
		return fmt.Errorf("expected an User object but got %T", obj)
	}

	logger.V(3).Info("setup defaults", "user.name", inst.Name)
	if inst.Labels == nil {
		inst.Labels = make(map[string]string)
	}
	inst.Labels[builder.ManagedByLabelKey] = config.AppName
	inst.Labels[builder.InstanceNameLabelKey] = inst.Spec.InstanceName

	if inst.GetDeletionTimestamp() != nil {
		return
	}
	rule, err := user.NewRule(inst.Spec.AclRules)
	if err != nil {
		return
	}

	switch inst.Spec.Arch {
	case core.ValkeyFailover, core.ValkeyReplica:
		vf := &v1alpha1.Failover{}
		if err := d.mgrClient.Get(context.TODO(), types.NamespacedName{
			Namespace: inst.Namespace,
			Name:      inst.Spec.InstanceName,
		}, vf); err != nil {
			logger.Error(err, "get valkey failover failed", "name", inst.Name)
		} else {
			inst.OwnerReferences = util.BuildOwnerReferencesWithParents(vf)
		}
	case core.ValkeyCluster:
		cluster := &v1alpha1.Cluster{}
		if err := d.mgrClient.Get(context.TODO(), types.NamespacedName{
			Namespace: inst.Namespace,
			Name:      inst.Spec.InstanceName,
		}, cluster); err != nil {
			logger.Error(err, "get valkey cluster failed", "name", inst.Name)
		} else {
			inst.OwnerReferences = util.BuildOwnerReferencesWithParents(cluster)
		}
	}

	if inst.Spec.AccountType != v1alpha1.SystemAccount && inst.Spec.Username != user.DefaultOperatorUserName {
		if rule.IsACLCommandEnabled() {
			if slices.Contains(rule.AllowedCommands, "acl") {
				cmds := rule.AllowedCommands[:0]
				for _, cmd := range rule.AllowedCommands {
					if cmd != "acl" {
						cmds = append(cmds, cmd)
					}
				}
				rule.AllowedCommands = cmds
			}
			rule.DisallowedCommands = append(rule.DisallowedCommands, "acl")
		}
	}
	inst.Spec.AclRules = rule.Encode()

	return nil
}

// +kubebuilder:webhook:path=/validate-valkey-buf-red-v1alpha1-user,mutating=false,failurePolicy=fail,sideEffects=None,groups=valkey.buf.red,resources=users,verbs=create;update,versions=v1alpha1,name=vuser-v1alpha1.kb.io,admissionReviewVersions=v1

// UserCustomValidator struct is responsible for validating the User resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type UserCustomValidator struct {
	mgrClient client.Client
}

var _ webhook.CustomValidator = &UserCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type User.
func (v *UserCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (warns admission.Warnings, err error) {
	inst, ok := obj.(*valkeybufredv1alpha1.User)
	if !ok {
		return nil, fmt.Errorf("expected a User object but got %T", obj)
	}
	logger.Info("Validation for User upon creation", "name", inst.GetName())

	rule, err := user.NewRule(strings.ToLower(inst.Spec.AclRules))
	if err != nil {
		return nil, err
	}

	if inst.Spec.AccountType == v1alpha1.SystemAccount {
		if inst.Spec.Username != user.DefaultOperatorUserName {
			return nil, fmt.Errorf("system account username must be operator")
		}
		if rule.IsACLCommandsDisabled() {
			return nil, fmt.Errorf("system account must enable acl command")
		}
		return nil, nil
	}

	// check custom instance
	if len(inst.Spec.PasswordSecrets) == 0 {
		return nil, fmt.Errorf("password secret can not be empty")
	}
	if err := rule.Validate(true); err != nil {
		return nil, err
	}
	for _, passwordSecret := range inst.Spec.PasswordSecrets {
		if passwordSecret == "" {
			return nil, fmt.Errorf("password secret can not be empty")
		}
		secret := &v1.Secret{}
		if err := v.mgrClient.Get(context.Background(), types.NamespacedName{
			Namespace: inst.Namespace,
			Name:      passwordSecret,
		}, secret); err != nil {
			return nil, err
		} else if err := security.PasswordValidate(string(secret.Data["password"]), 8, 32); err != nil {
			return nil, err
		}
	}

	switch inst.Spec.Arch {
	case core.ValkeyFailover, core.ValkeyReplica:
		rf := &v1alpha1.Failover{}
		if err := v.mgrClient.Get(context.Background(), types.NamespacedName{
			Namespace: inst.Namespace,
			Name:      inst.Spec.InstanceName}, rf); err != nil {
			return nil, err
		}
		if rf.Status.Phase != v1alpha1.FailoverPhaseReady {
			return nil, fmt.Errorf("failover %s is not ready", inst.Spec.InstanceName)
		}
	case core.ValkeyCluster:
		cluster := v1alpha1.Cluster{}
		if err := v.mgrClient.Get(context.Background(), types.NamespacedName{
			Namespace: inst.Namespace,
			Name:      inst.Spec.InstanceName}, &cluster); err != nil {
			return nil, err
		}
		if cluster.Status.Phase != v1alpha1.ClusterPhaseReady {
			return nil, fmt.Errorf("cluster %s is not ready", inst.Spec.InstanceName)
		}
	}
	return
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type User.
func (v *UserCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (warns admission.Warnings, err error) {
	inst, ok := newObj.(*valkeybufredv1alpha1.User)
	if !ok {
		return nil, fmt.Errorf("expected a User object for the newObj but got %T", newObj)
	}
	logger.Info("Validation for User upon update", "name", inst.GetName())

	rule, err := user.NewRule(strings.ToLower(inst.Spec.AclRules))
	if err != nil {
		return nil, err
	}

	if inst.Spec.AccountType == v1alpha1.SystemAccount {
		if inst.Spec.Username != user.DefaultOperatorUserName {
			return nil, fmt.Errorf("system account username must be operator")
		}
		if rule.IsACLCommandsDisabled() {
			return nil, fmt.Errorf("system account must enable acl command")
		}
		return
	}

	if inst.GetDeletionTimestamp() != nil {
		return nil, nil
	}
	if len(inst.Spec.PasswordSecrets) == 0 {
		return nil, fmt.Errorf("password secret can not be empty")
	}

	if err := rule.Validate(true); err != nil {
		return nil, err
	}
	for _, passwordSecret := range inst.Spec.PasswordSecrets {
		if passwordSecret == "" {
			return nil, fmt.Errorf("password secret can not be empty")
		}
		secret := &v1.Secret{}
		if err := v.mgrClient.Get(context.TODO(), types.NamespacedName{
			Namespace: inst.Namespace,
			Name:      passwordSecret,
		}, secret); err != nil {
			return nil, err
		} else if err := security.PasswordValidate(string(secret.Data["password"]), 8, 32); err != nil {
			return nil, err
		}
	}

	switch inst.Spec.Arch {
	case core.ValkeyFailover, core.ValkeyReplica:
		vf := &v1alpha1.Failover{}
		if err := v.mgrClient.Get(context.Background(), types.NamespacedName{
			Namespace: inst.Namespace,
			Name:      inst.Spec.InstanceName}, vf); err != nil {
			return nil, err
		}
		if vf.Status.Phase != v1alpha1.FailoverPhaseReady {
			return nil, fmt.Errorf("failover %s is not ready", inst.Spec.InstanceName)
		}
	case core.ValkeyCluster:
		cluster := v1alpha1.Cluster{}
		if err := v.mgrClient.Get(context.TODO(), types.NamespacedName{
			Namespace: inst.Namespace,
			Name:      inst.Spec.InstanceName}, &cluster); err != nil {
			return nil, err
		}
		if cluster.Status.Phase != v1alpha1.ClusterPhaseReady {
			return nil, fmt.Errorf("cluster %s is not ready", inst.Spec.InstanceName)
		}
	}
	return
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type User.
func (v *UserCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (warns admission.Warnings, err error) {
	inst, ok := obj.(*valkeybufredv1alpha1.User)
	if !ok {
		return nil, fmt.Errorf("expected a User object but got %T", obj)
	}
	logger.Info("Validation for User upon deletion", "name", inst.GetName())

	if inst.Spec.Username == user.DefaultUserName || inst.Spec.Username == user.DefaultOperatorUserName {
		switch inst.Spec.Arch {
		case core.ValkeyFailover, core.ValkeyReplica:
			rf := v1alpha1.Failover{}
			if err := v.mgrClient.Get(context.TODO(), types.NamespacedName{
				Namespace: inst.Namespace,
				Name:      inst.Spec.InstanceName,
			}, &rf); errors.IsNotFound(err) {
				return nil, nil
			} else if err != nil {
				return nil, err
			} else if rf.GetDeletionTimestamp() != nil {
				return nil, nil
			}
		case core.ValkeyCluster:
			cluster := v1alpha1.Cluster{}
			if err := v.mgrClient.Get(context.Background(), types.NamespacedName{
				Namespace: inst.Namespace,
				Name:      inst.Spec.InstanceName,
			}, &cluster); errors.IsNotFound(err) {
				return nil, nil
			} else if err != nil {
				return nil, err
			} else if cluster.GetDeletionTimestamp() != nil {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("user %s can not be deleted", inst.GetName())
	}
	return
}
