# HighgoBase CLI 使用手册

> 基线版本：Supabase CLI **v2.76.14**（提交 `2f6e0c328`）  
> 开发分支：`highgobase-cli-base-2.76.14`  
> 数据库：HighgoBase 9.0.5.1（PostgreSQL 14 兼容）  
> 文档验证日期：2026-11-19

本文说明如何构建 HighgoBase CLI、准备 HighgoBase 资产、启动完整本地栈，以及在
Kubernetes 自托管环境中使用 CLI 连接 HighgoBase 服务。文中所有演示均在本次测试环境
实际执行过；K8s 侧的完整结果矩阵见 [`HIGHGOBASE_K8S_TEST_REPORT.md`](./HIGHGOBASE_K8S_TEST_REPORT.md)。

---

## 1. 结论速览

| 场景 | 结论 |
| --- | --- |
| 本地 Docker 全栈 `start` / `stop` / `status` | 通过，重启后数据保留，健康检查全部通过 |
| 本地 REST / Auth / Storage / Functions / Realtime / GraphQL | 通过 |
| 本地 Studio / Meta / Analytics 健康检查 | 通过；Storage S3 端点已配置（本次未做 S3 SDK 往返） |
| 本地迁移、`db push`、`db diff`、`db dump`、`gen types`、`inspect` | 通过；`db reset` 在隔离库验证 |
| K8s `higobase` 命名空间远程数据库命令 | 通过（使用 `--db-url` + `sslmode=disable`） |
| K8s REST / Auth / Storage / Functions / Realtime / GraphQL | 通过 |
| K8s Studio | 通过（Basic Auth） |
| K8s Meta / Analytics 直连健康检查 | 通过（通过端口转发；经 Kong 路由受限） |
| `db lint` / `db test` | 受镜像缺少 `plpgsql_check` / `pgtap` 扩展限制 |

命令行名称显示为 `highgobase`，同时保留 `supabase` 作为别名。本文示例统一使用
`highgobase`；如果你把可执行文件改名为 `supabase`，请设置环境变量 `HIGHGOBASE=1`。

---

## 2. 版本、分支与目录

```text
D:\higobase\HighgoBase
├── cli          # 本仓库，HighgoBase CLI
└── higobase     # HighgoBase 自托管栈资产和 Docker Compose 文件
```

| 项目 | 值 |
| --- | --- |
| 上游 CLI | Supabase CLI `v2.76.14` |
| 基线提交 | `2f6e0c328` |
| 工作分支 | `highgobase-cli-base-2.76.14` |
| CLI 版本输出 | `2.76.14` |
| 默认 HGDB 镜像 | `hgdb:9.0.5.1-1-5837-31bd189-20260827-pg_partman-pgmq-refactor.linux.x86_64` |
| HGDB 兼容版本 | PostgreSQL 14 兼容（配置必须使用 `major_version = 14`） |

复现分支：

```bash
git fetch --tags
git checkout v2.76.14
git switch -c highgobase-cli-base-2.76.14
```

> 当前工作树中的 HighgoBase 适配改动尚未提交。需要提交时请按团队流程单独整理，
> 不要直接把测试临时文件提交进仓库。

---

## 3. 构建与安装

### 3.1 前置条件

- Go **1.25+**（仅构建时需要）。
- Docker Engine / Docker Desktop **20.10+**（本地全栈需要）。
- 可选：`kubectl`、`psql`、`curl`、`jq`（K8s 远程模式和排障使用）。
- 本地栈默认占用 54321、54322、54323、54324、54327、54329 等端口；如冲突请在
  `supabase/config.toml` 中修改。

### 3.2 Windows PowerShell

```powershell
cd D:\higobase\HighgoBase\cli
$env:CGO_ENABLED = "0"
go build `
  -ldflags "-X github.com/supabase/cli/internal/utils.Version=2.76.14 -X github.com/supabase/cli/internal/highgobase.BuildEnabled=true" `
  -o highgobase.exe .

.\highgobase.exe --version
# 2.76.14
```

### 3.3 Linux / macOS

```bash
cd /path/to/cli
CGO_ENABLED=0 go build \
  -ldflags "-X github.com/supabase/cli/internal/utils.Version=2.76.14 -X github.com/supabase/cli/internal/highgobase.BuildEnabled=true" \
  -o highgobase .

./highgobase --version
```

交叉编译 Linux amd64 二进制（用于部署到 Linux 服务器）：

```powershell
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"
go build -ldflags "-X github.com/supabase/cli/internal/utils.Version=2.76.14 -X github.com/supabase/cli/internal/highgobase.BuildEnabled=true" `
  -o highgobase-linux-amd64 .
```

`.goreleaser.yml` 中已增加 `highgobase` 构建目标，发布构建会生成名为 `highgobase`
的二进制，并注入 `BuildEnabled=true`。

### 3.4 启用 HighgoBase 行为的三种方式

满足任意一种即可：

1. 可执行文件名是 `highgobase` / `highgobase.exe`（推荐）；
2. 设置 `HIGHGOBASE=1`；
3. 构建时加入 `-X github.com/supabase/cli/internal/highgobase.BuildEnabled=true`。

临时回到原版 Supabase 行为：

```bash
HIGHGOBASE=false highgobase --version
```

---

## 4. 准备 HighgoBase 资产

### 4.1 `HIGHGOBASE_HOME`

CLI 需要找到上层 `higobase` 仓库中的 Docker 资产。查找顺序：

1. 环境变量 `HIGHGOBASE_HOME`；
2. 可执行文件同级的 `../higobase`；
3. 可执行文件同级的 `../../higobase`；
4. 当前工作目录的 `../higobase`；
5. 当前工作目录的 `../../higobase`。

该目录必须包含：

```text
higobase/
└── docker/
    └── docker-compose.hgdb.yml
