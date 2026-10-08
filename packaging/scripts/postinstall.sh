#!/bin/bash
install -d -m 0750 -o sensor-hub -g sensor-hub /var/lib/sensor-hub
install -d -m 0750 -o sensor-hub -g sensor-hub /var/log/sensor-hub

CONFIG_DIR=/etc/sensor-hub
DEFAULTS_DIR=/usr/share/sensor-hub/defaults
CONFIG_FILES="environment application.properties database.properties smtp.properties"
# The rpm posttrans script reads what this one leaves here.
RPM_STATE_DIR=/run/sensor-hub-package
# init-key exits with this status when a key already exists in any form.
KEY_EXISTS=3

install_default() {
  install -m 0640 -o sensor-hub -g sensor-hub "$DEFAULTS_DIR/$1" "$CONFIG_DIR/$1"
}

# The configuration files belong to the operator, not the package. The
# package ships them as templates and creates only the missing ones, so a new
# default never stops an upgrade to ask what to do with a changed file.
create_missing_config() {
  install -d -m 0755 "$CONFIG_DIR"
  for name in $CONFIG_FILES; do
    [ -e "$CONFIG_DIR/$name" ] || install_default "$name"
  done
}

# Up to 1.5.x dpkg owned the configuration files as conffiles, and it still
# remembers what each one held when it was installed. One the operator never
# changed takes the new default, as a conffile upgrade would have done; a
# changed one is kept as it is.
update_unchanged_dpkg_conffiles() {
  dpkg-query -W -f='${Conffiles}\n' sensor-hub 2>/dev/null | while read -r path md5 state _; do
    [ "$state" = obsolete ] || continue
    name="${path#"$CONFIG_DIR"/}"
    case " $CONFIG_FILES " in *" $name "*) ;; *) continue ;; esac
    [ -f "$path" ] && [ "$(md5sum < "$path" | cut -d' ' -f1)" = "$md5" ] || continue
    install_default "$name"
  done
}

has_tpm() {
  command -v systemd-creds >/dev/null 2>&1 && systemd-creds has-tpm2 >/dev/null 2>&1
}

init_key() {
  init_key_output=$(/usr/bin/sensor-hub local secrets init-key --config-dir="$CONFIG_DIR" "$@" 2>&1)
}

# Creates the secret-store key unless one exists in any form: sealed with the
# TPM where there is one, so a copy of the disk carries nothing that decrypts
# the stored secrets, and otherwise as a key file only the service can read.
ensure_secrets_key() {
  if has_tpm; then
    init_key --seal
    case $? in
      0)
        echo "Sealed a new secret-store key with the TPM in $CONFIG_DIR/secrets.key.cred."
        echo "Keep a copy of it: sudo sensor-hub local secrets show-key"
        return
        ;;
      "$KEY_EXISTS") return ;;
    esac
    echo "Could not seal the secret-store key with the TPM, so it goes in a key file instead:" >&2
    echo "$init_key_output" >&2
  fi
  init_key
  case $? in
    0)
      chown sensor-hub:sensor-hub "$CONFIG_DIR/secrets.key"
      echo "Wrote a new secret-store key to $CONFIG_DIR/secrets.key."
      echo "Keep a copy of it: sudo sensor-hub local secrets show-key"
      ;;
    "$KEY_EXISTS") ;;
    *)
      echo "WARNING: could not create the secret-store key, so sensor-hub will not start:" >&2
      echo "$init_key_output" >&2
      echo "Create one with 'sudo sensor-hub local secrets init-key', then start sensor-hub." >&2
      ;;
  esac
}

if [ "$1" = "configure" ]; then
  update_unchanged_dpkg_conffiles
fi
create_missing_config
ensure_secrets_key

systemctl daemon-reload

if [ "$1" = "2" ]; then
  # An rpm upgrade. rpm removes the old package after this script, which on an
  # upgrade from 1.5.x takes its configuration files with it, so the posttrans
  # script puts them back and restarts once the transaction is done.
  install -d -m 0700 "$RPM_STATE_DIR" && touch "$RPM_STATE_DIR/restart"
elif [ "$1" = "configure" ] && [ -n "$2" ]; then
  # A deb upgrade: dpkg names the version it replaces.
  systemctl restart sensor-hub
else
  systemctl enable sensor-hub
  echo ""
  echo "=========================================="
  echo " Sensor Hub installed successfully."
  echo " Configure: /etc/sensor-hub/"
  echo " Then start: systemctl start sensor-hub"
  echo "=========================================="
fi
exit 0
