# HighgoBase CLI K8s 验证报告

> 验证日期：2026-11-19  
> CLI 基线：Supabase CLI `v2.76.14` / 分支 `highgobase-cli-base-2.76.14`  
> 被测环境：Kubernetes `higobase` 命名空间  
> 结论：**CLI 在 K8s 自托管 HighgoBase 环境中的数据库命令与 HTTP 服务链路可用。**

---

## 1. 测试环境

| 项目 | 值 |
| --- | --- |
| 服务器 | `192.168.21.145` |
| SSH 用户 | `root` |
| Namespace | `higobase` |
| Helm Release | `higobase` |
| Chart | `higobase-0.7.2` |
| Kong NodePort | `http://192.168.21.145:32626` |
| DB Service | `higobase-db:5866` |
| DB 端口转发 | `127.0.0.1:15432 -> 5866` |
| DB 引擎 | HighgoBase 9.0.5.1（PostgreSQL 14 兼容） |
| Auth | GoTrue `v2.186.0`，`MAILER_AUTOCONFIRM=true` |
| PostgREST | `14.8`，schema：`public,storage,graphql_public` |
| Studio | Basic Auth（本次 `admin` / `mypassword`） |
| CLI 版本 | `2.76.14` |

K8s Pod 状态：DB、REST、Auth、Storage、Functions、Realtime、Kong、Studio、Meta、
Analytics、Vector、MinIO、imgproxy 均为 Running。

> 本报告不记录业务表内容、业务数据、数据库密码、JWT key 或其他 Secret 值。
> 测试只使用只读查询、隔离数据库和临时对象。

---

## 2. 验证范围与方法

### 2.1 范围

1. CLI 版本、帮助、初始化、配置加载；
2. 通过 `--db-url` 连接 K8s HGDB 的数据库命令；
3. 迁移历史与 `db push` / `migration`；
4. `db dump`、`db diff`、`db pull`；
5. `gen types`（TypeScript/Go）；
6. `inspect db` 全部 13 个子命令与 `inspect report`；
7. REST + RLS；
8. Auth 注册/登录/用户信息；
9. Storage bucket/object/signed URL；
10. Edge Functions；
11. GraphQL；
12. Realtime WebSocket；
13. Studio；
14. Meta / Analytics 健康检查。

### 2.2 安全边界

- 主库 `postgres` 只做只读查询/巡检与元数据读取；
- 迁移、push、pull、diff、reset 只在隔离库 `hgcli_verify` / `hgcli_reset` 验证；
- REST CRUD 使用临时表 `hgcli_rest_probe_<timestamp>`，测试后删除；
- Storage 使用临时 bucket `hgcli-probe-<timestamp>`，测试后删除；
- Auth 使用临时邮箱用户，测试后通过 Admin API 删除；
- 未删除或修改 `higobase` namespace 及任何业务表。

---

## 3. 结果矩阵

### 3.1 CLI 与数据库

| 功能 | 命令/方法 | 结果 | 说明 |
| --- | --- | --- | --- |
| 版本 | `highgobase --version` | PASS | `2.76.14` |
| 帮助 | `highgobase --help` | PASS | 显示 `highgobase` 名称与 `supabase` 别名 |
| 初始化 | `highgobase init` | PASS | 生成 `major_version=14` 配置 |
| 数据库连接 | `db dump --db-url ... --dry-run` | PASS | 输出 58 行 pg_dump 脚本 |
| 迁移列表 | `migration list --db-url ...` | PASS | Local/Remote 对齐 |
| `db push --dry-run` | 隔离库 | PASS | 正确列出待应用迁移 |
| `db push --yes` | 隔离库 | PASS | 实际应用新迁移并写入历史 |
| `migration up` | 隔离库 | PASS | 已应用时返回 up to date |
| `db dump --schema` | 主库/隔离库 | PASS | 只读导出 |
| `db dump --data-only` | 主库/隔离库 | PASS | 只读导出 |
| `db dump --role-only` | 主库/隔离库 | PASS | 只读导出 |
| `db diff` | 干净迁移项目 | PASS | 成功创建 shadow DB 并输出差异 |
| `db pull` | 干净迁移项目 | PASS | 写入 remote schema 并修复历史 |
| `gen types --lang typescript` | `--db-url` | PASS | 输出 1039 行（主库） |
| `gen types --lang go` | `--db-url` | PASS | 输出 525 行（主库） |
| `inspect db` 13 个子命令 | `--db-url` 主库 | PASS | 见 3.2 |
| `inspect report` | `--db-url` | PASS | 生成 CSV |
| `db lint` | `--db-url` | BLOCKED | 镜像缺 `plpgsql_check` |
| `db test` | `--db-url` | BLOCKED | 镜像缺 `pgtap` |