```

Windows 示例：

```powershell
$env:HIGHGOBASE_HOME = "D:\higobase\HighgoBase\higobase"
$env:HIGHGOBASE = "1"
```

Linux 示例：

```bash
export HIGHGOBASE_HOME=/root/higobase
export HIGHGOBASE=1
```

### 4.2 必需文件

`highgobase doctor` 和 `highgobase start` 会检查以下文件：

```text
docker/docker-compose.hgdb.yml
docker/volumes/db/docker-entrypoint.hgdb.sh
docker/volumes/db/pg_hba.conf
docker/volumes/db/pgsodium_getkey.sh
docker/volumes/db/hgdb_init_scripts/migrate.sh
docker/volumes/db/hgdb_init_scripts/init-scripts/00000000000000-initial-schema.sql
docker/volumes/db/hgdb_init_scripts/migrations/97-_supabase.sql
```

### 4.3 License

HGDB 镜像需要许可证。默认挂载目录：

```text
higobase/docker/volumes/db/license/
├── license.dat          # 必需
└── client_info.dat       # 随许可证提供时一并放入
```

也可以显式指定目录：

```bash
export HIGHGOBASE_LICENSE_DIR=/path/to/license
```

`doctor` 只有在目录中存在 `license.dat` 时才显示 `[OK] hgdb license`；仅有
`client_info.dat` 会显示 `WARN`。

### 4.4 镜像

本地栈默认使用以下镜像（可通过环境变量覆盖）：

| 组件 | 默认镜像 | 覆盖变量 |
| --- | --- | --- |
| 数据库 | `hgdb:9.0.5.1-1-5837-31bd189-20260827-pg_partman-pgmq-refactor.linux.x86_64` | `HIGHGOBASE_DB_IMAGE` |
| Kong | `hgbase-kong:latest` | `HIGHGOBASE_KONG_IMAGE` |
| REST | `hgbase-rest:latest` | `HIGHGOBASE_REST_IMAGE` |
| Auth | `hgbase-auth:latest` | `HIGHGOBASE_AUTH_IMAGE` |
| Realtime | `realtime-dev.hgbase-realtime:latest` | `HIGHGOBASE_REALTIME_IMAGE` |
| Storage | `hgbase-storage:latest` | `HIGHGOBASE_STORAGE_IMAGE` |
| imgproxy | `hgbase-imgproxy:latest` | `HIGHGOBASE_IMGPROXY_IMAGE` |
| Meta | `hgbase-meta:latest` | `HIGHGOBASE_META_IMAGE` |
| Studio | `hgbase-studio:latest` | `HIGHGOBASE_STUDIO_IMAGE` |
| Edge Functions | `hgbase-edge-functions:latest` | `HIGHGOBASE_EDGE_FUNCTIONS_IMAGE` |
| Analytics | `hgbase-analytics:latest` | `HIGHGOBASE_ANALYTICS_IMAGE` |
| Vector | `hgbase-vector:latest` | `HIGHGOBASE_VECTOR_IMAGE` |
| Pooler | `hgbase-supavisor:latest` | `HIGHGOBASE_POOLER_IMAGE` |
| Mailpit | `axllent/mailpit:v1.22.3` | `HIGHGOBASE_MAILPIT_IMAGE` |

> `higobase/docker/docker-compose.hgdb.yml` 中的示例镜像标签可能较旧（例如
> `...-20260604...`）。CLI 默认使用上表中的新镜像；如果要完全复用 Compose 文件中的
> 标签，请设置 `HIGHGOBASE_DB_IMAGE`，保证 CLI 与 Compose 使用同一个镜像。

导入镜像示例：

```bash
docker load -i hgdb-9.0.5.1-....tar
docker load -i hgbase-kong-latest.tar
# 其余 hgbase-* 镜像同理
docker images | grep -E 'hgdb|hgbase'
```

`highgobase doctor` 会逐项检查镜像。若某个服务在 `config.toml` 中被禁用，其镜像缺失
只显示 `WARN`，不会导致 `doctor` 失败。

---

## 5. 健康检查：`highgobase doctor`

```bash
cd /path/to/project
export HIGHGOBASE_HOME=/path/to/higobase
export HIGHGOBASE=1
highgobase doctor
```

本次测试的典型输出（节选）：

```text
[OK] higobase home
       /root/higobase
[OK] asset /root/higobase/docker/docker-compose.hgdb.yml
...
[OK] hgdb license
       /root/higobase/docker/volumes/db/license
[OK] database image
       hgdb:9.0.5.1-1-5837-31bd189-20260827-pg_partman-pgmq-refactor.linux.x86_64
[OK] database version
       major_version=14 (HighgoBase 9.x is PostgreSQL 14 compatible)
[OK] docker daemon
[OK] image hgdb:...
[OK] image hgbase-kong:latest
...
[WARN] image hgbase-supavisor:latest
       not loaded locally (service disabled)
[WARN] image axllent/mailpit:v1.22.3
       not loaded locally (service disabled)
```

退出码为 0 表示所有**必需**检查通过；`WARN` 不影响退出码。可用：

```bash
highgobase doctor --json
highgobase doctor --skip-docker   # 只检查资产/配置，不检查 Docker
```

`doctor` 检查的是本机 Docker 前置条件，不检查 K8s 集群。K8s 环境请使用
`kubectl -n higobase get pods` 和本文第 15 节的服务健康检查。

---

## 6. 初始化并启动本地全栈

### 6.1 初始化项目

```bash
mkdir -p /path/to/demo
cd /path/to/demo
export HIGHGOBASE_HOME=/path/to/higobase
export HIGHGOBASE=1
highgobase init
```

生成的 `supabase/config.toml` 会把 `db.major_version` 设为 `14`，并使用 HighgoBase 镜像。
本次完整栈测试使用以下端口（可按需修改）：

```toml
project_id = "hgcli-full"

[api]
enabled = true
port = 55421

[db]
port = 55422
shadow_port = 55420
major_version = 14

[db.pooler]
enabled = false

[inbucket]
enabled = false

[studio]
enabled = true
port = 55423

[analytics]
enabled = true
port = 55427
```

> 不要为了“跟上游保持一致”把 `major_version` 改成 17；HGDB 9.x 对应 PostgreSQL 14。
> 本次测试关闭了 Pooler 和 Inbucket，因此 `doctor` 对这两个镜像给出 `WARN`。

### 6.2 启动

```bash
highgobase start
```

首次启动会初始化 HGDB 数据目录并执行大量初始化脚本，通常需要数分钟。不要使用
`--ignore-health-check` 掩盖问题；本次验证要求不带该参数也能成功。

启动成功后会输出 Studio、API、数据库、Storage S3 等信息。示例：

```text
Started highgobase local development setup.

Studio     http://127.0.0.1:55423
Project URL http://127.0.0.1:55421
REST       http://127.0.0.1:55421/rest/v1
GraphQL    http://127.0.0.1:55421/graphql/v1
Edge Functions http://127.0.0.1:55421/functions/v1
Database   postgresql://postgres:postgres@127.0.0.1:55422/postgres
```

本次测试的 `stop -> start` 重启也成功，数据卷保留；重启后 `demo_items` 数据和
Storage bucket 仍然存在。

### 6.3 查看状态

```bash
highgobase status
highgobase status -o env
highgobase status -o json
highgobase status -o yaml
highgobase status -o toml
```

推荐在脚本中加载环境变量：

```bash
eval "$(highgobase status -o env)"
echo "$API_URL"
echo "$REST_URL"
```

`status -o env` 会提供：

```text
ANON_KEY
API_URL
DB_URL
FUNCTIONS_URL
GRAPHQL_URL
JWT_SECRET
MCP_URL
PUBLISHABLE_KEY
REST_URL
S3_PROTOCOL_ACCESS_KEY_ID
S3_PROTOCOL_ACCESS_KEY_SECRET
S3_PROTOCOL_REGION
SECRET_KEY
SERVICE_ROLE_KEY
STORAGE_S3_URL
STUDIO_URL
```

PowerShell 读取：

```powershell
$status = .\highgobase.exe status -o json | ConvertFrom-Json
$env:API_URL = $status.API_URL
$env:ANON_KEY = $status.ANON_KEY
$env:SERVICE_ROLE_KEY = $status.SERVICE_ROLE_KEY
```

> 本文后续 API 示例使用 JWT 形式的 `ANON_KEY` / `SERVICE_ROLE_KEY`，因为 K8s 当前
> PostgREST 只接受 JWT key。本地 `status` 同时提供 `PUBLISHABLE_KEY` / `SECRET_KEY`，
> 但为了本地和 K8s 共用示例，统一下载 `ANON_KEY` / `SERVICE_ROLE_KEY`。

### 6.4 停止与数据

```bash
highgobase stop
```

停止不会删除 Docker volume，数据会保留。再次 `highgobase start` 会从卷恢复数据库。
彻底删除本地数据前，请确认项目名和 volume：

```bash
docker volume ls --filter label=com.supabase.cli.project=<project-id>
```

### 6.5 服务版本

```bash
highgobase services
```

输出会列出 `hgdb`、`hgbase-auth`、`hgbase-rest`、`hgbase-realtime`、`hgbase-storage`、
`hgbase-edge-functions`、`hgbase-studio`、`hgbase-meta`、`hgbase-analytics` 等镜像版本。

---

## 7. 演示一：迁移、RLS 与 Seed

### 7.1 新建迁移

```bash
cd /path/to/demo
highgobase migration new create_demo_items
# 生成 supabase/migrations/<timestamp>_create_demo_items.sql
```

写入示例 SQL：

```sql
create table if not exists public.demo_items (
  id bigint generated by default as identity primary key,
  name text not null,
  created_at timestamptz not null default now()
);

