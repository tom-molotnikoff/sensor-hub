#!/bin/sh
# rpm runs this once the whole transaction is done, after the old package is
# removed. A configuration file that removal took away is put back: the
# operator's changed file from its .rpmsave, an unchanged one from the new
# default, as a config file upgrade would have. Then the service restarts if
# postinstall asked for it, so the hub only starts on the finished
# configuration.
CONFIG_DIR=/etc/sensor-hub
DEFAULTS_DIR=/usr/share/sensor-hub/defaults
STATE_DIR=/run/sensor-hub-package

for name in environment application.properties database.properties smtp.properties; do
  file="$CONFIG_DIR/$name"
  [ -e "$file" ] && continue
  if [ -f "$file.rpmsave" ] && [ -f "$STATE_DIR/$name.md5" ] &&
    [ "$(md5sum < "$file.rpmsave" | cut -d' ' -f1)" = "$(cat "$STATE_DIR/$name.md5")" ]; then
    mv "$file.rpmsave" "$file"
    echo "Kept your $file."
  else
    install -m 0640 -o sensor-hub -g sensor-hub "$DEFAULTS_DIR/$name" "$file"
  fi
done

restart=no
[ -e "$STATE_DIR/restart" ] && restart=yes
rm -rf "$STATE_DIR"
if [ "$restart" = yes ]; then
  systemctl restart sensor-hub
fi
exit 0
