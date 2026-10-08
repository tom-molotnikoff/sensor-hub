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

# restore_saved puts back the operator's changed file that removing the old
# package saved as .rpmsave during this upgrade.
restore_saved() {
  file="$CONFIG_DIR/$1"
  [ -f "$file.rpmsave" ] && [ -f "$STATE_DIR/$1.md5" ] &&
    [ "$(md5sum < "$file.rpmsave" | cut -d' ' -f1)" = "$(cat "$STATE_DIR/$1.md5")" ] || return 1
  mv "$file.rpmsave" "$file"
  echo "Kept your $file."
}

for name in environment application.properties database.properties; do
  [ -e "$CONFIG_DIR/$name" ] && continue
  restore_saved "$name" ||
    install -m 0640 -o sensor-hub -g sensor-hub "$DEFAULTS_DIR/$name" "$CONFIG_DIR/$name"
done

# 1.5.x shipped smtp.properties, which 2.0 does not. A changed one is put back
# so the hub can carry its smtp.user into the email settings, as it is left in
# place on a deb upgrade; an unchanged one held nothing and stays gone.
[ -e "$CONFIG_DIR/smtp.properties" ] || restore_saved smtp.properties || true

restart=no
[ -e "$STATE_DIR/restart" ] && restart=yes
rm -rf "$STATE_DIR"
if [ "$restart" = yes ]; then
  systemctl restart sensor-hub
fi
exit 0