alter table public.demo_items enable row level security;
grant usage, select on sequence public.demo_items_id_seq to anon, authenticated;
grant select, insert, update, delete on public.demo_items to anon, authenticated;
grant all on public.demo_items to service_role;

create policy "demo read" on public.demo_items
  for select to anon, authenticated using (true);

create policy "demo insert" on public.demo_items
  for insert to anon, authenticated with check (name <> '');
```

### 7.2 查看并推送迁移

```bash
highgobase migration list
highgobase db push --local
```

`migration list` 会同时显示 Local 和 Remote 两列；`db push --local` 把未应用的本地迁移
应用到本地数据库。`--include-all` 用于把远端历史表中没有记录的本地迁移一并纳入：

```bash
highgobase db push --local --include-all
```

本次测试在 `db push` 后通过 REST 成功插入、查询了 `demo_items`。如果刚建表就请求
PostgREST，可能短暂出现 `PGRST205`，等待 1-2 秒或手动刷新：

```sql
notify pgrst, 'reload schema';
```

CLI 在应用迁移后已自动执行该通知，但 PostgREST 处理通知是异步的。

### 7.3 生成迁移差异

```bash
highgobase db diff --local
highgobase db diff --local --schema public
highgobase db diff --local -f supabase/migrations/next.sql
```

在 HGDB 上，`db diff` 可能报告系统扩展差异（例如 `pg_net`、`pgcrypto`、
`uuid-ossp`）。这些是 HGDB 初始化脚本安装的扩展，不一定是业务变更。建议：

- 使用 `--schema public` 聚焦业务 schema；
- 或把系统扩展相关差异排除在迁移评审之外；
- 不要为了“清零 diff”而删除 HGDB 运行时扩展。

### 7.4 导出数据库

```bash
highgobase db dump --local --dry-run
highgobase db dump --local --schema public --file public.sql
highgobase db dump --local --data-only --file data.sql
highgobase db dump --local --role-only --file roles.sql
highgobase db dump --local --schema public --use-copy --file public-copy.sql
```

`--dry-run` 只打印将要执行的 `pg_dump` 脚本，不会生成文件。`--exclude schema.table` 可排除
指定表（主要配合 `--data-only`）。

### 7.5 重置本地数据库

```bash
highgobase db reset --local
highgobase db reset --local --version 20261119000000
```

`db reset` 会删除并重建本地数据库，再应用全部迁移和 seed。**不要对 K8s 业务库执行
`db reset`。** 本次 K8s 测试只在隔离库 `hgcli_reset` 上验证过。

### 7.6 Seed

`supabase/seed.sql` 会在 `db reset` 时执行。也可以显式推送 seed：

```bash
highgobase db push --local --include-seed
```

### 7.7 Lint 与 pgTAP 测试

```bash
highgobase db lint --local
highgobase db test --local
```

本次 HGDB 镜像缺少：

- `plpgsql_check`（`db lint` 需要）；
- `pgtap`（`db test` 需要）。

因此这两个命令会报扩展控制文件不存在。这是镜像扩展限制，不是 CLI 崩溃；在安装了
对应扩展的 PostgreSQL/HGDB 镜像上可正常使用。命令与测试文件生成仍可用：

```bash
highgobase test new my_test
```

---

## 8. 演示二：REST API 与 RLS

先加载环境变量：

```bash
cd /path/to/demo
eval "$(highgobase status -o env)"
```

### 8.1 查询

```bash
curl -sS "$REST_URL/demo_items?select=*" \
  -H "apikey: $ANON_KEY" \
  -H "Authorization: Bearer $ANON_KEY"
```

### 8.2 插入

```bash
curl -sS -X POST "$REST_URL/demo_items" \
  -H "apikey: $ANON_KEY" \
  -H "Authorization: Bearer $ANON_KEY" \
  -H "Content-Type: application/json" \
  -H "Prefer: return=representation" \
  -d '{"name":"local-demo"}'
```

### 8.3 修改

```bash
curl -sS -X PATCH "$REST_URL/demo_items?name=eq.local-demo" \
  -H "apikey: $ANON_KEY" \
  -H "Authorization: Bearer $ANON_KEY" \
  -H "Content-Type: application/json" \
  -H "Prefer: return=representation" \
  -d '{"name":"local-demo-updated"}'
```

### 8.4 删除

```bash
curl -sS -X DELETE "$REST_URL/demo_items?name=eq.local-demo-updated" \
  -H "apikey: $ANON_KEY" \
  -H "Authorization: Bearer $ANON_KEY"
```

### 8.5 RLS 验证

上面迁移已启用 RLS 并创建了 anon 策略，因此使用 `ANON_KEY` 也能插入/查询。若删除
`demo insert` 策略，再用 anon 插入，PostgREST 会按 RLS 拒绝。可以用两个 key 对比：

```bash
# anon：受 RLS 约束
curl -sS "$REST_URL/demo_items?select=id,name" \
  -H "apikey: $ANON_KEY" -H "Authorization: Bearer $ANON_KEY"

# service_role：绕过 RLS，仅用于管理/测试
curl -sS "$REST_URL/demo_items?select=id,name" \
  -H "apikey: $SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SERVICE_ROLE_KEY"
```

本次测试验证了：带策略的 anon 访问成功；REST 的 SELECT/INSERT/PATCH/DELETE 均返回
预期状态码。

---

## 9. 演示三：Auth 注册、登录与用户信息

```bash
eval "$(highgobase status -o env)"
EMAIL="demo-$(date +%s)@example.com"
PASSWORD='DemoPass!2026'

# 注册
curl -sS -X POST "$API_URL/auth/v1/signup" \
  -H "apikey: $ANON_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"

# 密码登录
TOKEN=$(curl -sS -X POST "$API_URL/auth/v1/token?grant_type=password" \
  -H "apikey: $ANON_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}" | jq -r .access_token)

# 当前用户
curl -sS "$API_URL/auth/v1/user" \
  -H "apikey: $ANON_KEY" \
  -H "Authorization: Bearer $TOKEN"
