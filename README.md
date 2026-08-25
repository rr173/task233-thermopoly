# task233-thermopoly 药物晶型转变热分析判读服务

基于 Go 实现的药物晶型转变热分析判读 Web 项目，一款后端服务，导入 DSC/TGA 热分析曲线与升温程序、完成基线校正与峰区间检测、按晶型先验判定转变/脱除/熔融事件并发布不可变判读快照。

## 业务背景

药物固态化学中，不同晶型（多晶型）具有不同的物理化学性质。差示扫描量热（DSC）与热重分析（TGA）是表征晶型转变、溶剂脱除与熔融的常规手段。本服务帮助药物分析人员：

1. 从 DSC 热流曲线定位吸热/放热峰区间；
2. 用 TGA 质量曲线佐证是否伴随质量损失（晶型转变≈无损失，溶剂脱除有损失）；
3. 依据晶型先验知识（温度窗口 + 热流方向 + 质量损失上限）自动判定事件类型；
4. 对峰重叠造成的判读不确定性进行标记、人工复核与证据拆分；
5. 发布版本化、输入冻结的判读快照，保证结论可追溯。

## 核心闭环与状态机

**业务闭环（5 步）**：创建试验 → 导入 DSC/TGA 曲线与升温程序（哈希幂等）→ 基线校正 → 峰检测 → 晶型事件判读（重叠标记）→ 事件裁决/拆分 → 快照发布。

**状态机**：

| 实体 | 状态流转 |
|---|---|
| 试验 Trial | `receiving → pending_review → needs_review → confirmed → sealed`（只进不退，封存终态） |
| 热分析段 Segment | `raw → baseline_corrected / anomalous / duplicate` |
| 转变事件 Event | `candidate → overlapping / confirmed / vetoed`（重叠须拆分后才可确认） |
| 判读快照 Snapshot | `draft → published → superseded` |

## 构建与运行

```bash
# 标准构建门禁（必须全部通过）
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/thermopoly --smoke-test

# 启动服务（默认 :8080，SQLite 落盘）
go run ./cmd/thermopoly --addr :8080 --db ./thermopoly.db
```

`--smoke-test`：创建试验 → 导入合成 DSC（双重叠峰）/TGA 曲线 → 基线校正 → 峰检测 → 事件生成（重叠→needs_review）→ 证据拆分 → 裁决 → 快照发布 → 封存 → **关闭并重开同一数据库验证持久化恢复** → 校验快照输入冻结，全部通过退出码 0。

## API 一览（前缀 /api）

| 能力 | API | 说明 |
|---|---|---|
| 试验 | `POST /api/trials` | 创建试验（receiving） |
| | `GET /api/trials?status=&limit=` | 列出 |
| | `GET /api/trials/{id}` | 详情 |
| | `PATCH /api/trials/{id}` | 状态机流转 `{"status":"..."}` |
| | `POST /api/trials/{id}/seal` | 封存（终态） |
| 升温程序 | `PUT /api/trials/{id}/program` | 设置/更新程序（版本递增） |
| | `GET /api/trials/{id}/program` | 取激活程序 |
| 曲线 | `POST /api/trials/{id}/curves` | 导入 DSC/TGA（哈希幂等，拒绝混用单位） |
| | `GET /api/trials/{id}/curves` | 列出 |
| | `GET /api/curves/{id}` | 详情 |
| | `GET /api/trials/{id}/segments` | 分析段 |
| 分析 | `POST /api/trials/{id}/baseline` | 基线校正 |
| | `POST /api/trials/{id}/peaks/detect` | 峰检测 |
| | `GET /api/trials/{id}/peaks` | 峰列表 |
| | `POST /api/trials/{id}/events/generate` | 晶型事件判读 |
| | `GET /api/trials/{id}/events` | 事件列表 |
| | `PATCH /api/events/{id}` | 裁决 confirmed/vetoed/overlapping |
| | `POST /api/events/{id}/split` | 重叠事件证据拆分 |
| 快照 | `POST /api/trials/{id}/snapshots` | 创建草稿 |
| | `GET /api/trials/{id}/snapshots` | 列出 |
| | `GET /api/snapshots/{id}` | 详情 |
| | `POST /api/snapshots/{id}/publish` | 发布（冻结输入） |
| | `GET /api/snapshots/{id}/verify` | 校验输入冻结 |
| 先验/统计 | `GET/POST /api/priors`、`GET/PATCH /api/priors/{id}` | 晶型先验管理 |
| | `GET /api/stats`、`GET /api/health` | 统计与健康 |

## 持久化

- SQLite（`modernc.org/sqlite` 纯 Go 驱动，CGO 无关）。表：`trials`、`programs`、`curves`、`segments`、`peaks`、`events`、`snapshots`、`priors`。
- 重启恢复：`--smoke-test` 演示关闭重开同一 DB 后试验/峰/事件/快照完整恢复；未完成峰检测的试验可通过 `peaks/detect` 续跑（幂等）。
- 同一曲线哈希幂等（重复导入被拒）；封存试验冻结输入；发布快照记录曲线哈希指纹，`verify` 校验输入未变。

## 关键不变量

- 同一试验温度单位必须一致（C/K 混用拒绝）；采样间隔 ≤10K；曲线温度严格递增。
- 不同样品（试验）可并行处理；同一试验判读串行（per-trial 锁）。
- 重叠峰必须返回不确定性（事件标 `overlapping`），确认前必须拆分并附证据。
- 状态机只进不退；`sealed` 为终态，拒绝一切输入修改。
