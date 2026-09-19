#!/bin/bash
# 素见 启动/停止脚本
# 用法：
#   ./start.sh killport     # 强制释放被占端口
# 端口优先级：--port > PORT 环境变量 > 默认 8099
set -u
cd "$(dirname "$0")"

# 解析公共参数：--port NNN
while [[ $# -gt 0 ]]; do
  case "$1" in
    --port) PORT="$2"; shift 2 ;;
    --port=*) PORT="${1#*=}"; shift ;;
    --) shift; break ;;
    -*) echo "未知参数: $1"; exit 1 ;;
    *) break ;;  # 第一个非 flag 参数是子命令
  esac
done

APP="sujian-server"
PORT="${PORT:-8099}"
PIDFILE="data/sujian.pid"
LOGFILE="data/sujian.log"
mkdir -p data uploads

is_running() {
  [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null
}

lan_ip() {
  local ip
  ip=$(ipconfig getifaddr en0 2>/dev/null)
  [ -z "$ip" ] && ip=$(ipconfig getifaddr en1 2>/dev/null)
  echo "$ip"
}

start() {
  if is_running; then
    echo "素见已在运行 (PID $(cat "$PIDFILE"))，端口 $PORT"
    echo "  本机:   http://localhost:$PORT"
    return 0
  fi
  echo "==> 编译中…"
  go build -o "$APP" . || { echo "编译失败"; exit 1; }
  echo "==> 后台启动 (端口 $PORT)…"
  nohup "./$APP" --port "$PORT" > "$LOGFILE" 2>&1 &
  echo $! > "$PIDFILE"
  sleep 1
  if is_running; then
    echo "素见已启动 ✓"
    echo "  本机:   http://localhost:$PORT"
    local ip; ip=$(lan_ip)
    [ -n "$ip" ] && echo "  局域网: http://$ip:$PORT"
    echo "  日志:   $LOGFILE  （停止: ./start.sh stop）"
  else
    echo "启动失败，日志见：$LOGFILE"
    if lsof -ti :"$PORT" >/dev/null 2>&1; then
      echo "提示：端口 $PORT 似被其它进程占用，可先执行  ./start.sh killport  释放后再启动"
    fi
    rm -f "$PIDFILE"
    exit 1
  fi
}

killport() {
  local holder
  holder=$(lsof -ti :"$PORT" 2>/dev/null)
  if [ -n "$holder" ]; then
    echo "端口 $PORT 被进程占用: $holder"
    kill $holder 2>/dev/null && echo "已发送终止信号，等待退出…" || echo "无法终止(权限不足?)"
    sleep 1
    holder=$(lsof -ti :"$PORT" 2>/dev/null)
    if [ -n "$holder" ]; then
      kill -9 $holder 2>/dev/null && echo "已强制终止: $holder"
    else
      echo "端口已释放 ✓"
    fi
  else
    echo "端口 $PORT 当前未被占用"
  fi
}

stop() {
  if is_running; then
    local pid; pid=$(cat "$PIDFILE")
    kill "$pid"
    # 等待进程退出
    for _ in 1 2 3 4 5; do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.5
    done
    kill -9 "$pid" 2>/dev/null
    rm -f "$PIDFILE"
    echo "素见已停止 ✓"
  else
    rm -f "$PIDFILE"
    echo "素见未在运行"
  fi
}

status() {
  if is_running; then
    echo "运行中 (PID $(cat "$PIDFILE"))，端口 $PORT"
  else
    echo "未运行"
  fi
}

case "${1:-start}" in
  start)   start ;;
  stop)    stop ;;
  restart) stop; sleep 1; start ;;
  killport) killport ;;
  status)  status ;;
  *) echo "用法: ./start.sh [--port NNN] [start|stop|restart|status]" ; exit 1 ;;
esac
