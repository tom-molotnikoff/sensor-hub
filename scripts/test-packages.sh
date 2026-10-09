#!/usr/bin/env bash
#
# Install a server package under a real systemd in a container and check that
# the hub comes up with no operator step: as an upgrade from a 1.5.x release
# on a host with and without a TPM, then with a TPM that refuses the key and
# one that cannot be used at all, and as a fresh install.
#
# Usage:
#   scripts/test-packages.sh <package.deb|package.rpm> [--from <version>]
#
# A .deb is installed on Debian and a .rpm on Fedora. The release upgraded
# from (default 1.5.2) is downloaded from GitHub. Needs curl and jq, and a
# Docker that can run privileged containers.
#
# A container has no TPM, so the TPM case puts a stand-in systemd-creds in
# front of the real one: it reports a TPM and seals with systemd's host key in
# its place. Everything else, including systemd decrypting the credential for
# the unit, is the real thing.
#
set -euo pipefail

REPO="tom-molotnikoff/sensor-hub"
FROM_VERSION="1.5.2"
PACKAGE=""

usage() {
  sed -n '3,18p' "$0" | sed -E 's/^# ?//'
  exit "${1:-0}"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --from)    FROM_VERSION="$2"; shift 2 ;;
    -h|--help) usage ;;
    -*)        echo "Unknown option: $1" >&2; usage 1 ;;
    *)         PACKAGE="$1"; shift ;;
  esac
done
[[ -f "$PACKAGE" ]] || { echo "Error: package file required" >&2; usage 1; }

case "$PACKAGE" in
  *.deb) FORMAT=deb ;;
  *.rpm) FORMAT=rpm ;;
  *) echo "Error: $PACKAGE is neither a .deb nor a .rpm" >&2; exit 1 ;;
esac

case "$(docker info --format '{{.Architecture}}')" in
  x86_64)  ARCH=amd64 ;;
  aarch64) ARCH=arm64 ;;
  *) echo "Error: unsupported Docker architecture" >&2; exit 1 ;;
esac

