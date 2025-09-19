#!/bin/sh

VALKEY_DEFAULT_CONFIG_FILE="/etc/valkey/valkey.conf"
VALKEY_CONFIG_FILE="/tmp/valkey.conf"
ACL_CONFIG_FILE="/tmp/acl.conf"
ANNOUNCE_CONFIG_FILE="/data/announce.conf"
OPERATOR_PASSWORD_FILE="/account/password"
TLS_DIR="/tmp"

# init valkey.config, ignore replica-priority and slave-priority
cat ${VALKEY_DEFAULT_CONFIG_FILE} | grep -vE "^(replica-priority|slave-priority)" > "$VALKEY_CONFIG_FILE"
# patch replica-priority to control failover order
echo "replica-priority $(($(echo $POD_NAME | grep -oE '[0-9]+$')+100))" >> ${VALKEY_CONFIG_FILE}

echo "masteruser \"${OPERATOR_USERNAME}\"" >> ${VALKEY_CONFIG_FILE}
password=$(cat ${OPERATOR_PASSWORD_FILE} 2>/dev/null)
if [ -n ${password} ]; then
    echo "masterauth \"${password}\"" >> ${VALKEY_CONFIG_FILE}
fi

# Handle sentinel monitoring policy
if [ "$MONITOR_POLICY" = "sentinel" ]; then
    ANNOUNCE_IP=""
    ANNOUNCE_PORT=""

    if [ -f "$ANNOUNCE_CONFIG_FILE" ]; then
        echo "" >> "$VALKEY_CONFIG_FILE"
        cat "$ANNOUNCE_CONFIG_FILE" >> "$VALKEY_CONFIG_FILE"

        ANNOUNCE_IP=$(grep 'announce-ip' "$ANNOUNCE_CONFIG_FILE" | awk '{print $2}')
        ANNOUNCE_PORT=$(grep 'announce-port' "$ANNOUNCE_CONFIG_FILE" | awk '{print $2}')
    fi
    
    echo "## check and do failover"
    addr=$(/opt/valkey-helper sentinel failover --escape "${ANNOUNCE_PORT}" --escape "${ANNOUNCE_IP}:${ANNOUNCE_PORT}" --escape "[${ANNOUNCE_IP}]:${ANNOUNCE_PORT}" --timeout 120)
    if [ $? -eq 0 ] && [ -n "$addr" ]; then
        echo "## found current master: $addr"
        # Check if the address is IPv6 or IPv4
        if echo "$addr" | grep -q ']:'; then
            master=$(echo "$addr" | sed -n 's/\(\[.*\]\):\([0-9]*\)/\1/p' | tr -d '[]')
            masterPort=$(echo "$addr" | sed -n 's/\(\[.*\]\):\([0-9]*\)/\2/p')
        else
            master=$(echo "$addr" | cut -d ':' -f 1)
            masterPort=$(echo "$addr" | cut -d ':' -f 2)
        fi

        echo "## config $master $masterPort as my master"
        echo "" >> "$VALKEY_CONFIG_FILE"
        echo "slaveof $master $masterPort" >> "$VALKEY_CONFIG_FILE"
    fi
fi

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
if [ "${LISTEN}" != "${POD_IP}" ]; then
    LISTEN="${LISTEN} ${POD_IP}"
fi
# Listne only to protocol matched IP
ARGS="--protected-mode no --bind $LISTEN $LOCALHOST"

# Add TLS arguments if TLS is enabled
if [ "$TLS_ENABLED" = "true" ]; then
    ARGS="${ARGS} --port 0 --tls-port 6379 --tls-replication yes --tls-cert-file ${TLS_DIR}/tls.crt --tls-key-file ${TLS_DIR}/tls.key --tls-ca-cert-file ${TLS_DIR}/ca.crt"
fi

ACL_ARGS=""
# when valkey acl supported, inject acl config
if [ -n "${ACL_CONFIGMAP_NAME}" ]; then
    echo "# Run: generate acl"
    /opt/valkey-helper helper generate acl --name "${ACL_CONFIGMAP_NAME}" --namespace "${NAMESPACE}" > "$ACL_CONFIG_FILE" || exit 1
    ACL_ARGS="--aclfile $ACL_CONFIG_FILE"
fi

# Set permissions for configuration files
chmod 0600 "$VALKEY_CONFIG_FILE"
chmod 0600 "$ACL_CONFIG_FILE"

# Start valkey server with the constructed arguments
exec valkey-server "$VALKEY_CONFIG_FILE" $ACL_ARGS $ARGS $@
