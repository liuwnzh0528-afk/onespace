#!/bin/sh
set -eu

STATE_DIR="${ONESPACE_STATE_DIR:-/workspace/.onespace}"
PID_FILE="$STATE_DIR/service.pid"
LOG_FILE="$STATE_DIR/service.log"

mkdir -p "$STATE_DIR"

is_running() {
  pid="$1"
  kill -0 "$pid" 2>/dev/null || return 1
  state="$(ps -o stat= -p "$pid" 2>/dev/null | awk '{print $1}')"
  case "$state" in
    Z*) return 1 ;;
  esac
  return 0
}

case "${1:-}" in
  start)
    shift
    if [ -f "$PID_FILE" ]; then
      if is_running "$(cat "$PID_FILE")"; then
        echo "service already running"
        exit 0
      fi
      rm -f "$PID_FILE"
    fi
    sh -c "exec $*" >>"$LOG_FILE" 2>&1 &
    echo "$!" >"$PID_FILE"
    ;;
  stop)
    if [ -f "$PID_FILE" ] && is_running "$(cat "$PID_FILE")"; then
      kill "$(cat "$PID_FILE")"
      for _ in 1 2 3 4 5; do
        if ! is_running "$(cat "$PID_FILE")"; then
          break
        fi
        sleep 1
      done
      if is_running "$(cat "$PID_FILE")"; then
        kill -9 "$(cat "$PID_FILE")"
      fi
    fi
    rm -f "$PID_FILE"
    ;;
  status)
    if [ -f "$PID_FILE" ] && is_running "$(cat "$PID_FILE")"; then
      echo "running"
    else
      echo "stopped"
    fi
    ;;
  logs)
    if [ -f "$LOG_FILE" ]; then
      tail -n "${2:-200}" "$LOG_FILE"
    fi
    ;;
  *)
    echo "usage: onespace-supervisor.sh start <command> | stop | status | logs [lines]" >&2
    exit 2
    ;;
esac
