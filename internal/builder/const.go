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
*/package builder

import (
	"fmt"
	"strings"
)

const (
	HostnameTopologyKey  = "kubernetes.io/hostname"
	RestartAnnotationKey = "kubectl.kubernetes.io/restartedAt"
	PodNameLabelKey      = "statefulset.kubernetes.io/pod-name"
	ManagedByLabelKey    = "app.kubernetes.io/managed-by"
	AppComponentLabelKey = "app.kubernetes.io/component"
	AppNameLabelKey      = "app.kubernetes.io/name"

	ArchLabelKey          = "valkeyarch"
	RoleLabelKey          = "valkey.buf.red/role"
	AnnounceIPLabelKey    = "valkey.buf.red/announce_ip"
	AnnouncePortLabelKey  = "valkey.buf.red/announce_port"
	AnnounceIPortLabelKey = "valkey.buf.red/announce_iport"
	ChecksumLabelKey      = "valkey.buf.red/checksum"

	InstanceTypeLabelKey = "buf.red/type"
	InstanceNameLabelKey = "buf.red/name"
)

func ChecksumKey(typ string) string {
	return fmt.Sprintf("%s-%s", ChecksumLabelKey, strings.ToLower(typ))
}

const (
	// Env
	OperatorUsername   = "OPERATOR_USERNAME"
	OperatorSecretName = "OPERATOR_SECRET_NAME"
)

const (
	ServerContainerName   = "valkey"
	SentinelContainerName = "sentinel"
	ValkeyConfigKey       = "valkey.conf"

	DefaultValkeyServerPort    = 6379
	DefaultValkeyServerBusPort = 16379
	DefaultValkeySentinelPort  = 26379
)

const (
	ValkeyTLSVolumeName             = "valkey-tls"
	ValkeyTLSVolumeDefaultMountPath = "/tls"

	ValkeyDataVolumeName             = "valkey-data"
	ValkeyDataVolumeDefaultMountPath = "/data"
)
