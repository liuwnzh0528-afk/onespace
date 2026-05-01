#!/bin/sh
set -eu

STATE_DIR="${ONESPACE_STATE_DIR:-/workspace/.onespace}"
PID_FILE="$STATE_DIR/service.pid"
LOG_FILE="$STATE_DIR/service.log"

mkdir -p "$STATE_DIR"

case "${1:-}" in
  start)
    shift
    if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
      echo "service already running"
      exit 0
    fi
    sh -lc "$*" >>"$LOG_FILE" 2>&1 &
    echo "$!" >"$PID_FILE"
    ;;
  stop)
    if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
      kill "$(cat "$PID_FILE")"
      rm -f "$PID_FILE"
    fi
    ;;
  status)
    if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
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