### 3.2 `inspect db` 子命令

以下命令均对 K8s 主库执行只读巡检，退出码为 0：

```text
db-stats              PASS
table-stats           PASS
index-stats           PASS
role-stats            PASS（修复 NULL 扫描后）
vacuum-stats          PASS
traffic-profile       PASS
blocking              PASS
locks                 PASS
long-running-queries  PASS
replication-slots     PASS
calls                 PASS
outliers              PASS
bloat                 PASS
```

`role-stats` 初次执行时报：

```text
failed to scan targets: can't scan into dest[3]: cannot scan null into *string
```

根因是 HGDB 中部分角色的 `rolconfig` 为 NULL。已改为：

```sql
COALESCE(array_to_string(rolconfig, ',', '*'), '') AS custom_config
```

修复后在 K8s 和本地均通过，并增加了单元测试。

### 3.3 REST 与 RLS

使用临时表 `hgcli_rest_probe_<timestamp>`，开启 RLS 并创建 anon 策略：

| 操作 | HTTP 状态 | 结果 |
| --- | --- | --- |
| INSERT（anon + RLS） | `201` | 返回插入行 |
| SELECT（anon + RLS） | `200` | 返回插入行 |
| PATCH（anon + RLS） | `200` | 返回更新后的行 |
| DELETE（anon + RLS） | `204` | 删除成功 |
| `GET /rest/v1/`（apikey） | `200` | 返回 OpenAPI/Swagger |

使用 JWT `anonKey` 和 `Authorization: Bearer` 头；未使用 opaque `sb_*` key。

### 3.4 Auth

| 操作 | HTTP 状态 | 结果 |
| --- | --- | --- |
| `POST /auth/v1/signup` | `200` | 临时用户注册成功 |
| `POST /auth/v1/token?grant_type=password` | `200` | 返回 access token |
| `GET /auth/v1/user` | `200` | 返回临时用户邮箱/ID |
| `GET /auth/v1/health` | `200` | GoTrue v2.186.0 |
| Admin 删除临时用户 | `200` | 清理完成 |

### 3.5 Storage

使用临时 bucket `hgcli-probe-<timestamp>`：

| 操作 | HTTP 状态 | 结果 |
| --- | --- | --- |
| 创建 bucket | `200` | 公开 bucket |
| 上传对象 | `200` | 返回 Key/Id |
| 列表对象 | `200` | 返回对象元数据 |
| 公开下载 | `200` | 内容正确 |
| 生成 signed URL | `200` | 返回 `signedURL` |
| 通过 signed URL 下载 | `200` | 内容正确 |
| 删除对象 | `200` | 成功 |
| 删除 bucket | `200` | 成功 |
| `GET /storage/v1/status` | `200` | 服务健康 |

### 3.6 Functions / GraphQL / Realtime / Studio

| 功能 | 方法 | 结果 |
| --- | --- | --- |
| Edge Function | `POST/GET $BASE/functions/v1/legal-probe` + JWT | `200`，返回函数 JSON |
| GraphQL | `POST $BASE/graphql/v1` `{ __typename }` | `200`，`{"data":{"__typename":"Query"}}` |
| Realtime | WebSocket Upgrade 握手 | `101 Switching Protocols` |
| Studio | `GET $BASE/` + Basic Auth | `200` |
| Meta 直连 | `port-forward svc/higobase-meta 18080:8080` + `/health` | `200` |
| Analytics 直连 | `port-forward svc/higobase-analytics 14000:4000` + `/health` | `200` |

经 Kong 访问时：

```text
GET /pg/health        -> 403 You cannot consume this service
GET /analytics/health -> 401 Unauthorized
```

这是当前 Kong 路由/认证策略，不代表 Meta/Analytics 不健康；直连健康检查均 200。

---