```

本次本地测试中：

- 邮箱注册返回成功；
- 密码登录返回 `access_token`；
- `/auth/v1/user` 返回对应邮箱和用户 ID。

如果配置开启了邮箱确认，注册后需要先确认；本次本地和 K8s 测试环境均启用了
`MAILER_AUTOCONFIRM=true`。

管理员创建用户（service_role）：

```bash
curl -sS -X POST "$API_URL/auth/v1/admin/users" \
  -H "apikey: $SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SERVICE_ROLE_KEY" \
  -H "Content-Type: application/json" \
  -d '{"email":"admin-created@example.com","password":"AdminPass!2026","email_confirm":true}'
```

---

## 10. 演示四：Storage

### 10.1 REST API 操作

```bash
eval "$(highgobase status -o env)"
BUCKET="demo-bucket"
FILE="/tmp/demo.txt"
printf 'hello highgobase storage\n' > "$FILE"

# 创建公开 bucket
curl -sS -X POST "$API_URL/storage/v1/bucket" \
  -H "apikey: $SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SERVICE_ROLE_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"$BUCKET\",\"public\":true}"

# 上传
curl -sS -X POST "$API_URL/storage/v1/object/$BUCKET/demo.txt" \
  -H "apikey: $SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SERVICE_ROLE_KEY" \
  -H "Content-Type: text/plain" \
  -H "x-upsert: true" \
  --data-binary @"$FILE"

# 列表
curl -sS -X POST "$API_URL/storage/v1/object/list/$BUCKET" \
  -H "apikey: $SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SERVICE_ROLE_KEY" \
  -H "Content-Type: application/json" \
  -d '{"prefix":"","limit":100}'

# 公开下载（公开 bucket）
curl -sS "$API_URL/storage/v1/object/public/$BUCKET/demo.txt" \
  -H "apikey: $ANON_KEY"

# 生成 10 分钟签名 URL
SIGNED=$(curl -sS -X POST "$API_URL/storage/v1/object/sign/$BUCKET/demo.txt" \
  -H "apikey: $SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SERVICE_ROLE_KEY" \
  -H "Content-Type: application/json" \
  -d '{"expiresIn":600}' | jq -r .signedURL)

curl -sS "$API_URL/storage/v1$SIGNED" -H "apikey: $ANON_KEY"

# 删除对象与 bucket
curl -sS -X DELETE "$API_URL/storage/v1/object/$BUCKET/demo.txt" \
  -H "apikey: $SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SERVICE_ROLE_KEY"
curl -sS -X DELETE "$API_URL/storage/v1/bucket/$BUCKET" \
  -H "apikey: $SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SERVICE_ROLE_KEY"
```

本次测试验证了：创建 bucket、上传、列表、公开下载、签名 URL 下载、删除对象和删除
bucket，均返回成功。

### 10.2 Storage CLI（本地）

`storage` 子命令需要 `--experimental`；本地模式使用 `--local`。`rm` 会提示确认，
非交互脚本加 `--yes`。

```bash
# 上传
highgobase --experimental storage cp --local ./demo.txt ss:///demo-bucket/demo.txt

# 列出对象（注意 bucket 后要有 /）
highgobase --experimental storage ls --local ss:///demo-bucket/

# 下载
highgobase --experimental storage cp --local ss:///demo-bucket/demo.txt ./download.txt

# 移动
highgobase --experimental storage mv --local \
  ss:///demo-bucket/demo.txt ss:///demo-bucket/archive/demo.txt

# 删除（--yes 跳过确认）
highgobase --experimental --yes storage rm --local \
  ss:///demo-bucket/archive/demo.txt
```

注意 URL 是 `ss:///bucket/path`（三个斜杠）。本次构建已修复 `storage --local` 被误判为
Management API、从而强制要求云端 token 的问题；如果你使用原始上游 2.76.14 二进制，
`storage --local` 可能仍要求 access token。K8s 自托管环境请直接使用 Storage REST API。

---

## 11. 演示五：Edge Functions

### 11.1 创建函数

```bash
cd /path/to/demo
highgobase functions new hello
```

编辑 `supabase/functions/hello/index.ts`：

```ts
Deno.serve(() => {
  return Response.json({
    ok: true,
    runtime: "highgobase-edge-functions",
    message: "hello from local edge runtime",
  });
});
```

### 11.2 本地运行

```bash
highgobase functions serve --no-verify-jwt
```

另开终端调用：

```bash
eval "$(highgobase status -o env)"
curl -sS "$FUNCTIONS_URL/hello" \
  -H "apikey: $ANON_KEY" \
  -H "Authorization: Bearer $ANON_KEY"
```

本次测试中，`functions serve --no-verify-jwt` 启动成功，`/functions/v1/hello` 返回
`200` 和预期 JSON。即使关闭函数侧 JWT 校验，Kong 仍要求合法的 apikey/Authorization
头；不带 key 会返回 401。

### 11.3 函数管理命令

```bash
highgobase functions list
highgobase functions download hello
highgobase functions deploy hello
highgobase functions delete hello
```

`list/download/deploy/delete` 使用 Supabase Management API，需要 `login`/`link` 或
`SUPABASE_ACCESS_TOKEN`。K8s 自托管 Functions 由 Helm/集群部署管理，当前 CLI 不会
直接向 `higobase` 命名空间部署函数；请使用现有函数或修改 K8s 部署。

---

## 12. 演示六：GraphQL

```bash
eval "$(highgobase status -o env)"

curl -sS -X POST "$GRAPHQL_URL" \
  -H "apikey: $ANON_KEY" \
  -H "Authorization: Bearer $ANON_KEY" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ __typename }"}'
```

预期返回：

```json
{"data":{"__typename":"Query"}}
```

本次本地和 K8s 测试均返回 200。要查询业务表，请按 pg_graphql 的 schema 命名规则
编写查询；不同表的暴露配置可能不同，先用简单 `__typename` 验证连通性。

---

## 13. 演示七：Realtime

### 13.1 WebSocket 握手

```bash
eval "$(highgobase status -o env)"
curl -i -N --http1.1 --max-time 3 \
  "$API_URL/realtime/v1/websocket?apikey=$ANON_KEY&vsn=1.0.0" \
  -H 'Connection: Upgrade' \
  -H 'Upgrade: websocket' \
  -H 'Sec-WebSocket-Version: 13' \
  -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ=='
```

预期看到：

```text
HTTP/1.1 101 Switching Protocols
```

本次本地测试还完成了 Phoenix `phx_join`，返回 `{"status":"ok"}`；K8s 测试完成了
WebSocket 101 握手。

### 13.2 JavaScript 订阅示例

先把表加入 Realtime publication：

```sql
alter publication supabase_realtime add table public.demo_items;
```

```js
import { createClient } from "https://esm.sh/@supabase/supabase-js@2";

const supabase = createClient(API_URL, ANON_KEY);

const channel = supabase
  .channel("demo-items")
  .on(
    "postgres_changes",
    { event: "*", schema: "public", table: "demo_items" },
    (payload) => console.log("change", payload),
  )
  .subscribe((status) => console.log("status", status));
```

---

## 14. 演示八：Studio、Meta、Analytics 与 S3 端点

| 服务 | 本次测试地址 | 说明 |
| --- | --- | --- |
| Studio | `http://127.0.0.1:55423` | 本次环境 Basic Auth `admin` / `mypassword` |
| API | `http://127.0.0.1:55421` | Kong 入口 |
| Analytics | `http://127.0.0.1:55427` | 本地配置 `[analytics] port` |
| Storage S3 | `http://127.0.0.1:55421/storage/v1/s3` | 使用 `S3_PROTOCOL_*` 凭据 |
| MCP | `http://127.0.0.1:55421/mcp` | `status` 输出 `MCP_URL` |

