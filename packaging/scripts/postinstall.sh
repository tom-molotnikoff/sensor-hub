#!/bin/bash
install -d -m 0750 -o sensor-hub -g sensor-hub /var/lib/sensor-hub
install -d -m 0750 -o sensor-hub -g sensor-hub /var/log/sensor-hub

CONFIG_DIR=/etc/sensor-hub
# init-key exits with this status when a key already exists in any form.
KEY_EXISTS=3

is_upgrade() {
  # RPM: $1=2 on upgrade
  [ "$1" = "2" ] && return 0
  # DEB: $1=configure and $2 is the old version
  [ "$1" = "configure" ] && [ -n "$2" ] && return 0
  return 1
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

ensure_secrets_key

systemctl daemon-reload

if is_upgrade "$@"; then
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
