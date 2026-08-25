基于 Go 实现的药物晶型转变热分析判读 Web 项目，一款后端服务，导入 DSC/TGA 热分析曲线与升温程序、完成基线校正与峰区间检测、按晶型先验判定转变/脱除/熔融事件并发布不可变判读快照。

# BENZHI 评测说明

## 项目类型

药物晶型转变热分析判读服务（task233-thermopoly）：从 DSC（差示扫描量热）热流曲线与 TGA（热重分析）质量曲线出发，完成基线校正、峰区间检测、晶型先验事件判读、重叠不确定性处理与版本化快照发布。纯后端 Web 服务，SQLite 持久化。

## 环境与依赖

- Go 1.26.3（`GOTOOLCHAIN=local`、`CGO_ENABLED=0`）
- SQLite：`modernc.org/sqlite v1.52.0`（纯 Go，无 CGO）
- GOPROXY=`https://goproxy.cn,direct`，GOSUMDB=`sum.golang.google.cn`

## 构建与验证命令

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/thermopoly --smoke-test
```

## 双架构 Docker 验证

```bash
bash build_benzhi_docker.sh my-project linux/amd64
bash build_benzhi_docker.sh my-project linux/arm64
```

镜像 ENTRYPOINT=`/bin/thermopoly`，CMD=`--smoke-test`。`docker run` 时只传 `--smoke-test` 标志，不追加路径参数。

## --smoke-test 契约

`--smoke-test` 不启动长驻服务，按顺序执行并全部通过后以退出码 0 结束：

1. 创建试验（Celsius 单位）与升温程序；
2. 导入合成 DSC 曲线（120°C / 126°C 两个重叠吸热峰）与 TGA 曲线（124-127°C 失重 5%）；
3. 创建晶型先验（FormA→FormB，110-130°C 吸热，质量损失≤0.5%）；
4. 基线校正 → 峰检测（断言检出 2 个峰且均标记重叠）；
5. 事件生成（两个事件均 `overlapping`，试验进入 `needs_review`）；
6. 补充 TGA 证据拆分重叠（FormA→FormB / 溶剂脱除）→ 裁决全部 `confirmed`；
7. 创建并发布快照（试验推进 `confirmed`）→ 封存试验；
8. 关闭并重新打开同一 SQLite 文件，断言试验/峰/事件/快照全部恢复（重启恢复）；
9. 校验已发布快照输入冻结性（VerifySnapshotInput）与封存试验拒绝修改。

任一断言失败即打印 `smoke-test FAILED` 并退出码 1。

## API 入口（前缀 /api）

试验 `POST/GET /api/trials`、`GET/PATCH /api/trials/{id}`、`POST /api/trials/{id}/seal`；程序 `PUT/GET /api/trials/{id}/program`；曲线 `POST/GET /api/trials/{id}/curves`、`GET /api/curves/{id}`、`GET /api/trials/{id}/segments`；分析 `POST /api/trials/{id}/baseline`、`POST /api/trials/{id}/peaks/detect`、`GET /api/trials/{id}/peaks`、`POST /api/trials/{id}/events/generate`、`GET /api/trials/{id}/events`、`PATCH /api/events/{id}`、`POST /api/events/{id}/split`；快照 `POST/GET /api/trials/{id}/snapshots`、`GET /api/snapshots/{id}`、`POST /api/snapshots/{id}/publish`、`GET /api/snapshots/{id}/verify`；先验 `GET/POST /api/priors`、`GET/PATCH /api/priors/{id}`；`GET /api/stats`、`GET /api/health`。

## 数据库

SQLite 文件（`--db` 指定，默认内存）。表：trials、programs、curves、segments、peaks、events、snapshots、priors。同一曲线哈希幂等；封存试验冻结输入；发布快照冻结曲线哈希指纹。