Studio：

```bash
curl -sS -o /dev/null -w '%{http_code}\n' \
  -u admin:mypassword -L "$STUDIO_URL"
# 200
```

Analytics 健康检查：

```bash
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:55427/health
# 200
```

Meta 本地未直接映射到宿主机时，可通过容器内健康检查或 Docker 网络访问：

```bash
docker ps --filter name=supabase_pg_meta_ --format '{{.Names}}'
docker exec <meta-container> node --eval="fetch('http://127.0.0.1:8080/health').then(r=>console.log(r.status))"
```

S3 端点与凭据来自 `status -o env`（本次验证了端点/凭据输出和 Storage REST，未执行 S3 SDK 往返）：

```bash
eval "$(highgobase status -o env)"
echo "$STORAGE_S3_URL"
echo "$S3_PROTOCOL_ACCESS_KEY_ID"
echo "$S3_PROTOCOL_REGION"
```

> Studio 的 Basic Auth 凭据取决于部署配置；本次测试环境为 `admin` / `mypassword`。
> 如果你的环境不同，请以 `higobase` 的 Kong/Studio 配置为准。
---

## 15. CLI 命令地图

命令行按用途分为三组：本地开发、Management API（Supabase Cloud）和附加命令。
自托管 K8s 环境主要使用「本地开发」组中的数据库/类型/巡检命令，以及 HTTP API。

### 15.1 本地开发

| 命令 | 作用 | 本次验证 |
| --- | --- | --- |
| `init` | 生成 `supabase/config.toml` | 通过 |
| `start` | 启动本地 Docker 全栈 | 通过（含重启） |
| `stop` | 停止本地栈 | 通过 |
| `status` | 查看容器、URL、key | 通过（pretty/env/json） |
| `doctor` | 检查资产、License、Docker、镜像 | 通过 |
| `services` | 查看各服务镜像版本 | 通过 |
| `db start` | 只启动本地数据库 | 通过（由 start 间接验证） |
| `db push` | 应用迁移（`--local` / `--db-url`） | 通过 |
| `db pull` | 从远程拉取 schema | 通过（干净迁移项目） |
| `db diff` | 比较迁移与数据库 schema | 通过（干净迁移项目） |
| `db dump` | 导出 schema/data/roles | 通过 |
| `db reset` | 重置数据库 | 通过（隔离库） |
| `db lint` | 用 `plpgsql_check` 检查 | 受限（镜像缺扩展） |
| `db test` | 用 pgTAP 运行测试 | 受限（镜像缺扩展） |
| `migration new/list/up/down/repair/fetch/squash` | 迁移管理 | 通过 |
| `gen types` | 生成 TypeScript/Go/Swift/Python 类型 | 通过（TS/Go） |
| `inspect db ...` | 13 个数据库巡检视图 | 通过 |
| `inspect report` | 输出 CSV 巡检报告 | 通过 |
| `functions new/serve` | 本地创建/运行 Edge Function | 通过 |
| `seed buckets` | 按配置创建 Storage bucket | 通过（无 bucket 配置时为空操作） |
| `test new` | 生成 pgTAP 测试文件 | 通过（`db test` 执行受扩展限制） |
| `storage ls/cp/mv/rm --local` | 本地 Storage 对象操作 | 通过（需 `--experimental`） |

### 15.2 Management API（Supabase Cloud）

以下命令需要 Supabase 账户/`SUPABASE_ACCESS_TOKEN`，并针对 Supabase Cloud 项目，
不适用于当前自托管 `higobase` 命名空间：

```text
login/logout, link/unlink, projects, orgs, secrets, config, backups, branches,
domains, encryption, network-bans, network-restrictions, postgres-config,
snippets, ssl-enforcement, sso, vanity-subdomains,
functions list/download/deploy/delete,
storage 云端模式（不带 --local）
```

在 K8s 自托管环境中：

- 数据库操作使用 `--db-url`；
- REST/Auth/Storage/Functions/Realtime 使用 Kong NodePort + JWT key；
- 函数和基础设施变更通过 Helm/Kubernetes 部署，而不是 CLI 的 `functions deploy`。

### 15.3 常用示例

```bash
# 迁移
highgobase migration new add_notes
highgobase migration list
highgobase migration up
highgobase migration down --last 1
highgobase migration repair --status reverted 20261119000000
highgobase migration fetch --db-url "$DB_URL"
highgobase migration squash --version 20261119000000

# 类型生成
highgobase gen types --local --lang typescript > database.types.ts
highgobase gen types --local --lang go > database.go
highgobase gen types --db-url "$DB_URL" --lang typescript > remote.types.ts

# 巡检
highgobase inspect db db-stats --local
highgobase inspect db table-stats --local
highgobase inspect db index-stats --local
highgobase inspect db role-stats --local
highgobase inspect db vacuum-stats --local
highgobase inspect db traffic-profile --local
highgobase inspect db blocking --local
highgobase inspect db locks --local
highgobase inspect db long-running-queries --local
highgobase inspect db replication-slots --local
highgobase inspect db calls --local
highgobase inspect db outliers --local
highgobase inspect db bloat --local
highgobase inspect report --local --output-dir ./inspect-report
```

---

## 16. K8s 远程模式

### 16.1 环境信息

本次测试环境：

| 项目 | 值 |
| --- | --- |
| 服务器 | `192.168.21.145`（SSH 用户 `root`） |
| Kubernetes Namespace | `higobase` |
| Helm Release | `higobase` |
| Chart | `higobase-0.7.2` |
| Kong NodePort | `http://192.168.21.145:32626` |
| DB Service | `higobase-db:5866` |
| DB 本地转发 | `127.0.0.1:15432 -> 5866` |
| Studio | Basic Auth（本次 `admin` / `mypassword`） |
| Auth | GoTrue v2.186.0，`MAILER_AUTOCONFIRM=true` |
| PostgREST | 14.8，schema：`public,storage,graphql_public` |

连接服务器：

```bash
ssh root@192.168.21.145
# 按提示输入管理员提供的密码，或使用已配置的 SSH key
```

检查 Pod：

```bash
kubectl -n higobase get pods
kubectl -n higobase get svc
```

### 16.2 数据库端口转发

在独立终端保持端口转发：

```bash
kubectl -n higobase port-forward --address 127.0.0.1 \
  svc/higobase-db 15432:5866
```

获取数据库密码：

```bash
export PGPASSWORD="$(
  kubectl -n higobase get secret higobase-db \
    -o jsonpath='{.data.password}' | base64 -d
)"
```

数据库 URL（K8s HGDB 未启用 TLS，必须带 `sslmode=disable`）：

```bash
export DB_URL="postgresql://postgres:${PGPASSWORD}@127.0.0.1:15432/postgres?sslmode=disable"
```

> 不要把 `postgres` 业务库当作实验库。迁移/重置测试请使用可丢弃数据库；本次隔离库为
> `hgcli_verify` 和 `hgcli_reset`。禁止对 K8s 主库执行 `db reset`，也不要删除
> `higobase` namespace。

验证连接：

```bash
psql "$DB_URL" -c 'select version();'
# 或
highgobase db dump --db-url "$DB_URL" --schema public --dry-run
```

