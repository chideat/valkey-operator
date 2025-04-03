#!/bin/sh

chmod -f 644 /data/*.rdb /data/*.aof 2>/dev/null || true
chown -f 999:1000 /data/*.rdb /data/*.aof 2>/dev/null || true

if [ "${SERVICE_TYPE}" = "LoadBalancer" ] || [ "${SERVICE_TYPE}" = "NodePort" ] || [ -n "${IP_FAMILY_PREFER}" ] ; then
    echo "check pod binded service"
    /opt/valkey-helper failover expose || exit 1
fi

# copy binaries
cp /opt/*.sh /opt/valkey-helper /mnt/opt/ && chmod 555 /mnt/opt/*.sh /mnt/opt/valkey-helper
