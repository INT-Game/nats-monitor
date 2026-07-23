#!/bin/sh
set -eu

BASE_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
APP="$BASE_DIR/nats-monitor"
CONFIG="$BASE_DIR/config.json"
RUN_DIR="$BASE_DIR/run"
LOG_DIR="$BASE_DIR/logs"
PID_FILE="$RUN_DIR/nats-monitor.pid"
LOG_FILE="$LOG_DIR/nats-monitor.log"

is_running() {
  [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null
}

start() {
  if is_running; then
    echo "nats-monitor is already running (pid $(cat "$PID_FILE"))"
    return 0
  fi

  mkdir -p "$RUN_DIR" "$LOG_DIR"
  rm -f "$PID_FILE"
  nohup "$APP" -config "$CONFIG" >>"$LOG_FILE" 2>&1 &
  echo $! >"$PID_FILE"
  sleep 1

  if ! is_running; then
    echo "nats-monitor failed to start; check $LOG_FILE"
    rm -f "$PID_FILE"
    return 1
  fi
  echo "nats-monitor started (pid $(cat "$PID_FILE"))"
}

stop() {
  if ! is_running; then
    echo "nats-monitor is not running"
    rm -f "$PID_FILE"
    return 0
  fi

  pid=$(cat "$PID_FILE")
  kill "$pid"
  count=0
  while kill -0 "$pid" 2>/dev/null && [ "$count" -lt 10 ]; do
    sleep 1
    count=$((count + 1))
  done
  if kill -0 "$pid" 2>/dev/null; then
    echo "nats-monitor did not stop within 10 seconds (pid $pid)"
    return 1
  fi
  rm -f "$PID_FILE"
  echo "nats-monitor stopped"
}

status() {
  if is_running; then
    echo "nats-monitor is running (pid $(cat "$PID_FILE"))"
  else
    echo "nats-monitor is not running"
    return 1
  fi
}

case "${1:-start}" in
  start) start ;;
  stop) stop ;;
  restart) stop; start ;;
  status) status ;;
  *) echo "usage: $0 {start|stop|restart|status}"; exit 2 ;;
esac
