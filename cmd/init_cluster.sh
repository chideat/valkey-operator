#!/bin/sh

chmod -f 644 /data/*.rdb /data/*.aof /data/*.conf 2>/dev/null || true
chown -f 999:1000 /data/*.rdb /data/*.aof /data/*.conf 2>/dev/null || true

rm -rf /data/announce.conf

echo "check pod binded service"
/opt/valkey-helper cluster expose || exit 1

# copy binaries
cp /opt/*.sh /opt/valkey-helper /mnt/opt/ && chmod 555 /mnt/opt/*.sh /mnt/opt/valkey-helper