### 16.3 获取 API Key

```bash
export ANON_KEY="$(
  kubectl -n higobase get secret higobase-jwt \
    -o jsonpath='{.data.anonKey}' | base64 -d
)"
export SERVICE_ROLE_KEY="$(
  kubectl -n higobase get secret higobase-jwt \
    -o jsonpath='{.data.serviceKey}' | base64 -d
)"
export BASE=http://192.168.21.145:32626
```

> **重要：K8s 当前 PostgREST 只接受上述 JWT key。** `higobase-apikey` Secret 中的
> `publishableKey` / `secretKey`（`sb_publishable_*` / `sb_secret_*`）在该部署中会被
> PostgREST 以 `PGRST301` 拒绝。请使用 `higobase-jwt` 的 `anonKey` / `serviceKey`。

### 16.4 CLI 数据库命令

```bash
# 迁移列表
highgobase migration list --db-url "$DB_URL"

# 预览/执行迁移
highgobase db push --db-url "$DB_URL" --dry-run
highgobase db push --db-url "$DB_URL" --yes

# 导出
highgobase db dump --db-url "$DB_URL" --schema public --dry-run
highgobase db dump --db-url "$DB_URL" --schema public --file public.sql
highgobase db dump --db-url "$DB_URL" --data-only --file data.sql
highgobase db dump --db-url "$DB_URL" --role-only --file roles.sql

# 类型
highgobase gen types --db-url "$DB_URL" --lang typescript > remote.types.ts
highgobase gen types --db-url "$DB_URL" --lang go > remote.go

# 巡检
highgobase inspect db db-stats --db-url "$DB_URL"
highgobase inspect db role-stats --db-url "$DB_URL"
highgobase inspect report --db-url "$DB_URL" --output-dir ./inspect-report

# 只对可丢弃库执行
highgobase db reset --db-url "$ISOLATED_DB_URL"
```

`db push` 的迁移历史规则：

- 远端 `supabase_migrations.schema_migrations` 中的版本必须能在本地迁移目录找到，
  否则 CLI 会拒绝推送并提示 `migration repair`；
- 想让远端历史表忽略某些本地迁移，可使用 `--include-all`，但它不会绕过“远端有、
  本地没有”的检查；
- 先用 `highgobase migration list --db-url "$DB_URL"` 对比 Local/Remote，再决定是否
  `migration repair` 或 `db pull`。

### 16.5 K8s REST

```bash
curl -sS "$BASE/rest/v1/" \
  -H "apikey: $ANON_KEY" \
  -H "Authorization: Bearer $ANON_KEY"
# 200，返回 OpenAPI/Swagger 信息
```

临时测试表（本次测试使用临时表并在结束后删除）：

```sql
create table public.demo_items (
  id bigint generated by default as identity primary key,
  name text not null,
  created_at timestamptz not null default now()
);
alter table public.demo_items enable row level security;
grant usage, select on sequence public.demo_items_id_seq to anon, authenticated, service_role;
grant select, insert, update, delete on public.demo_items to anon, authenticated, service_role;
create policy demo_select on public.demo_items for select to anon using (true);
create policy demo_insert on public.demo_items for insert to anon with check (true);
create policy demo_update on public.demo_items for update to anon using (true) with check (true);
create policy demo_delete on public.demo_items for delete to anon using (true);
notify pgrst, 'reload schema';
```

执行 SQL（本地 psql）：

```bash
psql "$DB_URL" -c "create table ...;"
```

或直接通过 Pod 执行：

```bash
kubectl -n higobase exec higobase-db-0 -c db -- \
  psql "host=127.0.0.1 port=5866 user=postgres dbname=postgres sslmode=disable" \
  -c "select 1;"
```

> Pod 内 psql 用 `sslmode=require` 会报“server does not support SSL”；用
> `sslmode=disable`。通过 port-forward 从宿主机连接同理。

REST CRUD：

```bash
# 插入
curl -sS -X POST "$BASE/rest/v1/demo_items" \
  -H "apikey: $ANON_KEY" \
  -H "Authorization: Bearer $ANON_KEY" \
  -H "Content-Type: application/json" \
  -H "Prefer: return=representation" \
  -d '{"name":"k8s-anon"}'

# 查询
curl -sS "$BASE/rest/v1/demo_items?select=id,name&order=id.desc&limit=3" \
  -H "apikey: $ANON_KEY" \
  -H "Authorization: Bearer $ANON_KEY"

# 修改
curl -sS -X PATCH "$BASE/rest/v1/demo_items?id=eq.1" \
  -H "apikey: $ANON_KEY" \
  -H "Authorization: Bearer $ANON_KEY" \
  -H "Content-Type: application/json" \
  -H "Prefer: return=representation" \
  -d '{"name":"k8s-anon-updated"}'

# 删除
curl -sS -X DELETE "$BASE/rest/v1/demo_items?id=eq.1" \
  -H "apikey: $ANON_KEY" \
  -H "Authorization: Bearer $ANON_KEY"
```

本次测试结果：INSERT `201`、SELECT `200`、PATCH `200`、DELETE `204`，RLS 策略生效。

### 16.6 K8s Auth

```bash
EMAIL="k8s-demo-$(date +%s)@example.com"
PASSWORD='K8sDemo!2026'

curl -sS -X POST "$BASE/auth/v1/signup" \
  -H "apikey: $ANON_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}"

TOKEN=$(curl -sS -X POST "$BASE/auth/v1/token?grant_type=password" \
  -H "apikey: $ANON_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}" | jq -r .access_token)

curl -sS "$BASE/auth/v1/user" \
  -H "apikey: $ANON_KEY" \
  -H "Authorization: Bearer $TOKEN"
```

本次测试：注册 200、登录 200、`/auth/v1/user` 200 并返回邮箱/用户 ID；
`/auth/v1/health` 200（GoTrue v2.186.0）。

### 16.7 K8s Storage

```bash
BUCKET="k8s-demo-$(date +%s)"
printf 'hello k8s storage\n' > /tmp/k8s.txt

# 创建公开 bucket
curl -sS -X POST "$BASE/storage/v1/bucket" \
  -H "apikey: $SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SERVICE_ROLE_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"$BUCKET\",\"public\":true}"

# 上传
curl -sS -X POST "$BASE/storage/v1/object/$BUCKET/demo.txt" \
  -H "apikey: $SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SERVICE_ROLE_KEY" \
  -H "Content-Type: text/plain" \
  --data-binary @/tmp/k8s.txt

# 列表
curl -sS -X POST "$BASE/storage/v1/object/list/$BUCKET" \
  -H "apikey: $SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SERVICE_ROLE_KEY" \
  -H "Content-Type: application/json" \
  -d '{"prefix":"","limit":100}'

# 公开下载
curl -sS "$BASE/storage/v1/object/public/$BUCKET/demo.txt" \
  -H "apikey: $ANON_KEY"

# 签名 URL
SIGNED=$(curl -sS -X POST "$BASE/storage/v1/object/sign/$BUCKET/demo.txt" \
  -H "apikey: $SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SERVICE_ROLE_KEY" \
  -H "Content-Type: application/json" \
  -d '{"expiresIn":600}' | jq -r .signedURL)
curl -sS "$BASE/storage/v1$SIGNED" -H "apikey: $ANON_KEY"

# 清理
curl -sS -X DELETE "$BASE/storage/v1/object/$BUCKET/demo.txt" \
  -H "apikey: $SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SERVICE_ROLE_KEY"
curl -sS -X DELETE "$BASE/storage/v1/bucket/$BUCKET" \
  -H "apikey: $SERVICE_ROLE_KEY" \
  -H "Authorization: Bearer $SERVICE_ROLE_KEY"
```

