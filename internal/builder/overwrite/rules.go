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

package overwrite

import (
	"slices"

	"github.com/chideat/valkey-operator/internal/builder"
	"github.com/chideat/valkey-operator/internal/builder/certbuilder"
)

// Component is a group of pods that one overwrites list applies to. Each has
// its own containers, so its own rules.
type Component string

const (
	// ClusterNodes are the Valkey pods of a cluster instance.
	ClusterNodes Component = "cluster"
	// FailoverNodes are the Valkey pods of a failover or replica instance.
	FailoverNodes Component = "failover"
	// SentinelNodes are the sentinel pods of a failover instance.
	SentinelNodes Component = "sentinel"
)

// What the builders generate. Names defined in the clusterbuilder,
// failoverbuilder and sentinelbuilder packages are spelled out here, since
// those packages import this one; the completeness tests check every list
// against real builder output.
var (
	// containers each component may run, depending on its settings.
	containers = map[Component][]string{
		ClusterNodes:  {builder.ServerContainerName, builder.ExporterContainerName, builder.AgentContainerName},
		FailoverNodes: {builder.ServerContainerName, builder.ExporterContainerName},
		SentinelNodes: {builder.SentinelContainerName, builder.AgentContainerName},
	}
	initContainers = []string{builder.InitContainerName}

	// probes the operator sets on its main containers.
	operatorProbes = map[string][]string{
		builder.ServerContainerName:   {"startupProbe", "livenessProbe", "readinessProbe"},
		builder.SentinelContainerName: {"startupProbe", "livenessProbe"},
	}

	// labelKeys the operator puts on generated objects and pod templates;
	// the selectors use some of them.
	labelKeys = []string{
		builder.ManagedByLabelKey,
		builder.AppComponentLabelKey,
		builder.AppNameLabelKey,
		builder.InstanceTypeLabelKey,
		builder.InstanceNameLabelKey,
		builder.ArchLabelKey,
		builder.RoleLabelKey,
		// clusterbuilder: the StatefulSet of one shard.
		"statefulset",
	}

	// envNames the operator sets in any container, plus the exporter
	// variables that would point it at another server or credentials.
	envNames = []string{
		"ACL_CONFIGMAP_NAME",
		"IP_FAMILY_PREFER",
		// failoverbuilder.MonitorOperatorSecretName
		"MONITOR_OPERATOR_SECRET_NAME",
		"MONITOR_POLICY",
		"MONITOR_URI",
		"NAMESPACE",
		builder.OperatorSecretName,
		builder.OperatorUsername,
		"POD_IP",
		"POD_IPS",
		"POD_NAME",
		"POD_UID",
		"REDIS_ADDR",
		"REDIS_EXPORTER_SKIP_TLS_VERIFICATION",
		"REDIS_EXPORTER_TLS_CA_CERT_FILE",
		"REDIS_EXPORTER_TLS_CLIENT_CERT_FILE",
		"REDIS_EXPORTER_TLS_CLIENT_KEY_FILE",
		builder.PasswordEnvName,
		"REDIS_PASSWORD_FILE",
		builder.UserEnvName,
		"SENTINEL_ANNOUNCE_PATH",
		"SERVICE_NAME",
		"SERVICE_TYPE",
		certbuilder.TLSCaFileKey,
		certbuilder.TLSCertFileKey,
		certbuilder.TLSKeyFileKey,
		certbuilder.TLSEnabledKey,
	}

	// volumeNames and mountPaths the operator uses in any pod. Anything
	// mounted under one of the paths is protected too.
	volumeNames = []string{
		"conf",
		"sentinel-config",
		"temp",
		"valkey-auth",
		builder.ValkeyDataVolumeName,
		"valkey-opt",
		builder.ValkeyTLSVolumeName,
	}
	mountPaths = []string{
		"/account",
		builder.ValkeyDataVolumeDefaultMountPath,
		"/etc/valkey",
		"/mnt/opt",
		"/opt",
		"/tmp",
		builder.ValkeyTLSVolumeDefaultMountPath,
	}

	// localhostIPs are the addresses of the local.inject host alias.
	localhostIPs = []string{"127.0.0.1", "::1"}

	// exporterFlags are the redis_exporter flags the operator sets, or that
	// would point the exporter at another server or credentials.
	exporterFlags = []string{
		"redis.addr",
		"redis.password",
		"redis.password-file",
		"redis.user",
		"skip-tls-verification",
		"tls-ca-cert-file",
		"tls-client-cert-file",
		"tls-client-key-file",
		"version",
		"web.listen-address",
		"web.telemetry-path",
	}
)

// objectMetaFields are the ObjectMeta fields other than labels and
// annotations: they identify the object or belong to the API server.
var objectMetaFields = []string{
	"name",
	"generateName",
	"namespace",
	"selfLink",
	"uid",
	"resourceVersion",
	"generation",
	"creationTimestamp",
	"deletionTimestamp",
	"deletionGracePeriodSeconds",
	"ownerReferences",
	"finalizers",
	"managedFields",
}

