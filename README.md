# NATS Monitor

一个使用 Go 编写的轻量 NATS 监控面板。程序直接读取 NATS Server 的 HTTP Monitoring API，提供运行状态、连接、Subject、慢消费者、集群拓扑、账号和 JetStream 数据展示。

前端资源已经嵌入 Go 二进制，部署时只需要可执行文件和 `config.json`，不需要 Node.js、数据库或额外的静态文件服务。

## 功能

- 运行总览：健康状态、连接数、消息速率、网络吞吐、订阅数、CPU、内存和运行时间。
- 趋势统计：保存最近一小时的消息速率、网络吞吐等采样数据。
- 连接明细：客户端名称、IP、账号、用户名、语言版本、订阅和流量信息。
- 慢消费者：当前存在 Pending 的风险连接，以及最近关闭连接中的已确认慢消费者。
- Subject 明细：展示订阅 Subject、Queue Group、账号、CID、SID 和投递统计。
- Subject 分类：区分业务 Subject、临时 `_INBOX` 和 `$SYS` 等系统 Subject，并使用固定排序。
- 账号统计：连接、订阅、消息、流量、Leaf Node 和慢消费者统计。
- 集群拓扑：Route、Gateway 和 Leaf Node 连接明细。
- JetStream：启用状态、Streams、Consumers、消息量、存储和 API 统计。
- 多目标切换：通过配置提供命名地址，也可以在页面临时输入任意 HTTP(S) 监控地址。
- 登录保护：支持用户名、密码、会话有效期、登录失败限制和安全 Cookie。
- 自动刷新：服务端定时采集，页面支持自动刷新和立即刷新。
- 响应式界面：适配桌面和移动端。

## 数据来源

本项目访问的是 NATS Server 的 HTTP Monitoring 端口，默认端口为 `8222`，不是客户端连接端口 `4222`。

采集的端点包括：

```text
/healthz
/varz
/connz?auth=true&subs=detail
/connz?state=closed&auth=true
/subsz?subs=1
/routez?subs=detail
/gatewayz
/leafz?subs=1
/accountz
/accstatz
/jsz
```