本次测试：创建 bucket、上传、列表、公开下载、签名 URL 下载、删除对象、删除 bucket
全部成功，`/storage/v1/status` 返回 200。

### 16.8 K8s Functions

当前 `higobase` 部署中的函数包括 `legal-probe` 等；先查看 Pod 内函数目录：

```bash
kubectl -n higobase exec deploy/higobase-functions -- \
  ls /home/deno/functions
```

调用 `legal-probe`（部署相关示例；没有该函数时返回 404）：

```bash
curl -sS "$BASE/functions/v1/legal-probe" \
  -H "apikey: $ANON_KEY" \
  -H "Authorization: Bearer $ANON_KEY"
```

本次测试返回 200 和函数 JSON。Functions 的 Kong 路由仍要求合法 JWT；不带 key 会 401。
`functions deploy` 面向 Supabase Cloud，不适用于自托管 K8s；请通过 Helm/集群更新函数。

### 16.9 K8s GraphQL 与 Realtime

GraphQL：

```bash
curl -sS -X POST "$BASE/graphql/v1" \
  -H "apikey: $ANON_KEY" \
  -H "Authorization: Bearer $ANON_KEY" \
  -H "Content-Type: application/json" \
  -d '{"query":"{ __typename }"}'
# {"data":{"__typename":"Query"}}
```

Realtime 握手：

```bash
curl -i -N --http1.1 --max-time 3 \
  "$BASE/realtime/v1/websocket?apikey=$ANON_KEY&vsn=1.0.0" \
  -H 'Connection: Upgrade' \
  -H 'Upgrade: websocket' \
  -H 'Sec-WebSocket-Version: 13' \
  -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ=='
# HTTP/1.1 101 Switching Protocols
```

### 16.10 K8s Studio、Meta、Analytics

Studio：

```bash
curl -sS -o /dev/null -w '%{http_code}\n' \
  -u admin:mypassword -L "$BASE/"
# 200
```

Meta / Analytics 经 Kong 的路由在本次部署中受限（Meta `/pg/health` 返回 403
`You cannot consume this service`，Analytics `/analytics/health` 返回 401）。直接端口
转发验证健康：

```bash
# 终端 1
kubectl -n higobase port-forward --address 127.0.0.1 \
  svc/higobase-meta 18080:8080

# 终端 2
curl -sS http://127.0.0.1:18080/health
# {"date":"..."}

# 终端 3
kubectl -n higobase port-forward --address 127.0.0.1 \
  svc/higobase-analytics 14000:4000

# 终端 4
curl -sS http://127.0.0.1:14000/health
# {"status":"ok",...}
```

本次测试两个直连健康检查均返回 200。

### 16.11 K8s 端到端演示清单

1. `kubectl -n higobase get pods`，确认 DB/REST/Auth/Storage/Functions/Kong 等 Running。
2. 另开终端 `kubectl -n higobase port-forward ... svc/higobase-db 15432:5866`。
3. 从 Secret 取出 DB 密码、JWT anon/service key。
4. 在**可丢弃数据库**上执行 `highgobase db push --db-url "$DB_URL" --yes`。
5. 用 `highgobase gen types --db-url "$DB_URL" --lang typescript` 验证 schema 可读。
6. 通过 `$BASE/rest/v1/...` 做 CRUD，确认 RLS。
7. 通过 `$BASE/auth/v1/...` 注册/登录。
8. 通过 `$BASE/storage/v1/...` 上传/下载/签名 URL。
9. 调用 `$BASE/functions/v1/legal-probe`（或你自己的函数）。
10. 验证 `$BASE/graphql/v1` 和 Realtime WebSocket 101。
11. 用 Studio Basic Auth 打开 `$BASE/`。
12. 清理临时表、bucket、测试用户；不要动业务库。

---

## 17. 常见问题与排障

### 17.1 Kong 启动失败：`no LMDB environment defined`

已修复。HighgoBase 使用自定义 `custom_nginx.template`，Kong 3.9 的 DB-less 模式需要
LMDB 初始化指令和合适的 worker 配置。修复内容包括：

```nginx
user kong kong;
worker_rlimit_nofile 1024;
include 'nginx-inject.conf';

events {
    multi_accept on;
    worker_connections 1024;
}
```

同时把声明式配置路径改为 `/usr/local/kong/kong.yml`，并把 `_format_version` 升到
`2.1`。如果仍看到该错误，确认使用的是本次构建的二进制，而不是旧版 `supabase`。

### 17.2 数据库起不来：License

- 确认 `HIGHGOBASE_HOME/docker/volumes/db/license/license.dat` 存在；
- 或设置 `HIGHGOBASE_LICENSE_DIR`；
- 运行 `highgobase doctor`，应显示 `[OK] hgdb license`。

只有 `client_info.dat` 不会被视为有效 License。

### 17.3 镜像缺失

```bash
highgobase doctor
docker images | grep -E 'hgdb|hgbase'
```

必需镜像缺失会显示 `[FAIL]`；禁用服务的镜像缺失只显示 `[WARN]`。导入镜像后重试。
CLI 在本地模式下不会用 Compose 自动拉取整套镜像；它会在启动各容器时按需检查/拉取，
失败时提示把镜像加载到本机 Docker。

### 17.4 `PGRST205`：表存在但 REST 找不到

PostgREST 的 schema cache 有延迟。迁移后等待 1-2 秒，或执行：

```sql
notify pgrst, 'reload schema';
```

CLI 已在迁移后自动发送该通知，但处理是异步的。

### 17.5 `PGRST301`：JWT/key 被拒绝

K8s 当前只接受 JWT key。使用：

```bash
kubectl -n higobase get secret higobase-jwt \
  -o jsonpath='{.data.anonKey}' | base64 -d
kubectl -n higobase get secret higobase-jwt \
  -o jsonpath='{.data.serviceKey}' | base64 -d
```

不要用 `higobase-apikey` 的 `sb_publishable_*` / `sb_secret_*` 替换。

### 17.6 `db lint` / `db test` 扩展缺失

```text
could not open extension control file ".../plpgsql_check.control"
could not open extension control file ".../pgtap.control"
```

当前 HGDB 镜像未安装这两个扩展。需要在镜像中安装扩展，或跳过这两个命令。

### 17.7 `db diff` shadow 数据库报 owner 错误

如果迁移文件是全库 dump，包含：

```sql
ALTER FUNCTION ... OWNER TO "supabase_admin";
```

在 shadow 数据库中当前用户可能不是该角色，报 `must be owner of function`。处理方式：

- 使用只包含业务对象的干净迁移；
- 或在有足够权限的环境运行；
- 不要把系统函数/owner 语句放进业务迁移。

### 17.8 `db pull` 报迁移历史不匹配

```text
The remote database's migration history does not match local files
```

按提示 `migration repair --status reverted <version>`，或先同步/拉取远端迁移。不要
在不了解远端历史的情况下直接 `db reset`。

