# subscriptionlink

一个轻量的订阅链接服务，支持按用户 Token 动态生成 `Clash` / `V2Ray` / `Sing-box` 订阅内容，并提供 Web 管理后台、用户与节点管理、访问统计。

## 功能

- 动态订阅生成
  - `GET /api/subscription/{token}`
  - `GET /api/v2ray/{token}`
  - `GET /api/singbox/{token}`
- 用户 Token 管理（增删改查）
- 节点管理（增删改查）
- Web 管理后台（`/admin.html`）
- 访问统计（总量、按格式、按 Token、最近访问时间）

## 项目结构

```text
.
├── cmd/server/main.go
├── internal/
│   ├── api/
│   ├── generator/
│   ├── model/
│   ├── stats/
│   └── store/
├── data/
│   ├── clash.yaml
│   ├── xray.config
│   ├── users.json
│   └── nodes.json
├── web/
│   ├── index.html
│   └── admin.html
├── Makefile
└── go.mod
```

## 环境要求

- Go `1.25+`
- macOS / Linux / Windows（`make build` 可交叉编译）

## 快速开始

1. 初始化数据文件（首次建议写入空数组）

```bash
echo '[]' > data/users.json
echo '[]' > data/nodes.json
```

2. 启动服务（必须设置管理后台登录密钥）

```bash
ADMIN_TOKEN=your-secret-token go run ./cmd/server
```

启用 Xray 同步（推荐）：

```bash
ADMIN_TOKEN=your-secret-token \
XRAY_CONFIG_PATH=data/xray.config \
XRAY_RELOAD_CMD="" \
go run ./cmd/server
```

可选：自定义监听地址（默认 `127.0.0.1:8081`）

```bash
ADMIN_TOKEN=your-secret-token LISTEN_ADDR=127.0.0.1:18081 go run ./cmd/server
```

3. 打开页面

- 首页：http://127.0.0.1:8081/
- 管理后台：http://127.0.0.1:8081/admin.html

说明：服务当前监听 `127.0.0.1:8081`。

## 本地冒烟测试

```bash
./scripts/smoke_test.sh
```

脚本会启动临时实例（默认 `127.0.0.1:18081`）、执行登录会话与 CSRF 写操作验证，并在结束后恢复 `data/` 文件。

## 构建

默认构建当前平台：

```bash
make build
```

按指定平台跨平台构建（可单个或多个）：

```bash
make build PLATFORM=linux/amd64
make build PLATFORM=linux/amd64,windows/amd64,darwin/arm64
```

产物输出到 `dist/`，例如：

- `dist/subscriptionlink-linux-amd64`
- `dist/subscriptionlink-darwin-arm64`
- `dist/subscriptionlink-windows-amd64.exe`

清理产物：

```bash
make clean
```

## 数据模型

### User

```json
{
  "name": "alice",
  "email": "alice@example.com",
  "token": "user-token",
  "uuid": "user-uuid",
  "expire": 0
}
```

- `expire` 为 Unix 时间戳（秒）
- `expire=0` 表示不过期

### Node

```json
{
  "name": "hk-01",
  "server": "1.2.3.4",
  "port": 1234,
  "protocol": "vless",
  "network": "ws",
  "security": "none",
  "path": "/xhttp",
  "host": ""
}
```

## API 说明

### 1) 订阅接口（无需管理 Token）

- `GET /api/subscription/{token}`
- `GET /api/v2ray/{token}`
- `GET /api/singbox/{token}`

返回规则：

- Token 不存在：`403`
- Token 过期：`403`
- 格式不支持：`404`

### 2) 管理接口（Cookie 会话 + CSRF）

管理鉴权流程：

1. `POST /api/admin/login`，提交 `ADMIN_TOKEN`
2. 服务端设置 `HttpOnly` 会话 Cookie（`admin_session`）
3. 返回 `csrf_token`
4. 后续写操作（`POST/PUT/DELETE`）需带 `X-CSRF-Token`
5. 管理页每次打开默认进入登录页（不会自动跳过登录）

未配置服务端 `ADMIN_TOKEN` 时，登录接口返回 `503`。

#### 登录与会话

- `POST /api/admin/login`
- `GET /api/admin/session`
- `POST /api/admin/logout`