NATS HTTP Monitoring API 通常不提供独立身份认证。请将 `8222` 保持在内网，只对外开放本监控面板。官方说明参见 [NATS Monitoring](https://docs.nats.io/running-a-nats-service/nats_admin/monitoring)。

本项目不是 NATS 客户端、代理或消息抓包工具，不会订阅 Subject，也不会读取消息正文。

## 快速开始

### 1. 启用 NATS Monitoring

确保目标 NATS Server 已启用 HTTP Monitoring，例如：

```bash
nats-server -m 8222
```

已有 NATS 配置时，只需确保监控端口已启用，并且运行本面板的服务器能够访问该端口。

### 2. 准备配置

编辑项目目录中的 `config.json`：

```json
{
  "listen_addr": "0.0.0.0:18222",
  "nats_monitor_url": "http://127.0.0.1:8222",
  "nats_monitor_targets": [
    {
      "name": "本机 NATS",
      "url": "http://127.0.0.1:8222"
    },
    {
      "name": "测试环境",
      "url": "http://10.0.0.12:8222"
    }
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

部署前必须修改用户名和密码。程序拒绝使用默认密码 `CHANGE_ME` 启动。

### 3. 启动

Windows：

```powershell
.\start.bat
```

CentOS/Linux：

```bash
chmod +x nats-monitor start.sh
./start.sh start
./start.sh status
```

默认访问地址：

```text
http://SERVER_IP:18222
```

Linux 日志写入 `logs/nats-monitor.log`，PID 写入 `run/nats-monitor.pid`。

## 配置说明

| 配置项 | 默认值 | 说明 |
|---|---|---|
| `listen_addr` | `0.0.0.0:18222` | 面板 HTTP 监听地址 |
| `nats_monitor_url` | `http://127.0.0.1:8222` | 启动后默认监控目标 |
| `nats_monitor_targets` | `[]` | 页面下拉框中的命名目标列表 |
| `refresh_interval` | `5s` | 服务端采集周期，不能小于 `1s` |
| `auth.username` | `admin` | 面板登录用户名 |
| `auth.password` | 无可用默认值 | 面板登录密码 |
| `auth.session_ttl` | `12h` | 登录会话有效期，不能小于 `5m` |
| `auth.secure_cookie` | `false` | 使用 HTTPS 时应设置为 `true` |

监控地址规则：

- 支持 `http://` 和 `https://`。
- 未填写协议时自动使用 `http://`。
- 未填写端口时自动使用 `8222`。
- 不接受包含账号、查询参数、片段或具体 API 路径的地址。
- 页面临时切换只在当前进程内有效；服务重启后恢复 `nats_monitor_url`。

以下环境变量可以覆盖配置文件：

| 环境变量 | 对应配置 |
|---|---|
| `LISTEN_ADDR` | `listen_addr` |
| `NATS_MONITOR_URL` | `nats_monitor_url` |
| `REFRESH_INTERVAL` | `refresh_interval` |
| `MONITOR_USERNAME` | `auth.username` |
| `MONITOR_PASSWORD` | `auth.password` |

示例：

```bash
MONITOR_PASSWORD='your-password' ./nats-monitor -config ./config.json
```

## Linux 管理命令

```bash
./start.sh start
./start.sh status
./start.sh restart
./start.sh stop
```

查看日志：

```bash
tail -f logs/nats-monitor.log
```

升级运行中的 Go 二进制时，不要通过 SFTP 直接截断覆盖，否则可能收到 `SSH_FX_FAILURE` 或 `Text file busy`。先上传到同目录的临时文件，再原子替换：

```bash
chmod +x nats-monitor.new
mv -f nats-monitor.new nats-monitor
./start.sh restart
```

## 从源码构建

要求 Go `1.26.2` 或兼容版本。不需要生成 Proto，也不需要 Node.js。

运行测试：

```bash
go test ./...
```

构建当前平台：

```bash
go build -buildvcs=false -o nats-monitor .
```

在 Windows 上交叉构建 CentOS/Linux AMD64：

```powershell
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -buildvcs=false -trimpath -ldflags "-s -w" -o nats-monitor .
```

## 安全建议

- 不要将 NATS Monitoring 端口 `8222` 直接暴露到公网。
- 面板允许已登录用户切换到任意 HTTP(S) 地址，只应向可信运维人员开放。
- 公网部署应放在 HTTPS 反向代理后，并设置 `auth.secure_cookie` 为 `true`。
- 使用防火墙或安全组限制 `18222` 的访问来源。
- 不要将真实密码提交到版本库；生产环境可以通过 `MONITOR_PASSWORD` 注入。
- 登录连续失败 5 次后，同一 IP 会被限制 10 分钟。

## 指标说明

- 消息速率：相邻两次 `/varz` 采样的收发消息总增量除以时间间隔。
- 网络吞吐：相邻两次采样的入站与出站字节总增量除以时间间隔。
- 平均扇出：一个具体发布 Subject 平均匹配的投递目标数量；普通订阅各算一个，同名 Queue Group 整组算一个。
- 最大扇出：所有已统计 Subject 中最大的投递目标数量。
- 当前积压连接：当前连接中 `pending_bytes > 0` 的连接，是风险信号，不等同于 NATS 已确认慢消费者。
- 最近确认慢消费者：最近关闭连接中，断开原因包含 `Slow Consumer` 的记录。

## 已知边界

- 趋势历史只保存在内存中，进程重启或切换监控目标后会清空。
- 最近关闭连接最多扫描 NATS 返回的最新 `1024` 条记录。
- 监控目标必须能从运行本程序的服务器访问；浏览器能访问该地址并不代表服务端能访问。
- 部分 NATS 版本或未启用的能力可能不返回所有端点，页面会显示对应模块暂不可用。

## 代码结构

```text
main.go       程序入口与前端资源嵌入
config.go     配置读取、校验和环境变量覆盖
collector.go  NATS Monitoring API 采集、统计和目标切换
models.go     监控响应及页面快照模型
auth.go       登录校验、会话 Cookie 和失败限制
server.go     HTTP 路由、API 和安全响应头
web/          页面、样式、脚本和 favicon
start.sh      CentOS/Linux 启停脚本
start.bat     Windows 启动脚本
```