WORK_DIR="$(mktemp -d)"
CONTAINER=""
cleanup() {
  [[ -n "$CONTAINER" ]] && docker rm -f "$CONTAINER" >/dev/null 2>&1
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

cp "$PACKAGE" "$WORK_DIR/new.$FORMAT"
OLD_PACKAGE_URL="https://github.com/$REPO/releases/download/v$FROM_VERSION/sensor-hub_${FROM_VERSION}_linux_${ARCH}.$FORMAT"
echo "==> Downloading $OLD_PACKAGE_URL"
curl -fsSL -o "$WORK_DIR/old.$FORMAT" "$OLD_PACKAGE_URL"

# --- Host image: the distribution with systemd as PID 1 ---
IMAGE="sensor-hub-package-test:$FORMAT"
if [[ "$FORMAT" == deb ]]; then
  INSTALL="dpkg -i"
  UPGRADE="dpkg -i"
  DOCKERFILE='FROM debian:bookworm
RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends systemd systemd-sysv adduser curl ca-certificates \
 && rm -rf /var/lib/apt/lists/*'
else
  INSTALL="rpm -i"
  UPGRADE="rpm -U"
  DOCKERFILE='FROM fedora:42
RUN dnf -y install systemd shadow-utils util-linux && dnf clean all'
fi
# On a real host systemd makes / a shared mount, which is how it hands a unit
# its credentials; it skips that in a container, so it is done here first.
DOCKERFILE="$DOCKERFILE
STOPSIGNAL SIGRTMIN+3
CMD [\"/bin/sh\", \"-c\", \"mount --make-rshared / && exec /sbin/init\"]"

echo "==> Building $IMAGE"
docker build -q -t "$IMAGE" - <<<"$DOCKERFILE" >/dev/null

# --- Helpers ---
# Failures are counted in a file, so that the ones in a step's subshell count.
FAILURE_LOG="$WORK_DIR/failures"
: >"$FAILURE_LOG"

on_host() {
  docker exec "$CONTAINER" bash -c "$1"
}

fail() {
  echo "    FAIL: $*" >&2
  echo "$*" >>"$FAILURE_LOG"
}

failure_count() {
  awk 'END { print NR }' "$FAILURE_LOG"
}

check() {
  local description="$1" expected="$2" actual="$3"
  if [[ "$expected" == "$actual" ]]; then
    echo "    ok: $description"
  else
    fail "$description: expected [$expected], got [$actual]"
  fi
}

start_host() {
  CONTAINER="$(docker run -d --privileged --cgroupns=host \
    -v /sys/fs/cgroup:/sys/fs/cgroup:rw --tmpfs /run --tmpfs /run/lock "$IMAGE")"
  docker cp -q "$WORK_DIR/." "$CONTAINER:/packages"
  # "is-system-running --wait" waits for boot to finish, but only once it can
  # reach systemd, which listens on its private socket a moment after start.
  local state _
  for _ in $(seq 60); do
    on_host 'test -S /run/systemd/private' && break
    sleep 1
  done
  state="$(on_host 'systemctl is-system-running --wait' || true)"
  [[ "$state" == running || "$state" == degraded ]] || { fail "systemd did not start: $state"; return 1; }
}

stop_host() {
  docker rm -f "$CONTAINER" >/dev/null
  CONTAINER=""
}

# A stand-in TPM, installed in place of systemd-creds so it is found on any
# PATH, including the fixed one rpm gives its scriptlets.
fake_tpm() {
  docker exec -i "$CONTAINER" bash -c 'mv /usr/bin/systemd-creds /usr/bin/systemd-creds.real && cat > /usr/bin/systemd-creds && chmod 0755 /usr/bin/systemd-creds' <<'EOF'
#!/bin/sh
# /root/fake-tpm-gone stands for a TPM that cannot be used at all.
if [ "$1" = has-tpm2 ]; then [ -e /root/fake-tpm-gone ] && exit 1; exit 0; fi
for arg; do
  shift
  [ "$arg" = --with-key=tpm2 ] && arg=--with-key=host
  set -- "$@" "$arg"
done
exec /usr/bin/systemd-creds.real "$@"
EOF
}

# restart_hub restarts the hub. The scenarios restart it more often than
# systemd's default start limit allows (five starts in ten seconds), and
# systemd refuses a start over the limit for good rather than retrying it, so
# each restart clears the count first.
restart_hub() {
  on_host 'systemctl reset-failed sensor-hub && systemctl restart sensor-hub' \
    || { fail "restarting the hub"; show_service; return 1; }
}

# show_service prints how systemd last ran the hub and its key check, after a
# check on the service failed. The hub logs to the journal as well as its file.
show_service() {
  on_host 'systemctl show -p ActiveState -p Result -p NRestarts sensor-hub; journalctl --no-pager -n 40 -u sensor-hub -u sensor-hub-key-check' >&2 || true
}

# wait_healthy waits for the service to answer and checks systemd did not
# have to restart it on the way.
wait_healthy() {
  local _ state
  for _ in $(seq 60); do
    if on_host 'curl -fsS http://127.0.0.1:8080/api/health' >/dev/null 2>&1; then
      # shellcheck disable=SC2016 # expanded on the host under test
      state="$(on_host 'echo $(systemctl is-active sensor-hub) $(systemctl show -p NRestarts --value sensor-hub)')"
      check "the service runs without restarts" "active 0" "$state"
      [[ "$state" == "active 0" ]] || show_service
      return 0
    fi
    sleep 1
  done
  fail "the service did not answer on /api/health"
  show_service
  return 1
}

# run_steps runs a scenario's steps in order and stops at the first that
# cannot go on, so the scenario still reaches its cleanup and the scenarios
# after it still run. Each step runs in a subshell with errexit on, so a
# command that fails partway through stops the step. Run any other way, as
# the condition of an if for one, bash would ignore errexit in the whole step.
run_steps() {
  local step failures level status
  for step; do
    failures="$(failure_count)"
    level=$((BASH_SUBSHELL + 1))
    set +e
    (
      set -eE
      trap 'step_stopped "$BASH_COMMAND"' ERR
      "$step"
    )
    status=$?
    set -e
    if [[ "$status" -ne 0 ]]; then
      [[ "$(failure_count)" -gt "$failures" ]] || fail "$step stopped with status $status"
      echo "    the rest of this scenario's steps are skipped" >&2
      return 0
    fi
  done
}

# step_stopped counts the command that stopped a step, and where it was,
# unless the step already counted a failed check of its own. Failures inside a
# command substitution are left to the command that uses its output.
step_stopped() {
  [[ "$BASH_SUBSHELL" -eq "$level" ]] || return 0
  [[ "$(failure_count)" -gt "$failures" ]] && return 0
  local i=1 trace=""
  while [[ "${FUNCNAME[i]}" != run_steps ]]; do
    trace+=" in ${FUNCNAME[i]} at line ${BASH_LINENO[i - 1]}"
    i=$((i + 1))
  done
  fail "$step stopped: $1 failed$trace"
}

ADMIN_PASSWORD="package-test-password"
OPERATOR_SETTING="SENSOR_HUB_PACKAGE_TEST=kept"
COLLECTION_INTERVAL="123"
BROKER_PASSWORD="package-test-broker-secret"
SMTP_USER="package-test@example.com"

# api METHOD PATH [BODY] calls the hub as the admin, keeping the session in a
# cookie jar on the host under test.
api() {
  local csrf=""
  [[ -f "$WORK_DIR/csrf" ]] && csrf="$(cat "$WORK_DIR/csrf")"
  # shellcheck disable=SC2016 # the body is single-quoted for the host's shell
  on_host "curl -sS -b /root/jar -c /root/jar -H 'Content-Type: application/json' -H 'X-CSRF-Token: $csrf' \
    -X $1 http://127.0.0.1:8080/api$2 ${3:+-d '$3'}"
}

login() {
  rm -f "$WORK_DIR/csrf"
  local response
  response="$(api POST /auth/login "{\"username\":\"admin\",\"password\":\"$ADMIN_PASSWORD\"}")"
  jq -r '.csrf_token // empty' <<<"$response" >"$WORK_DIR/csrf"
  [[ -s "$WORK_DIR/csrf" ]] || { fail "admin login: $response"; return 1; }
  echo "$response"
}

# Before the upgrade: the configuration changed the way an operator and the
# hub change it, an admin, and an outbound broker with a password the upgrade
# has to carry into the secret store. Up to 1.5.x the package owned the
# configuration files, so each change is one the upgrade must not ask about.
seed_old_install() {
  on_host "printf 'SENSOR_HUB_INITIAL_ADMIN=admin:$ADMIN_PASSWORD\n$OPERATOR_SETTING\n' >> /etc/sensor-hub/environment"
  on_host 'systemctl start sensor-hub'
  wait_healthy || return 1
  if [[ "$(login | jq -r .must_change_password)" == true ]]; then
    api PUT /users/password "{\"new_password\":\"$ADMIN_PASSWORD\"}" >/dev/null
  fi
  # The hub's properties saver rewrites application.properties.
  api PATCH /properties "{\"sensor.collection.interval\":\"$COLLECTION_INTERVAL\"}" >/dev/null
  check "the hub saved the property" "yes" \
    "$(on_host "grep -qx 'sensor.collection.interval=$COLLECTION_INTERVAL' /etc/sensor-hub/application.properties && echo yes || echo no")"
  api POST /mqtt/brokers "{\"name\":\"home\",\"type\":\"external\",\"host\":\"192.0.2.1\",\"port\":1883,\"username\":\"hub\",\"password\":\"$BROKER_PASSWORD\",\"enabled\":false}" >/dev/null
  # Email went through Gmail: the sender in smtp.properties, and the OAuth
  # client and token beside it.
  api PATCH /properties "{\"smtp.user\":\"$SMTP_USER\"}" >/dev/null
  check "the hub saved smtp.user" "yes" \
    "$(on_host "grep -qx 'smtp.user=$SMTP_USER' /etc/sensor-hub/smtp.properties && echo yes || echo no")"
  on_host 'cd /etc/sensor-hub && echo "{}" > credentials.json && echo "{}" > token.json && chown sensor-hub:sensor-hub credentials.json token.json'
}

# The upgrade runs with no terminal, as an unattended upgrade does, so a
# question about a configuration file would fail it.
upgrade_package() {
  on_host "DEBIAN_FRONTEND=noninteractive $UPGRADE /packages/new.$FORMAT </dev/null"
}

# After the upgrade the changed configuration is still there and in effect,
# and nothing is left beside it.
check_configuration_kept() {
  check "the operator's setting is in the hub's environment" "yes" \
    "$(on_host "tr '\\0' '\\n' < /proc/\$(systemctl show -p MainPID --value sensor-hub)/environ | grep -qx '$OPERATOR_SETTING' && echo yes || echo no")"
  check "the property the hub saved is in effect" "$COLLECTION_INTERVAL" \
    "$(api GET /properties | jq -r '."sensor.collection.interval"')"
  # 2.0 no longer ships smtp.properties, but leaves the 1.5.x one in place.
  check "the configuration files" \
    "640 sensor-hub:sensor-hub application.properties
640 sensor-hub:sensor-hub database.properties
640 sensor-hub:sensor-hub environment
640 sensor-hub:sensor-hub smtp.properties" \
    "$(on_host 'cd /etc/sensor-hub && stat -c "%a %U:%G %n" application.properties database.properties environment smtp.properties')"
  check "no saved or new copies beside them" "" \
    "$(on_host 'ls /etc/sensor-hub | grep -E "\.(rpmsave|rpmnew|dpkg-[a-z]+)$" || true')"
}

# Configuration files nobody changed take the new defaults, as a config file
# upgrade did.
check_new_defaults() {
  local name
  for name in environment application.properties database.properties; do
    check "$name is the new default" "same" \
      "$(on_host "cmp -s /etc/sensor-hub/$name /usr/share/sensor-hub/defaults/$name && echo same || echo different")"
  done
  check "no saved or new copies beside them" "" \
    "$(on_host 'ls /etc/sensor-hub | grep -E "\.(rpmsave|rpmnew|dpkg-[a-z]+)$" || true')"
}

# After the upgrade: logins work and the broker password is held, encrypted
# under the key the hub loaded, and readable again after another restart.
check_upgraded_install() {
  login >/dev/null || return 1
  check_configuration_kept
  check "the broker password is in the secret store" "set" \
    "$(api GET /mqtt/brokers | jq -r '.[] | select(.name == "home") | .password_status')"
  check "the plaintext password is gone from the database and its WAL" "0" \
    "$(on_host "cat /var/lib/sensor-hub/sensor_hub.db* | grep -ac '$BROKER_PASSWORD' || true")"
  check "smtp.user is carried into the email settings" "$SMTP_USER $SMTP_USER" \
    "$(api GET /email/smtp | jq -r '"\(.username) \(.from_address)"')"
  check "the Gmail OAuth files are deleted" "" \
    "$(on_host 'cd /etc/sensor-hub && ls credentials.json token.json 2>/dev/null | xargs')"
  restart_hub
  wait_healthy || return 1
  login >/dev/null || return 1
  check "the key survives a restart" "set" \
    "$(api GET /mqtt/brokers | jq -r '.[] | select(.name == "home") | .password_status')"
}

# hub_logged says whether the hub logged loading its key from the source.
hub_logged() {
  on_host "grep -F 'loaded the secret-store key' /var/log/sensor-hub/sensor-hub.log | grep -qF '$1' && echo yes || echo no"
}

check_key_file() {
  check "the key file" "600 sensor-hub:sensor-hub" "$(on_host 'stat -c "%a %U:%G" /etc/sensor-hub/secrets.key')"
  check "no sealed key" "no" "$(on_host 'test -e /etc/sensor-hub/secrets.key.cred && echo yes || echo no')"
  check "the hub loaded the key file" "yes" "$(hub_logged 'configuration directory')"
}

check_sealed_key() {
  check "the sealed key" "600 root:root" "$(on_host 'stat -c "%a %U:%G" /etc/sensor-hub/secrets.key.cred')"
  check "the drop-in" "$(printf '[Service]\nLoadCredentialEncrypted=secrets.key:/etc/sensor-hub/secrets.key.cred')" \
    "$(on_host 'cat /etc/systemd/system/sensor-hub.service.d/secrets-key.conf')"
  check "no plaintext key file" "no" "$(on_host 'test -e /etc/sensor-hub/secrets.key && echo yes || echo no')"
  check "the hub loaded the systemd credential" "yes" "$(hub_logged 'systemd credential')"
  check "show-key as root prints the key the unit is given" \
    "$(on_host 'cat /run/credentials/sensor-hub.service/secrets.key')" \
    "$(on_host 'sensor-hub local secrets show-key')"
}

# A TPM that will not unseal the key after a firmware or boot change. systemd
# fails a unit whose credential will not decrypt, so the key check set aside
# the key and sealed a new one before the hub started: the hub comes up, the
# broker password asks to be entered again, and entering it puts it back.
check_tpm_refusal() {
  local old_key new_key broker_id
  old_key="$(on_host 'sensor-hub local secrets show-key')"
  # The stand-in TPM seals with systemd's host key, so a new host key is a TPM
  # that no longer unseals what it sealed.
  on_host 'mv /var/lib/systemd/credential.secret /root/credential.secret.before && systemd-creds setup >/dev/null 2>&1'
  # A start of the check while the hub runs, as "systemctl start sensor-hub"
  # on a running hub pulls in, leaves the key the hub holds alone.
  on_host 'systemctl start sensor-hub-key-check.service'
  check "the check leaves a running hub's key alone" "0" \
    "$(on_host 'ls /etc/sensor-hub/secrets.key.cred.unsealable-* 2>/dev/null | wc -l')"
  restart_hub
  wait_healthy || return 1
  login >/dev/null || return 1
  new_key="$(on_host 'sensor-hub local secrets show-key')"
  check "a new key is sealed" "yes" "$([[ -n "$new_key" && "$new_key" != "$old_key" ]] && echo yes || echo no)"
  check "the unit is given the new key" "$new_key" "$(on_host 'cat /run/credentials/sensor-hub.service/secrets.key')"
  check "the old sealed key is kept" "1" "$(on_host 'ls /etc/sensor-hub/secrets.key.cred.unsealable-* | wc -l')"
  check "the broker password needs re-entry" "needs_reentry" \
    "$(api GET /mqtt/brokers | jq -r '.[] | select(.name == "home") | .password_status')"
  check "one secret_failure notification" "1" \
    "$(api GET '/notifications?limit=100' | jq '[.[] | select(.notification.category == "secret_failure")] | length')"
  broker_id="$(api GET /mqtt/brokers | jq -r '.[] | select(.name == "home") | .id')"
  api PUT "/mqtt/brokers/$broker_id" "{\"name\":\"home\",\"type\":\"external\",\"host\":\"192.0.2.1\",\"port\":1883,\"username\":\"hub\",\"password\":\"$BROKER_PASSWORD\",\"enabled\":false}" >/dev/null
  check "entering the password again stores it" "set" \
    "$(api GET /mqtt/brokers | jq -r '.[] | select(.name == "home") | .password_status')"
  restart_hub
  wait_healthy || return 1
  login >/dev/null || return 1
  check "the new key survives a restart" "set" \
    "$(api GET /mqtt/brokers | jq -r '.[] | select(.name == "home") | .password_status')"
  check "the old sealed key is set aside once" "1" "$(on_host 'ls /etc/sensor-hub/secrets.key.cred.unsealable-* | wc -l')"
}

# A TPM that cannot be used at all. The check leaves the key alone while the
# TPM may still appear, the hub's unit fails on its credential and restarts,
# and once the grace period (shortened here) is over the check seals a new key
# with the host's credential secret and the hub says so.
check_tpm_gone() {
  local _ restarts failures
  on_host 'mkdir -p /etc/systemd/system/sensor-hub-key-check.service.d && printf "[Service]\nExecStart=\nExecStart=/usr/bin/sensor-hub local secrets check-seal --config-dir=/etc/sensor-hub --tpm-grace=20s\n" > /etc/systemd/system/sensor-hub-key-check.service.d/grace.conf && systemctl daemon-reload'
  on_host 'touch /root/fake-tpm-gone && mv /var/lib/systemd/credential.secret /root/credential.secret.gone && systemd-creds setup >/dev/null 2>&1'
  restart_hub
  # shellcheck disable=SC2016 # expanded on the host under test
  check "the key is left alone while the TPM may still appear" "1 yes" \
    "$(on_host 'echo $(ls /etc/sensor-hub/secrets.key.cred.unsealable-* | wc -l) $(test -e /run/sensor-hub-key-check/tpm-unusable-since && echo yes || echo no)')"
  for _ in $(seq 120); do
    on_host 'curl -fsS http://127.0.0.1:8080/api/health' >/dev/null 2>&1 && break
    sleep 1
  done
  failures="$(failure_count)"
  restarts="$(on_host 'systemctl show -p NRestarts --value sensor-hub')"
  check "the hub restarted while it waited for the TPM" "yes" "$([[ "$restarts" -gt 0 ]] && echo yes || echo no)"
  check "the service runs once the grace period is over" "active" "$(on_host 'systemctl is-active sensor-hub')"
  [[ "$(failure_count)" -eq "$failures" ]] || show_service
  login >/dev/null || return 1
  # shellcheck disable=SC2016 # expanded on the host under test
  check "the new key is sealed with the host's credential secret" "2 host" \
    "$(on_host 'echo $(ls /etc/sensor-hub/secrets.key.cred.unsealable-* | wc -l) $(sed -n "s/^sealed_with=//p" /run/sensor-hub-key-check/key-replaced)')"
  check "the notification says the TPM no longer protects the key" "yes" \
    "$(api GET '/notifications?limit=100' | jq -r '[.[] | select(.notification.category == "secret_failure")] | max_by(.notification.id).notification.message' | grep -qF "protected by this host's credential secret, not the TPM" && echo yes || echo no)"
  check "the broker password needs re-entry" "needs_reentry" \
    "$(api GET /mqtt/brokers | jq -r '.[] | select(.name == "home") | .password_status')"
}

# remove_package removes the package as fully as the package manager can:
# dpkg --purge, or rpm -e, which has no purge. Purge takes the configuration
# files the package created; everything else stays, including the key, which
# goes only with the database it decrypts.
remove_package() {
  local key_files="$1" config_after
  if [[ "$FORMAT" == deb ]]; then
    on_host 'DEBIAN_FRONTEND=noninteractive dpkg --purge sensor-hub </dev/null' >/dev/null
    config_after=""
  else
    # rpm -e keeps the configuration, and the smtp.properties a 1.5.x install
    # left, which no package owns.
    config_after="application.properties database.properties environment$(on_host 'test -e /etc/sensor-hub/smtp.properties && echo " smtp.properties"' || true)"
    on_host 'rpm -e sensor-hub </dev/null' >/dev/null
  fi
  check "the configuration files after removal" "$config_after" \
    "$(on_host 'cd /etc/sensor-hub && ls application.properties database.properties environment smtp.properties 2>/dev/null | xargs')"
  check "the key after removal" "$key_files" \
    "$(on_host 'ls /etc/sensor-hub/secrets.key /etc/sensor-hub/secrets.key.cred /etc/systemd/system/sensor-hub.service.d/secrets-key.conf 2>/dev/null | xargs')"
  check "the database after removal" "yes" "$(on_host 'test -f /var/lib/sensor-hub/sensor_hub.db && echo yes || echo no')"
}

# --- Scenarios ---
scenario() {
  echo ""
  echo "==> $1"
}

scenario "Upgrade from $FROM_VERSION on a host without a TPM"
start_host
on_host "$INSTALL /packages/old.$FORMAT" >/dev/null
run_steps seed_old_install upgrade_package wait_healthy check_key_file check_upgraded_install
remove_package /etc/sensor-hub/secrets.key
stop_host

scenario "Upgrade from $FROM_VERSION on a host with a TPM"
start_host
fake_tpm
on_host "$INSTALL /packages/old.$FORMAT" >/dev/null
run_steps seed_old_install upgrade_package wait_healthy check_sealed_key check_upgraded_install \
  check_tpm_refusal check_tpm_gone
remove_package "/etc/sensor-hub/secrets.key.cred /etc/systemd/system/sensor-hub.service.d/secrets-key.conf"
stop_host

scenario "Upgrade from $FROM_VERSION with the configuration as it was installed"
start_host
on_host "$INSTALL /packages/old.$FORMAT" >/dev/null
on_host 'systemctl start sensor-hub'
run_steps wait_healthy upgrade_package wait_healthy check_new_defaults
stop_host

scenario "Fresh install on a host without a TPM"
start_host
on_host "$INSTALL /packages/new.$FORMAT"
check "the service is enabled" "enabled" "$(on_host 'systemctl is-enabled sensor-hub')"
on_host 'systemctl start sensor-hub'
run_steps wait_healthy check_key_file
remove_package /etc/sensor-hub/secrets.key
stop_host

echo ""
if [[ "$(failure_count)" -gt 0 ]]; then
  echo "==> $(failure_count) check(s) failed" >&2
  exit 1
fi
echo "==> All checks passed"
