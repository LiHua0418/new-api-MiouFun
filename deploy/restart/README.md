# 通过 `docker restart new-api` 加载已准备的程序

本目录保存 2026-09-23 已验证的 New API 启动器。它适用于已有 `/data`
持久化挂载、以 `/new-api` 为入口的容器。源码中的 CPU 修复保持 tokenizer
v0.6.2 的词表、模型映射和 token ID，优化长片段的 BPE 合并；同时保留
GPT Image 2 的 URL 转 `b64_json` 兼容处理。

## 当前发布约定

```text
/new-api                                      # entrypoint.sh 的容器内副本
/data/new-api-runtime/current                 # 指向 releases/<release> 的相对符号链接
/data/new-api-runtime/releases/<release>/new-api
/data/new-api-runtime/releases/<release>/SHA256
```

每次启动，入口解析一次 `current`，检查目标程序和 `SHA256` 文件中的裸哈希，
设置 `SKIP_DATABASE_MIGRATION=true`，再通过 `exec` 启动程序。参数和其他环境
变量继承原容器；Go 程序成为 PID 1，直接接收 Docker 的停止信号。

部署分两步：先编译、验证并准备好一个完整版本，再执行：

```bash
docker restart new-api
```

修改源码、`git pull` 或推送仓库不会自动编译，也不会改变已准备的程序。
使用本目录方案前，需要一次性安装启动入口。当前服务器已经完成安装；
默认仓库 Dockerfile 仍直接启动内置程序，没有自动启用此可选部署方式。

## 准备和验证版本

1. 按仓库构建流程生成前端和 Linux Go 程序，包含 `third_party/tokenizer`
   本地模块。使用与运行环境兼容的工具链和架构。先通过相关回归测试。
2. 为每个版本创建独立目录，将程序写入 `new-api` 并赋予执行权限；把其
   SHA256 十六进制摘要写入 `SHA256`，然后验证副本。禁止原地覆盖正在
   执行的二进制文件。
3. 确认该程序支持 `SKIP_DATABASE_MIGRATION`，且适配现有数据库结构。
   该开关默认关闭；启动器启用它，跳过主数据库和独立日志数据库迁移。
   它不禁止应用正常请求产生的数据库写入，也不适用于尚未初始化的数据库。
4. 先备份原容器 `/new-api` 和当前版本选择。首次安装时将 `entrypoint.sh`
   复制到容器的临时路径，校验后通过同目录 `mv` 原子替换 `/new-api`，
   避免直接覆盖正在运行的可执行文件。当前 Go 进程继续运行原程序。
5. 为已经验证的版本创建临时符号链接，在同一文件系统原子替换 `current`。
   已完成全部准备后再执行重启。后续更新只需准备版本、切换链接和重启。

上线后检查实际进程，而不是只看镜像名称：

```bash
docker exec new-api readlink /proc/1/exe
docker exec new-api sha256sum /proc/1/exe /data/new-api-runtime/current/new-api
curl --fail --silent --show-error http://127.0.0.1:3000/api/status
```

核对两个哈希一致，健康接口的 `success` 为 `true`，并检查本次启动的
日志是否包含 `database migration skipped by configuration`。

## 已上线版本及验证范围

- 版本目录：`tokenizer-cpu-fix-20260921`。
- 上线验证日期：2026-09-23；基线提交：`4e78ccf832fabcafd62bcb02e2665fd6752c6b8d`。
- 当时程序的 SHA256：`fc551fcefb30aa2e695038d10d0327e075abeaa32e5c55addd531a67ff9f252d`。
- 该哈希标识现有构建产物；重新构建可能因工具链、版本字符串或构建元数据
  产生不同哈希，应独立验证，不能当作可重复构建保证。
- token ID/count 对照原算法、并发使用、预扣费计算、上游 usage 优先以及
  缺失 usage 的本地回退均有回归测试。迁移开关使用临时 SQLite 验证。
- 启动器在断网临时容器验证了参数传递、`exec`、校验失败、原入口回退，
  以及普通 `docker restart` 在同一容器切换程序；生产确认了实际程序
  哈希、HTTP 健康状态和跳过迁移日志。

可从仓库根目录执行相关测试（与生产数据库隔离）：

```bash
go test github.com/tiktoken-go/tokenizer/...
go test ./service ./controller ./model ./relay/channel/openai ./relay/helper ./pkg/billingexpr
```

## 停止行为、持久化与回退

普通 `docker restart` 沿用容器的停止超时，可能打断长请求，包括未完成
的结算或异步退款。这一方案只改变程序加载方式，没有实现请求排空或无中断
切换。重启前应让在途请求结束；延长停止等待也不保证所有请求完成。

`docker restart` 和宿主机重启保留当前容器的可写层。删除或重建容器会
丢失其中的启动器，即使 `/data` 中的版本仍然存在。需要支持容器重建时，
应另行将启动器纳入镜像或 Compose 挂载；本次没有改动生产 Compose。
Docker 显示原镜像名称不代表仍运行原程序，以 `/proc/1/exe` 为准。

回退到已验证、支持跳过迁移且兼容现有数据库的旧版本时，原子恢复
`current` 链接，再安排重启。首次安装前的原程序可能不支持跳过迁移，
不可假定恢复入口后重启仍会跳过数据库迁移。原二进制、服务器配置、
数据库备份、日志和密钥保留在服务器，不纳入本仓库。
