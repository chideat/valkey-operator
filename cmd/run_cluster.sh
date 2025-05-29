#!/bin/sh

# Configuration file paths
VALKEY_DEFAULT_CONFIG_FILE="/etc/valkey/valkey.conf"
VALKEY_CONFIG_FILE="/tmp/valkey.conf"
ACL_CONFIG_FILE="/tmp/acl.conf"
ANNOUNCE_CONFIG_FILE="/data/announce.conf"
CLUSTER_CONFIG_FILE="/data/nodes.conf"
OPERATOR_PASSWORD_FILE="/account/password"
TLS_DIR="/tls"

echo "# Run: cluster heal"
/opt/valkey-helper cluster heal || exit 1

# Set up listening IP based on preference
LISTEN="${POD_IP}"
LOCALHOST="local.inject"
if echo "${POD_IPS}" | grep -q ','; then
    POD_IPS_LIST=$(echo "${POD_IPS}" | tr ',' ' ')
    for ip in $POD_IPS_LIST; do
        if [ "$IP_FAMILY_PREFER" = "IPv6" ] && echo "$ip" | grep -q ':'; then
            LISTEN="$ip"
            break
        elif [ "$IP_FAMILY_PREFER" = "IPv4" ] && echo "$ip" | grep -q '\.'; then
            LISTEN="$ip"
            break
        fi
    done
fi

# Update cluster config with correct IP
if [ -f "${CLUSTER_CONFIG_FILE}" ]; then
    sed -i.bak -e "/myself/ s/ .*:[0-9]*@[0-9]*/ ${LISTEN}:6379@16379/" "${CLUSTER_CONFIG_FILE}"
fi

# Create Valkey config
cat "${VALKEY_DEFAULT_CONFIG_FILE}" > "${VALKEY_CONFIG_FILE}" || { echo "Failed to copy default config"; exit 1; }

# Add authentication configuration
echo "masteruser \"${OPERATOR_USERNAME}\"" >> "${VALKEY_CONFIG_FILE}"
password=$(cat "${OPERATOR_PASSWORD_FILE}" 2>/dev/null)
if [ -n "${password}" ]; then
    echo "masterauth \"${password}\"" >> "${VALKEY_CONFIG_FILE}"
fi

# Handle announcement configuration
if [ -z "${SERVICE_TYPE}" ] || [ "${SERVICE_TYPE}" = "ClusterIP" ]; then
    rm -f "${ANNOUNCE_CONFIG_FILE}"
fi
if [ -f "${ANNOUNCE_CONFIG_FILE}" ]; then
    cat "${ANNOUNCE_CONFIG_FILE}" >> "${VALKEY_CONFIG_FILE}"
    echo "" >> "${VALKEY_CONFIG_FILE}"
fi

# Set binding parameters
if [ "${LISTEN}" != "${POD_IP}" ]; then
    LISTEN="${LISTEN} ${POD_IP}"
fi
ARGS="--cluster-enabled yes --cluster-config-file ${CLUSTER_CONFIG_FILE} --protected-mode no --bind ${LISTEN} ${LOCALHOST}"

# Configure TLS if enabled
if [ "${TLS_ENABLED}" = "true" ]; then
    if [ -f ${ANNOUNCE_CONFIG_FILE} ]; then
        ANNOUNCE_PORT=$(grep "cluster-announce-port" ${ANNOUNCE_CONFIG_FILE} | awk '{print $2}')
        echo "cluster-announce-tls-port ${ANNOUNCE_PORT}" >> "${VALKEY_CONFIG_FILE}"
    fi
    ARGS="${ARGS} --port 0 --tls-port 6379 --tls-cluster yes --tls-replication yes --tls-cert-file ${TLS_DIR}/tls.crt --tls-key-file ${TLS_DIR}/tls.key --tls-ca-cert-file ${TLS_DIR}/ca.crt"
fi

# Configure ACL if available
ACL_ARGS=""
if [ -n "${ACL_CONFIGMAP_NAME}" ]; then
    echo "# Run: generate acl"
    /opt/valkey-helper helper generate acl --name "${ACL_CONFIGMAP_NAME}" --namespace "${NAMESPACE}" > "${ACL_CONFIG_FILE}" || exit 1
    ACL_ARGS="--aclfile ${ACL_CONFIG_FILE}"
fi

# Set secure permissions on config files
chmod 0600 "${VALKEY_CONFIG_FILE}"
chmod 0600 "${ACL_CONFIG_FILE}" 2>/dev/null || true

# Start valkey server with all parameters
exec valkey-server "${VALKEY_CONFIG_FILE}" ${ACL_ARGS} ${ARGS} $@
