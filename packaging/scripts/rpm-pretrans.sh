#!/bin/sh
# rpm runs this before it changes anything. Up to 1.5.x the configuration
# files belonged to the package, so removing the old package during the
# upgrade deletes the unchanged ones and saves a changed one as .rpmsave. A
# checksum of each file now lets the posttrans script tell the .rpmsave this
# upgrade made from an older one.
CONFIG_DIR=/etc/sensor-hub
STATE_DIR=/run/sensor-hub-package

rm -rf "$STATE_DIR"
[ -d "$CONFIG_DIR" ] || exit 0
mkdir -m 0700 "$STATE_DIR" || exit 0
for name in environment application.properties database.properties smtp.properties; do
  if [ -f "$CONFIG_DIR/$name" ]; then
    md5sum < "$CONFIG_DIR/$name" | cut -d' ' -f1 > "$STATE_DIR/$name.md5"
  fi
done
exit 0
