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
package config

const (
	RedisSecretUsernameKey = "username"
	RedisSecretPasswordKey = "password" // #nosec

	S3_ACCESS_KEY_ID     = "AWS_ACCESS_KEY_ID"     // #nosec
	S3_SECRET_ACCESS_KEY = "AWS_SECRET_ACCESS_KEY" // #nosec
	S3_TOKEN             = "TOKEN"                 // #nosec
	S3_REGION            = "REGION"
	S3_ENDPOINTURL       = "ENDPOINTURL"

	PAUSE_ANNOTATION_KEY = "app.cpaas.io/pause-timestamp"

	// DNS
	LocalInjectName = "local.inject"

	ImageVersionKeyPrefix = "middleware.instance/imageversions-"
)

// Version Controller related keys
const (
	InstanceTypeKey               = "middleware.instance/type"
	CRUpgradeableVersion          = "middleware.upgrade.crVersion"
	CRUpgradeableComponentVersion = "middleware.upgrade.component.version"
	CRAutoUpgradeKey              = "middleware.instance/autoUpgrade"
	LatestKey                     = "middleware.instance/latest"
	CRVersionKey                  = "middleware.instance/crVersion"
	CRVersionSHAKey               = "middleware.instance/crVersion-sha"
	CoreComponentName             = "redis"

	OperatorVersionAnnotation = "operatorVersion"

	ACLSupportedVersionAnnotationKey = "middleware.alauda.io/acl-supported-version"
)