登录请求示例：

```json
{
  "token": "your-admin-token"
}
```

#### 用户管理 `/api/admin/users`

- `GET` 查询所有用户
- `POST` 新增用户
- `PUT` 更新用户（按 `token` 定位）
- `DELETE /api/admin/users?token=...` 删除用户

`POST`/`PUT` 请求示例：

```json
{
  "name": "alice",
  "email": "alice@example.com",
  "token": "optional-token",
  "uuid": "optional-uuid",
  "expire": 0
}
```

说明：`POST` 时若不传 `token` 或 `uuid`，服务会自动生成。
说明：`POST` 时若不传 `email`，服务会按 `name@example.com` 自动补全。

#### 节点管理 `/api/admin/nodes`

- `GET` 查询所有节点
- `POST` 新增节点
- `PUT` 更新节点（按 `name` 定位）
- `DELETE /api/admin/nodes?name=...` 删除节点

请求体示例：

```json
{
  "name": "hk-01",
  "server": "1.2.3.4",
  "port": 1234,
  "protocol": "vless",
  "network": "ws",
  "security": "none",
  "path": "/xhttp",
  "host": ""
}
```

#### 统计接口 `/api/admin/stats`

- `GET` 返回访问统计

返回示例：

```json
{
  "request_count": 12,
  "by_format": {"subscription": 5, "v2ray": 4, "singbox": 3},
  "by_token": {"token-a": 8, "token-b": 4},
  "last_access": 1741350000
}
```

## curl 示例

假设：

```bash
export ADMIN_TOKEN=your-secret-token
export BASE=http://127.0.0.1:8081
```

登录（保存 Cookie 并提取 CSRF）：

```bash
curl -i -c admin.cookie -X POST "$BASE/api/admin/login" \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"$ADMIN_TOKEN\"}"
```

从返回体中拿到 `csrf_token` 后，下面示例记为 `CSRF_TOKEN`。

新增用户：

```bash
curl -X POST "$BASE/api/admin/users" \
  -b admin.cookie \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"alice"}'
```

新增节点：

```bash
curl -X POST "$BASE/api/admin/nodes" \
  -b admin.cookie \
  -H "X-CSRF-Token: $CSRF_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"hk-01","server":"1.2.3.4","port":1234,"protocol":"vless","network":"ws","security":"none","path":"/xhttp"}'
```

获取统计：

```bash
curl "$BASE/api/admin/stats" -b admin.cookie
```

获取订阅：

```bash
curl "$BASE/api/subscription/<user-token>"
```

## Clash 模板与 Xray 同步

- `data/clash.yaml` 为订阅模板，支持占位符：
  - `{{UUID}}`
  - `{{SERVER}}`
  - `{{PORT}}`
  - `{{NODE_NAME}}`
  - `{{NETWORK}}`
  - `{{PATH}}`
  - `{{HOST}}`
  - `{{TLS}}`
- 访问 `/api/subscription/{token}` 时，服务会根据 token 选取一个节点并替换模板占位符。
- 当前 `{{TLS}}` 默认替换为 `true`。

用户新增/更新/删除后支持同步到 Xray（可选，未配置则跳过）：

- `XRAY_CONFIG_PATH`：Xray 配置文件路径
- `XRAY_INBOUND_TAG`：目标 inbound tag（可选）
- `XRAY_RELOAD_CMD`：写入配置后的重载命令

说明：

- 如果 `xray.config` 的 inbound 没有 `tag` 字段，不要设置 `XRAY_INBOUND_TAG`。
- 若设置了 `XRAY_INBOUND_TAG`，必须与 inbound 的 `tag` 完全一致，否则会报：
  `sync xray failed: xray inbound tag not found: <tag>`
- `XRAY_RELOAD_CMD` 未设置时，默认执行：`sudo systemctl reload xray`
- 若你不希望自动重载，可显式设置：`XRAY_RELOAD_CMD=""`

## 当前限制

- 数据存储为本地 JSON 文件，不适合多实例并发写入
- 管理鉴权为单一 `ADMIN_TOKEN`（会话化），未做多角色权限划分
- 订阅协议字段为最小实现，可按客户端要求继续扩展
