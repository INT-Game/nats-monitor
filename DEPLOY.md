# NATS Monitor Deployment

## Configuration

Edit `config.json` before starting:

```json
{
  "listen_addr": "0.0.0.0:18222",
  "nats_monitor_url": "http://127.0.0.1:8222",
  "nats_monitor_targets": [
    {"name": "本机 NATS", "url": "http://127.0.0.1:8222"},
    {"name": "测试环境", "url": "http://10.0.0.12:8222"}
  ],
  "refresh_interval": "5s",
  "auth": {
    "username": "admin",
    "password": "replace-with-a-strong-password",
    "session_ttl": "12h",
    "secure_cookie": false
  }
}
```

- Change the packaged password before deployment.
- Set `secure_cookie` to `true` after HTTPS is configured at the reverse proxy.
- Keep the NATS monitoring port `8222` private. Only expose this authenticated dashboard.
- `LISTEN_ADDR`, `NATS_MONITOR_URL`, `REFRESH_INTERVAL`, `MONITOR_USERNAME`,
  and `MONITOR_PASSWORD` can override the file values.
- `nats_monitor_targets` supplies named choices in the switch dialog. The dialog
  also accepts a temporary HTTP(S) address. Restarting restores
  `nats_monitor_url` as the default target.

## Start

Windows:

```powershell
.\start.bat
```

CentOS/Linux:

```bash
chmod +x nats-monitor start.sh
./start.sh start
./start.sh status
./start.sh restart
./start.sh stop
```

Logs are written to `logs/nats-monitor.log` and the process ID to
`run/nats-monitor.pid`.

Open `http://SERVER_IP:18222` after startup.
