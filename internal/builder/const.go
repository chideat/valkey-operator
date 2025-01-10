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
package builder

const (
	LabelValkeyArch   = "valkeyarch"
	ManagedByLabel    = "managed-by"
	InstanceTypeLabel = "middleware.instance/type"
	InstanceNameLabel = "middleware.instance/name"

	PodNameLabelKey          = "statefulset.kubernetes.io/pod-name"
	PodAnnounceIPLabelKey    = "middleware.alauda.io/announce_ip"
	PodAnnouncePortLabelKey  = "middleware.alauda.io/announce_port"
	PodAnnounceIPortLabelKey = "middleware.alauda.io/announce_iport"

	AppLabel             = "valkey-failover"
	HostnameTopologyKey  = "kubernetes.io/hostname"
	RestartAnnotationKey = "kubectl.kubernetes.io/restartedAt"

	ConfigSigAnnotationKey   = "middleware.alauda.io/config-sig"
	PasswordSigAnnotationKey = "middleware.alauda.io/secret-sig"

	S3SecretVolumeName = "s3-secret"
)
