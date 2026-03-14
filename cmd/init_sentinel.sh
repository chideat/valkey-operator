#!/bin/sh

rm -rf /data/announce.conf

echo "check pod binded service"
/opt/valkey-helper sentinel expose || exit 1

# copy binaries
cp /opt/*.sh /opt/valkey-helper /mnt/opt/ && chmod 555 /mnt/opt/*.sh /mnt/opt/valkey-helper
