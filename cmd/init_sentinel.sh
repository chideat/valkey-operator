#!/bin/sh

if [ "$SERVICE_TYPE" = "LoadBalancer" ] || [ "$SERVICE_TYPE" = "NodePort" ] || [ -n "$IP_FAMILY_PREFER" ] ; then
    echo "check pod binded service"
    /opt/valkey-helper sentinel expose || exit 1
fi

# copy binaries
cp /opt/*.sh /opt/valkey-helper /mnt/opt/ && chmod 555 /mnt/opt/*.sh /mnt/opt/valkey-helper
