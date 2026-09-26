#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"
sensor_hub=$(cd .. && pwd)
api=http://localhost:8080/api/health
ui=http://localhost:3000/
work=$(mktemp -d)
watch_pid=
declare -A result
declare -A backup

keep() { backup[$1]="$work/$(basename "$1")"; cp "$1" "${backup[$1]}"; }
put_back() { cp "${backup[$1]}" "$1"; unset "backup[$1]"; }

cleanup() {
  for file in "${!backup[@]}"; do cp "${backup[$file]}" "$file"; done
  [ -n "$watch_pid" ] && kill "$watch_pid" 2>/dev/null || true
  jobs -p | xargs -r kill 2>/dev/null || true
  rm -rf "$work"
}
trap cleanup EXIT

now() { date +%s%3N; }
seconds() { local ms=$(( $(now) - $1 )); printf '%d.%02d' $((ms / 1000)) $(((ms % 1000) / 10)); }
answers() { curl -fsS -o /dev/null --max-time 2 "$1" 2>/dev/null; }
log() { echo "$(date +%T) $*"; }

wait_until() {
  local deadline=$(( $(now) + 900000 ))
  until "$@"; do
    if (( $(now) > deadline )); then
      echo "timed out waiting for: $*" >&2
      exit 1
    fi
    sleep 0.05
  done
}

container() { docker compose ps -q "$1"; }
replaced() { local id; id=$(container "$1"); [ -n "$id" ] && [ "$id" != "$2" ] && answers "$3"; }
restarts() { grep -c 'running\.\.\.' "$work/sensor-hub.log" || true; }
restarted() { (( $(restarts) > $1 )) && answers "$api"; }

up_times() {
  local start=$1
  wait_until answers "$api"
  local api_s; api_s=$(seconds "$start")
  wait_until answers "$ui"
  echo "$api_s $(seconds "$start")"
}

log "pulling base images"
grep -h '^FROM' dockerfiles/*.dockerfile | awk '{print $2}' | sort -u | xargs -n1 docker pull -q >/dev/null

log "clearing this stack's containers, volumes, images and build cache mounts"
docker compose down -v --rmi local >/dev/null 2>&1
docker buildx du --verbose --filter type=exec.cachemount \
  | awk '/^ID:/ {id = $2} /with id "sensor-hub-dev-/ {print id}' \
  | xargs -r -I{} docker buildx prune -f --filter id={} >/dev/null

log "L1 cold: build --no-cache, then up"
start=$(now)
docker compose build --no-cache >"$work/l1-build.log" 2>&1
build_s=$(seconds "$start")
docker compose up -d >/dev/null 2>&1
read -r api_s ui_s < <(up_times "$start")
log "L1 build=${build_s}s api=${api_s}s ui=${ui_s}s"
result[L1]="$api_s"

for run in 1 2; do
  docker compose down >/dev/null 2>&1
  start=$(now)
  docker compose up --build -d >"$work/l2-build.log" 2>&1
  read -r api_s ui_s < <(up_times "$start")
  log "L2 warm up --build run$run api=${api_s}s ui=${ui_s}s"
  result["L2 run$run"]="$api_s"
done

for run in 1 2; do
  docker compose down >/dev/null 2>&1
  start=$(now)
  docker compose up -d >/dev/null 2>&1
  read -r api_s ui_s < <(up_times "$start")
  log "L3 down then up run$run api=${api_s}s ui=${ui_s}s"
  result["L3 run$run"]="$api_s"
done

docker compose watch --no-up >"$work/watch.log" 2>&1 &
watch_pid=$!
wait_until grep -q 'Watch enabled' "$work/watch.log"
docker compose logs -f --no-log-prefix --since 1s sensor-hub >"$work/sensor-hub.log" 2>&1 &

health_go="$sensor_hub/api/health_api.go"
keep "$health_go"
sleep 5
l4=()
for run in 1 2 3 4 5; do
  before=$(restarts)
  echo "// devstack-timing $run" >>"$health_go"
  start=$(now)
  wait_until restarted "$before"
  took=$(seconds "$start")
  log "L4 go edit run$run api back=${took}s"
  l4+=("$took")
  sleep 5
done
put_back "$health_go"
result["L4 median"]="$(printf '%s\n' "${l4[@]}" | sort -n | sed -n 3p)"
sleep 5

rebuild_after_edit() {
  local service=$1 file=$2 url=$3 loop=$4
  keep "$file"
  local id; id=$(container "$service")
  echo "" >>"$file"
  local start; start=$(now)
  wait_until replaced "$service" "$id" "$url"
  local took; took=$(seconds "$start")
  log "$loop $(basename "$file") change picked up by watch, back=${took}s"
  result["$loop"]="$took"
  id=$(container "$service")
  put_back "$file"
  wait_until replaced "$service" "$id" "$url"
}

rebuild_after_edit sensor-hub "$sensor_hub/go.mod" "$api" L5a
rebuild_after_edit ui "$sensor_hub/ui/sensor_hub_ui/package-lock.json" "$ui" L5b

kill "$watch_pid"
watch_pid=
docker compose down >/dev/null 2>&1

declare -A target=(
  [L1]=203 ["L2 run1"]=30 ["L2 run2"]=30 ["L3 run1"]=20 ["L3 run2"]=20
  ["L4 median"]=2.5 [L5a]=201 [L5b]=167
)
echo
printf '%-10s %10s %10s  %s\n' loop measured target result
for loop in L1 "L2 run1" "L2 run2" "L3 run1" "L3 run2" "L4 median" L5a L5b; do
  verdict=$(awk -v m="${result[$loop]}" -v t="${target[$loop]}" 'BEGIN { print (m <= t) ? "pass" : "FAIL" }')
  printf '%-10s %9ss %9ss  %s\n' "$loop" "${result[$loop]}" "${target[$loop]}" "$verdict"
done
