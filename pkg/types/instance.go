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
*/package types

import (
	"context"
	"crypto/tls"

	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/chideat/valkey-operator/api/core"
	"github.com/chideat/valkey-operator/pkg/version"
	"github.com/go-logr/logr"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Object interface {
	v1.Object
	GetObjectKind() schema.ObjectKind
	DeepCopyObject() runtime.Object
	NamespacedName() client.ObjectKey
	Version() version.ValkeyVersion
	IsReady() bool

	Restart(ctx context.Context, annotationKeyVal ...string) error
	Refresh(ctx context.Context) error
}

type InstanceStatus string

const (
	Any    InstanceStatus = ""
	OK     InstanceStatus = "OK"
	Fail   InstanceStatus = "Fail"
	Paused InstanceStatus = "Paused"
)

type Instance interface {
	Object

	Arch() core.Arch
	// Issuer custom cert issuer
	Issuer() *certmetav1.ObjectReference
	Users() Users
	TLSConfig() *tls.Config
	IsInService() bool
	IsACLUserExists() bool
	IsACLAppliedToAll() bool
	IsResourceFullfilled(ctx context.Context) (bool, error)
	UpdateStatus(ctx context.Context, st InstanceStatus, message string) error
	SendEventf(eventtype, reason, messageFmt string, args ...interface{})
	Logger() logr.Logger
}
