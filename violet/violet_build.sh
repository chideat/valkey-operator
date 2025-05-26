#!/usr/bin/env bash
set -e
export bundle_tag=${TAG}
echo "bundle_tag: ${bundle_tag}"

if [ -z "${TAG}" ]; then
    echo "TAG is not set"
    exit 1
fi

bundle_resp="registry.alauda.cn:60080/middleware/valkey-operator-bundle"
group_name="valkey-operator"
project_name="valkey-operator"
pkg_dir="${project_name}-${bundle_tag}"

minio_host="prod-minio.alauda.cn"
minio_access_key="platform-eco"
minio_secret_key="shi2ieWu"
bucket_name="platform-eco"

##########build bundle##########

echo -e "Begin create manifest from registry...\n"
violet create ${project_name} --artifact ${bundle_resp}:${bundle_tag} --no-auth --plain --platforms linux/amd64 --platforms linux/arm64 --debug
echo -e "Begin package manifest from registry...\n"
violet package ${project_name} --no-auth --plain --output ${pkg_dir} --debug

##########upload to minio##########

echo -e "package ${pkg_dir}.tgz done, start uploading...\n"
mc alias set prod http://${minio_host} ${minio_access_key} ${minio_secret_key} --api s3v4
mc cp "${pkg_dir}.tgz" "prod/${bucket_name}/${group_name}/${pkg_dir}.tgz"
echo -e "Upload finished successfully!\n"
echo -e "Get http://${minio_host}/${bucket_name}/${group_name}/${pkg_dir}.tgz to download this package\n"