## 4. 修复与改进

### 4.1 `role-stats` NULL 扫描

- 问题：HGDB 角色 `rolconfig` 为 NULL，CLI 扫描到 `string` 失败。
- 修复：SQL 使用 `COALESCE(..., '')`。
- 验证：K8s 与本地 `inspect db role-stats` 均通过；单元测试通过。

### 4.2 `storage --local` 被误判为 Management API

- 问题：`storage --local` 仍要求 Supabase access token，导致本地 Storage CLI 不可用。
- 修复：`cmd/storage.go` 在 `--local` 时把命令组切换到 local dev，跳过 `promptLogin`。
- 验证：本地 `storage cp/ls/mv/rm --local` 全部通过（`rm` 需 `--yes`）。

### 4.3 License 检查

- 问题：只检查目录内任意文件会把 `client_info.dat` 误判为 License。
- 修复：`LicenseDir` 明确检查 `license.dat`。
- 验证：远程环境 `doctor` 显示 `[OK] hgdb license`；单测覆盖仅有 `client_info.dat` 的情况。

### 4.4 其他兼容修复

- Kong 3.9 自定义 Nginx 模板补充 LMDB/worker 配置；
- Kong 声明式配置版本升至 `_format_version: "2.1"`；
- HGDB SQL 脚本 CRLF 归一化；
- `sslmode=disable` 在远程连接串中保留；
- `pg_stat_statements` schema 适配；
- 迁移后发送 `NOTIFY pgrst, 'reload schema'`；
- `drop.sql` 按 owner/系统 schema 过滤；
- `db diff/lint` 默认过滤 HGDB 系统 schema。

---

## 5. 已知问题与限制

| 问题 | 影响 | 处理 |
| --- | --- | --- |
| `plpgsql_check` 缺失 | `db lint` 无法启用 | 在镜像中安装扩展，或跳过 |
| `pgtap` 缺失 | `db test` 无法启用 | 在镜像中安装扩展，或跳过 |
| opaque key 被 PostgREST 拒绝 | `PGRST301` | 使用 `higobase-jwt` 的 JWT key |
| Meta/Analytics Kong 路由受限 | 403/401 | 端口转发直连 |
| `db diff` 全库 dump 迁移含 owner 语句 | shadow DB 报 `must be owner` | 使用干净业务迁移 |
| `db pull` 远端历史不匹配 | CLI 拒绝 pull | `migration list` + repair/pull |
| `db push` 需要远端历史与本地文件一致 | 拒绝推送 | 先修复/同步历史 |
| PostgREST schema cache 延迟 | 新表短暂 `PGRST205` | 等待 1-2 秒或 `NOTIFY pgrst` |
| `db reset` 会删库 | 业务数据风险 | 只在隔离库执行 |
| HGDB 未启用 TLS | `sslmode=require` 失败 | 使用 `?sslmode=disable` |
| CLI 不能向自托管 K8s 部署 Functions | `functions deploy` 不适用 | 通过 Helm/集群部署 |

---

## 6. 清理与影响

本次测试完成后执行了以下清理：

- 删除临时 REST 表 `hgcli_rest_probe_*`；
- 删除临时 Storage bucket `hgcli-probe-*` 及对象；
- 删除临时 Auth 用户（`hgcli-probe-*@example.com`）；
- 删除临时迁移项目目录；
- 删除 `db pull` 在隔离库 `hgcli_reset` 中新增的历史记录；
- 未修改 `higobase` 命名空间、Helm Release、业务表或业务数据。

验证 SQL 均为只读或针对隔离库/临时对象。

---

## 7. 最终结论

1. `highgobase` CLI 能以 `--db-url` 方式连接 K8s `higobase` 环境中的 HGDB，并完成
   迁移、导出、类型生成和巡检。
2. K8s 上的 REST、Auth、Storage、Functions、GraphQL、Realtime、Studio 均可通过
   Kong NodePort + JWT key 正常工作。
3. `db lint` / `db test` 受当前 HGDB 镜像扩展限制；这是环境限制，不是 CLI 崩溃。
4. 当前 K8s PostgREST 必须使用 JWT anon/service key；opaque `sb_*` key 不兼容。
5. 对业务库执行迁移前，应先比对迁移历史；`db reset` 只允许在可丢弃库执行。