### 17.9 SSL / `sslmode`

K8s HGDB 未启用 TLS：

```text
server does not support SSL, but SSL was required
```

连接串必须带：

```text
?sslmode=disable
```

### 17.10 Storage CLI

- 必须加 `--experimental`；
- 本地模式必须加 `--local`；
- `rm` 交互确认，脚本加 `--yes`；
- URL 使用 `ss:///bucket/path`（三个斜杠）；
- 列出对象时 bucket 后加 `/`，否则只列出 bucket 名。

### 17.11 Meta/Analytics 经 Kong 返回 403/401

这是 Kong 路由/认证策略，不是服务不健康。使用端口转发直连 8080/4000 检查，或通过
Analytics 的对外端口访问。

### 17.12 Windows 路径与 CRLF

CLI 已在执行 SQL 脚本和 `pg_dump` 脚本时把 CRLF 转为 LF，并在 Windows 上处理路径。
如果仍然遇到 `\r` 相关错误：

- 使用 Git Bash 或 WSL 执行 shell 示例；
- 用 `dos2unix` 转换自定义 SQL；
- 确认构建的是最新二进制。

---

## 18. 已知限制

| 限制 | 影响 | 建议 |
| --- | --- | --- |
| HGDB 镜像缺少 `plpgsql_check` | `db lint` 不可用 | 安装扩展或跳过 |
| HGDB 镜像缺少 `pgtap` | `db test` 不可用 | 安装扩展或跳过 |
| K8s PostgREST 只接受 JWT key | opaque `sb_*` key 报 `PGRST301` | 使用 `higobase-jwt` |
| K8s Analytics/Meta Kong 路由受限 | `/analytics/health` 401、`/pg/health` 403 | 端口转发直连 |
| `db diff` 可能报告系统扩展差异 | diff 噪音 | 用 `--schema` 聚焦业务 |
| `db push` 校验远端迁移历史 | 历史不一致时拒绝 | `migration list` + repair/pull |
| `start --from-backup` | HighgoBase runtime 不支持 | 使用迁移/seed 或手动恢复 |
| `functions deploy` 面向 Cloud | 不能部署到自托管 K8s | Helm/集群部署 |
| `storage` 云端模式需 token | 本地 `--local` 已可用 | K8s 用 REST |
| `db reset` 会删库 | 不能用于业务库 | 只在隔离库使用 |
| `doctor` 只检查本地 Docker | 不代表 K8s 状态 | K8s 用 `kubectl` |
| Pooler/Inbucket 默认关闭 | 相关镜像 WARN | 按需启用 |

---

## 19. 验证记录

### 19.1 本地 Docker 全栈

在 `/root/hgcli-full` 测试项目上执行：

```bash
highgobase stop
highgobase start
highgobase status
highgobase doctor
```

结果：

- `stop` 退出 0；
- `start` 退出 0，未使用 `--ignore-health-check`；
- 全部启用服务健康检查通过；
- 重启后 `demo_items` 数据和 Storage bucket 保留；
- `doctor` 除两个禁用服务镜像外全部 `[OK]`；
- `status` 的 pretty/env/json 输出正常。

API 验证：

| 功能 | 结果 |
| --- | --- |
| REST SELECT/INSERT/PATCH/DELETE | 通过 |
| RLS anon 策略 | 通过 |
| Auth 注册/登录/user | 通过 |
| Storage bucket/upload/list/download/signed URL/delete | 通过 |
| Functions `/functions/v1/hello` | 200 |
| GraphQL `{ __typename }` | 200 |
| Realtime WebSocket | 101 + `phx_join` ok |
| Studio | 200（Basic Auth） |
| Analytics `/health` | 200 |
| Meta 容器健康 | healthy |

数据库/CLI：

| 命令 | 结果 |
| --- | --- |
| `migration new/list` | 通过 |
| `db push --local` / `--include-all` | 通过 |
| `db diff --local` | 通过 |
| `db dump --dry-run` | 通过 |
| `gen types --local --lang typescript` | 通过 |
| `gen types --local --lang go` | 通过 |
| `inspect db` 13 个子命令 | 通过 |
| `inspect report --local` | 通过 |
| `services` | 通过 |
| `db lint` / `db test` | 扩展缺失，已知限制 |

### 19.2 K8s `higobase`

详见 [`HIGHGOBASE_K8S_TEST_REPORT.md`](./HIGHGOBASE_K8S_TEST_REPORT.md)。摘要：

- CLI 版本/帮助/init：通过；
- `db push/dump/diff/pull`、`migration`：通过（使用隔离库/干净迁移）；
- `gen types` TS/Go：通过；
- `inspect db` 13 个子命令：通过（主库只读巡检）；
- REST/Auth/Storage/Functions/Realtime/GraphQL：通过；
- Studio：通过；Meta/Analytics 直连健康：通过；
- `db lint` / `db test`：受扩展限制。

### 19.3 自动化测试

本次修复涉及的主要 Go 包测试：

```bash
go test ./internal/highgobase ./internal/db/start ./internal/start \
  ./internal/inspect/role_stats ./internal/db/diff ./internal/db/lint \
  ./internal/utils/flags

cd pkg
go test ./migration/...
```

以上均通过。仓库中个别 Windows 基线测试（迁移格式化/路径分隔符）与本次 HighgoBase
改动无关，未纳入本次修复范围。

---

## 20. 快速命令备忘

```bash
# 环境
export HIGHGOBASE_HOME=/path/to/higobase
export HIGHGOBASE=1

# 本地栈
highgobase init
highgobase doctor
highgobase start
highgobase status -o env
eval "$(highgobase status -o env)"
highgobase stop

# 迁移
highgobase migration new add_table
highgobase migration list
highgobase db push --local
highgobase db diff --local
highgobase db dump --local --schema public --file public.sql
highgobase db reset --local

# 类型/巡检
highgobase gen types --local --lang typescript
highgobase inspect db db-stats --local
highgobase inspect report --local --output-dir ./inspect-report

# 函数/存储
highgobase functions new hello
highgobase functions serve --no-verify-jwt
highgobase --experimental storage ls --local ss:///demo-bucket/
highgobase --experimental storage cp --local ./file.txt ss:///demo-bucket/file.txt

# K8s
kubectl -n higobase port-forward --address 127.0.0.1 svc/higobase-db 15432:5866
export DB_URL='postgresql://postgres:<password>@127.0.0.1:15432/postgres?sslmode=disable'
highgobase migration list --db-url "$DB_URL"
highgobase db push --db-url "$DB_URL" --yes
highgobase gen types --db-url "$DB_URL" --lang typescript
highgobase inspect db role-stats --db-url "$DB_URL"
```

---

## 21. 参考

- 上游 Supabase CLI：`v2.76.14`
- HighgoBase 自托管资产：`higobase/docker/docker-compose.hgdb.yml`
- HGDB 初始化脚本：`higobase/docker/volumes/db/hgdb_init_scripts/`
- Kong 声明式配置：`internal/start/templates/kong.yml`
- HighgoBase 默认值：`internal/highgobase/highgobase.go`
- K8s 验证报告：[`HIGHGOBASE_K8S_TEST_REPORT.md`](./HIGHGOBASE_K8S_TEST_REPORT.md)