func prefixed(prefix []string, keys ...string) []string {
	return append(slices.Clone(prefix), keys...)
}

// objectGuards protect the parts every generated object shares: its type, its
// identity, the operator's labels and checksums, and its status.
func objectGuards() []guard {
	guards := []guard{fixed{"apiVersion"}, fixed{"kind"}, fixed{"status"}}
	for _, f := range objectMetaFields {
		guards = append(guards, fixed{"metadata", f})
	}
	return append(guards,
		entries{path: []string{"metadata", "labels"}, keys: labelKeys, fromBase: true},
		entries{path: []string{"metadata", "annotations"}, prefixes: []string{builder.ChecksumLabelKey}},
	)
}

// statefulSetGuards protect a generated StatefulSet of component.
func statefulSetGuards(component Component) []guard {
	guards := objectGuards()
	for _, f := range []string{
		// The operator scales and rolls the StatefulSet itself.
		"replicas", "updateStrategy", "revisionHistoryLimit", "ordinals",
		// Immutable: a change makes the actors delete and recreate the StatefulSet.
		"selector", "serviceName", "podManagementPolicy", "volumeClaimTemplates",
		// spec.storage.retainAfterDeleted decides what happens to the PVCs.
		"persistentVolumeClaimRetentionPolicy",
	} {
		guards = append(guards, fixed{"spec", f})
	}

	tpl := []string{"spec", "template"}
	pod := prefixed(tpl, "spec")
	guards = append(guards,
		entries{path: prefixed(tpl, "metadata", "labels"), keys: labelKeys, fromBase: true},
		entries{
			path:     prefixed(tpl, "metadata", "annotations"),
			keys:     []string{builder.RestartAnnotationKey},
			prefixes: []string{builder.ChecksumLabelKey},
		},
	)
	for _, f := range []string{
		// The operator's RBAC, and the time its preStop shutdown needs.
		"serviceAccountName", "serviceAccount", "automountServiceAccountToken", "terminationGracePeriodSeconds",
		// These have typed fields in the spec.
		"affinity", "tolerations", "nodeSelector", "securityContext", "imagePullSecrets",
	} {
		guards = append(guards, fixed(prefixed(pod, f)))
	}
	guards = append(guards,
		items{path: prefixed(pod, "hostAliases"), mergeKey: "ip", ids: localhostIPs, fromBase: true},
		items{path: prefixed(pod, "volumes"), mergeKey: "name", ids: volumeNames, fromBase: true},
		generatedOnly{path: prefixed(pod, "initContainers"), mergeKey: "name", ids: initContainers},
		generatedOnly{path: prefixed(pod, "containers"), mergeKey: "name", ids: containers[component]},
	)
	for _, name := range initContainers {
		guards = append(guards, within{path: prefixed(pod, "initContainers"), mergeKey: "name", id: name, guards: containerGuards(name)})
	}
	for _, name := range containers[component] {
		guards = append(guards, within{path: prefixed(pod, "containers"), mergeKey: "name", id: name, guards: containerGuards(name)})
	}
	// Last, once everything generated is back in the lists.
	return append(guards,
		ordered{path: prefixed(pod, "initContainers"), mergeKey: "name"},
		ordered{path: prefixed(pod, "containers"), mergeKey: "name"},
		ordered{path: prefixed(pod, "volumes"), mergeKey: "name"},
		ordered{path: prefixed(pod, "hostAliases"), mergeKey: "ip"},
	)
}

// containerGuards protect one of the operator's containers.
func containerGuards(name string) []guard {
	guards := []guard{
		fixed{"image"},
		fixed{"imagePullPolicy"},
		fixed{"command"},
		fixed{"ports"},
		fixed{"securityContext"},
		items{path: []string{"env"}, mergeKey: "name", ids: envNames, fromBase: true},
		mounts{path: []string{"volumeMounts"}, paths: mountPaths},
	}
	switch name {
	case builder.ServerContainerName, builder.SentinelContainerName:
		// resources: maxmemory is computed from spec.resources.
		guards = append(guards, fixed{"args"}, fixed{"lifecycle"}, fixed{"resources"})
		for _, key := range []string{"startupProbe", "livenessProbe", "readinessProbe"} {
			guards = append(guards, probe{key: key, operatorSets: slices.Contains(operatorProbes[name], key)})
		}
	case builder.ExporterContainerName:
		// resources: spec.exporter.resources.
		guards = append(guards, fixed{"resources"}, flags{path: []string{"args"}, names: exporterFlags})
	default:
		guards = append(guards, fixed{"args"})
	}
	return append(guards,
		ordered{path: []string{"env"}, mergeKey: "name"},
		ordered{path: []string{"volumeMounts"}, mergeKey: "mountPath"},
	)
}
