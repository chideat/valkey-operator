#!/bin/sh

VALKEY_SENTINEL_DEFAULT_CONFIG_FILE="/etc/valkey/sentinel.conf"
VALKEY_SENTINEL_CONFIG_FILE="/data/sentinel.conf"
ANNOUNCE_CONFIG_FILE="/data/announce.conf"
OPERATOR_PASSWORD_FILE="/account/password"
TLS_DIR="/tls"

cat ${VALKEY_SENTINEL_DEFAULT_CONFIG_FILE} > ${VALKEY_SENTINEL_CONFIG_FILE}

# Append password to sentinel configuration if it exists
password=$(cat ${OPERATOR_PASSWORD_FILE} 2>/dev/null)
if [ -n "${password}" ]; then
     echo "requirepass \"${password}\"" >> ${VALKEY_SENTINEL_CONFIG_FILE}
fi

# Append announce configuration to sentinel configuration if it exists
if [ -f ${ANNOUNCE_CONFIG_FILE} ]; then
    echo "# append announce conf to sentinel config"
    cat "${ANNOUNCE_CONFIG_FILE}" | grep "announce" | sed "s/^/sentinel /" >> ${VALKEY_SENTINEL_CONFIG_FILE}
fi

# Merge custom configuration
/opt/valkey-helper sentinel merge-config --local-conf-file "${VALKEY_SENTINEL_CONFIG_FILE}"

# Set up listening IP based on preference
LISTEN="${POD_IP}"
# The loopback entry is bound as a literal, never as the local.inject alias the
# clients in this pod resolve through /etc/hosts: valkey-server picks a bind
# entry's address family by looking for a colon in it, so a hostname is always
# looked up as IPv4 and the server aborts with "Name has no usable address" when
# the alias behind it is ::1. The literal has to be the address the alias
# resolves to, or the probes and hooks that dial local.inject reach a port
# nothing listens on. builder.LocalhostAlias writes that alias from the same
# field, so the two mappings must stay in step: IPv6 -> ::1, anything else, the
# unset value included -> 127.0.0.1.
LOCALHOST="127.0.0.1"
if [ "${IP_FAMILY_PREFER}" = "IPv6" ]; then
    LOCALHOST="::1"
fi
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

# Construct arguments for valkey-server
ARGS="--sentinel --protected-mode no --bind ${LISTEN} ${LOCALHOST}"

# Add TLS arguments if TLS is enabled
if [ "${TLS_ENABLED}" = "true" ]; then
    ARGS="${ARGS} --port 0 --tls-port 26379 --tls-replication yes --tls-cert-file ${TLS_DIR}/tls.crt --tls-key-file ${TLS_DIR}/tls.key --tls-ca-cert-file ${TLS_DIR}/ca.crt"
fi

# Set permissions for sentinel configuration
chmod 0600 ${VALKEY_SENTINEL_CONFIG_FILE}

# Start valkey-server with the constructed arguments
exec valkey-server ${VALKEY_SENTINEL_CONFIG_FILE} ${ARGS} $@ | sed 's/auth-pass .*/auth-pass ******/'
