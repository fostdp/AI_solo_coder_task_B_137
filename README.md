# 🏹 三弓床弩弹射动力学仿真与穿甲威力分析系统

面向宋代三弓床弩复原研究的全栈工程化实现：**UDP 传感器采集 → 多体动力学/弹道仿真 → 侵彻力学威力评估 → MQTT 告警推送**，配合 ClickHouse 历史归档、Prometheus 指标、pProf 性能剖析和 Docker 一键部署。

---

## 📐 架构总览

```
                                   ┌─────────────────────────────────────────────────────────────┐
                                   │                        Docker Host                          │
                                   │                                                             │
  真实传感器 ───── UDP:8080 ─────┐│                                                             │
                                 ▼▼                                                             │
   ┌──────────────────────────────────────────┐     ┌──────────────────────┐                     │
   │  ballistics-server (Go, static binary)   │────▶│  ClickHouse 24.3     │                     │
   │  ├─ udp_receiver        (校验/入队)       │     │  5 原始表 + 4 MV     │                     │
   │  ├─ ballistic_simulator (多体+弹道)       │     │  小时/日 降采样      │                     │
   │  ├─ penetration_analyzer (陀螺+侵彻)      │     │  TTL 1年 自动清理    │                     │
   │  ├─ alarm_mqtt          (评估+推送)       │     └──────────┬───────────┘                     │
   │  └─ /api + /metrics + /debug/pprof        │                │                                 │
   └──────┬───────────┬───────────┬────────────┘                │                                 │
          │           │           │                              │                                 │
          ▼           │           ▼                    Port 8123/9000 (外连)                      │
    channel job       │    Alert topic=                           │                                 │
    编排+路由        │     ballistics/alerts/*                   │                                 │
          │           │                                          │                                 │
          │           ▼                                          │                                 │
          │   ┌───────────────┐                                  │                                 │
          │   │   Mosquitto   │◀────── MQTT WS 8083 ── 外连     │                                 │
          │   │  MQTT Broker  │   Port 1883                       │                                 │
          │   └───────────────┘                                  │                                 │
          │                                                      │                                 │
          └────▶ Prometheus (可选, :9091) ◀── /metrics ──────────┘                                 │
                                                                 │                                 │
  传感器模拟器 ── UDP ──────────────────────────────────────────┘                                 │
  (Python, 4 拉力档 / 4 箭镞)                                                                     │
                                                                                                   │
   ┌──────────────────────────────────────────────────────────────────────────────────────────────│
   │  nginx:80  ──────► /          = 前端 index.html + bed_crossbow_3d.js + power_panel.js (Gzip)│
   │                    /api/*     = 反代 ballistics-server:8081                                  │
   │                    /metrics   = 反代 ballistics-server:9090 (内网ACL)                        │
   └───────────────────────────────────────────────────────────────────────────────────────────────┘
```

| 模块 | 镜像 / 入口 | 端口 | 责任 |
|------|-------------|------|------|
| **ballistics-server** | 自研 `Dockerfile.backend` 多阶段构建 | `8080/udp` · `8081/tcp` · `9090/tcp` | UDP 采集校验、弹道仿真、侵彻威力、告警推送、HTTP API、指标 & pprof |
| **clickhouse** | `clickhouse/clickhouse-server:24.3` | `8123` · `9000` | 5 张原始 MergeTree + 1 张小时 SummingMV + 3 张日聚合 AggregatingMV，TTL 1~3 年 |
| **mqtt** | `eclipse-mosquitto:2.0` | `1883` · `8083(ws)` | 告警分级 Topic 路由 |
| **sensor-simulator** | 自研 `Dockerfile.simulator`（Python 3.11 slim） | — (只发 UDP) | 4 档拉力、4 种箭镞、可调老化速度 |
| **nginx** | `nginx:1.27-alpine` | `80` | 前端静态资源 (Gzip) + `/api` 反代 + CORS + 指标 ACL |

---

## 🚀 快速部署（Docker Compose）

### 1. 准备

```bash
# 1) 拷贝环境变量模板
cp .env.template .env

# 2) 按需修改 .env：
#   SIM_LEVEL=extreme     # 想测裂纹告警就用 extreme
#   SIM_ARROW=bodkin      # 穿甲箭镞
#   SIM_INTERVAL=5        # 5 秒一发，快速看效果
```

### 2. 拉镜像 / 构建 & 启动

```bash
# 预览最终配置 (先检查无报错)
docker compose config

# 构建自定义镜像 + 一键启动 5 个服务
docker compose up -d --build

# 查看健康状态
docker compose ps
# NAME             STATUS          PORTS
# bs-clickhouse    healthy (healthy)
# bs-mqtt          running
# bs-server        healthy
# bs-sim           running
# bs-nginx         healthy
```

### 3. 访问

| 入口 | URL |
|------|-----|
| **前端 3D 大屏** | http://localhost/ |
| **HTTP API (Gin)** | http://localhost/api/v1/health |
| **穿甲对比 POST** | http://localhost/api/v1/penetrate/compare |
| **铠甲类型 GET** | http://localhost/api/v1/armors |
| **Prometheus 指标** (仅内网/Docker 网) | http://localhost:9090/metrics |
| **pprof 性能剖析** (仅内网) | http://localhost:9090/debug/pprof/ |
| **ClickHouse HTTP** | http://localhost:8123/play |
| **MQTT TCP** | `mqtt://localhost:1883` |

### 4. 停止 / 清理

```bash
# 停止保留数据
docker compose down

# 全部清空 (含 ClickHouse 数据卷)
docker compose down -v
```

---

## 📡 传感器模拟器用法

### CLI 命令（本地或容器内）

```bash
# 先看帮助
python simulator/sensor_simulator.py -h
python simulator/sensor_simulator.py --list-levels
python simulator/sensor_simulator.py --list-arrows
```

### 拉力等级速查

| 档位 | 典型拉力 | 典型初速 | 典型变形 | 典型自旋 | 适用场景 |
|------|---------|---------|---------|---------|---------|
| **light** | ~2500 N | ~85 m/s | ~5 mm | ~18 Hz | 训练 / 可靠性测试 |
| **mid** | ~4500 N | ~120 m/s | ~8 mm | ~25 Hz | 实战标准（默认） |
| **high** | ~6500 N | ~150 m/s | ~11 mm | ~32 Hz | 攻城 / 远程 |
| **extreme** | ~8200 N | ~175 m/s | ~14 mm | ~38 Hz | 破甲 / **触发裂纹告警** |

### 箭镞类型速查

| 类型 | 质量系数 | 自旋系数 | 用途 |
|------|---------|---------|------|
| **bodkin** (默认) | ×1.00 | ×1.00 | 穿甲 |
| **broadhead** | ×1.15 | ×0.85 | 反人员 |
| **blunt** | ×1.30 | ×0.60 | 破盾 / 眩晕 |
| **whistler** | ×0.95 | ×0.90 | 信号 / 恐吓 |

### 常见场景

```bash
# 场景 1: 极限拉力, 穿甲箭镞, 5 秒一发, 打 100 发观察裂纹告警
python simulator/sensor_simulator.py \
  --level extreme --arrow bodkin -i 5 -n 100

# 场景 2: 三台设备并跑, 不同拉力档 (推荐在 compose 中启 3 个 sim 容器)
python simulator/sensor_simulator.py --device chuangnu-001 --level light   -i 30 &
python simulator/sensor_simulator.py --device chuangnu-002 --level mid     -i 30 &
python simulator/sensor_simulator.py --device chuangnu-003 --level extreme -i 30 &

# 场景 3: 只发 1 包做 smoke test
python simulator/sensor_simulator.py --once --host 127.0.0.1 --port 8080

# 场景 4: 快速老化 10000 发后看参数衰减 (wear=0.001)
python simulator/sensor_simulator.py -w 0.001 -n 10000 -i 0.1 --log /tmp/sim.log
```

### 全部参数

| 参数 | 环境变量 | 默认 | 说明 |
|------|---------|------|------|
| `--host` | `SIM_HOST` | `127.0.0.1` | UDP 目标主机 |
| `--port` | `SIM_PORT` | `8080` | UDP 目标端口 |
| `--device` | `SIM_DEVICE` | `chuangnu-001` | 设备 ID (用于告警关联) |
| `-i / --interval` | `SIM_INTERVAL` | `60` | 上报间隔（秒） |
| `-l / --level` | `SIM_LEVEL` | `mid` | 拉力档位 (light/mid/high/extreme) |
| `-a / --arrow` | `SIM_ARROW` | `bodkin` | 箭镞类型 |
| `-w / --wear` | `SIM_WEAR` | `0.0005` | 每发射老化系数 (1e-5 ~ 5e-3) |
| `--temp` | `SIM_TEMP` | `20.0` | 环境温度 °C |
| `--humid` | `SIM_HUMID` | `50.0` | 环境湿度 % |
| `--seed` | `SIM_SEED` | `0` | 随机种子 (0=不固定) |
| `--log` | `SIM_LOG` | — | 将每次 UDP 发送的 JSON 逐行写入文件 |
| `--once` | — | — | 只发 1 次 |
| `-n / --count` | `SIM_COUNT` | `0` | 发送 N 次后退出，0=无限 |

---

## 📊 Prometheus 指标清单

抓取目标：`ballistics-server:9090/metrics`（推荐 scrape 15s）

### 核心 Counter / Gauge

| 指标名 | 类型 | 标签 | 含义 |
|--------|------|------|------|
| `ballistics_up` | Gauge | — | 服务健康，恒 1 |
| `ballistics_start_time_seconds` | GaugeFunc | — | 启动时间戳 (Unix s) |
| `ballistics_udp_packets_total` | Counter | — | UDP 接收总包数 |
| `ballistics_udp_packets_invalid_total` | Counter | — | 校验失败被丢弃包数 |
| `ballistics_sim_runs_total` | Counter | — | 完成的弹道仿真数 |
| `ballistics_pen_runs_total` | Counter | — | 完成的穿甲计算数 |
| `ballistics_alert_total` | Counter | `level`, `type` | 告警总数 (按级别和类型分) |
| `ballistics_db_inserts_total` | CounterVec | `table` | ClickHouse INSERT 总数 |
| `ballistics_db_insert_errors_total` | CounterVec | `table` | ClickHouse INSERT 失败数 |
| `ballistics_mqtt_messages_total` | Counter | — | MQTT 成功发布数 |
| `ballistics_mqtt_reconnects_total` | Counter | — | MQTT 断线重连次数 |

### 活跃态 Gauge（观测积压）

| 指标名 | 含义 |
|--------|------|
| `ballistics_sim_active` | 正在跑的仿真任务数 |
| `ballistics_pen_active` | 正在跑的穿甲任务数 |
| `ballistics_queue_sensor_pending` | UDP 校验后队列积压 |
| `ballistics_queue_sim_pending` | 仿真作业队列积压 |
| `ballistics_queue_pen_pending` | 穿甲作业队列积压 |

### Histogram（P50/P95/P99）

| 指标名 | 桶类型 | 单位 |
|--------|--------|------|
| `ballistics_sim_duration_seconds` | Exponential, 12 桶 | 仿真耗时 |
| `ballistics_pen_duration_seconds` | Exponential, 10 桶 | 穿甲耗时 |
| `ballistics_udp_packet_size_bytes` | Exponential, 10 桶 | UDP 帧大小 |
| `ballistics_sim_impact_velocity_ms` | Linear 20-180 | 命中速度 m/s |
| `ballistics_pen_depth_mm` | Exponential, 10 桶 | 穿深 mm |

---

## 🔍 pprof 性能剖析

服务已自带 `net/http/pprof`，直接在浏览器/命令行访问：

```bash
# 30s CPU 采样
go tool pprof -http=:6060 http://<server-ip>:9090/debug/pprof/profile?seconds=30

# 堆内存
go tool pprof http://<server-ip>:9090/debug/pprof/heap

# 协程栈
curl http://<server-ip>:9090/debug/pprof/goroutine?debug=2
```

> 生产部署时 `/debug/pprof/*` 已被 Nginx ACL 限内网访问。

---

## 🗃️ ClickHouse 数据分层 (降采样 + TTL)

| 表 / MV | 粒度 | 引擎 | TTL | 典型场景 |
|---------|------|------|-----|---------|
| `sensor_data` | 原始 (ms) | MergeTree | **1 年** | 实时大屏 / 问题排查 |
| `sensor_data_stats_mv` | 小时 | SummingMergeTree | 继承 | 24h 趋势图 |
| `sensor_data_daily_ds` | **日** | AggregatingMergeTree | **3 年** | 年度报告 / 考古统计 |
| `simulation_results` | 原始 | MergeTree | **1 年** | 每次发射明细 |
| `sim_daily_ds` | **日** | AggregatingMergeTree | **3 年** | 射程 / KE 长期趋势 |
| `armor_performance` | 原始 | MergeTree | **1 年** | 铠甲试验明细 |
| `armor_perf_daily_ds` | **日** | AggregatingMergeTree | **3 年** | 铠甲对比月报 |
| `bow_release_energy` | 原始 | MergeTree | **1 年** | 能量守恒审计 |
| `alerts` | 原始 | MergeTree | **1 年** | 告警审计 |

**查询建议**：跨 1 个月以上报表一律查 `_daily_ds` 表，扫描量是原始表的 1/1000 量级。

---

## 🧩 部署模式对比

| 模式 | 适用 | 需要 |
|------|------|------|
| **开发单机** (`docker compose up -d`) | 本地调试、全栈打通 | Docker ≥ 24 |
| **生产** | 外部 ClickHouse 集群 + 独立 MQTT | 把 `CLICKHOUSE_DSN` 指向集群，关 compose 里的 `clickhouse` 服务 |
| **Kubernetes** | 多副本 + HPA | 用 Helm，把 `docker-compose.yml` 映射为 Deployment + Service + ConfigMap |

---

## 🛠️ 目录结构

```
.
├── Dockerfile.backend         # Go 多阶段构建 (golang-alpine → upx → alpine runtime)
├── Dockerfile.simulator       # Python 模拟器 (3.11-slim + tini)
├── docker-compose.yml         # 5 服务编排
├── .env.template              # 环境变量模板
├── .dockerignore
├── README.md
├── backend/
│   ├── main.go                # pipeline 编排器 + 指标注入
│   ├── go.mod
│   ├── metrics/metrics.go     # Prometheus + pprof HTTP 服务
│   ├── udp_receiver/          # 采集 + 校验 (可注入指标)
│   ├── ballistic_simulator/   # 多体动力学 + 外弹道
│   ├── penetration_analyzer/  # 陀螺修正 + Thompson 侵彻
│   ├── alarm_mqtt/            # 告警评估 + MQTT 推送
│   ├── api/                   # Gin HTTP + CORS
│   ├── clickhouse/            # ClickHouse 驱动 + 指标埋点
│   ├── config/                # 环境变量 + JSON config loader
│   └── models/                # 数据模型 + Channel Job/Result 类型
├── config/
│   ├── dynamics_params.json   # 弩臂几何/仿真/默认/气动 (外置)
│   └── armor_params.json      # 三种铠甲 / 三种箭镞 / 陀螺阈值 (外置)
├── clickhouse/
│   ├── init.sql               # 建表 + 4 降采样 MV + TTL
│   └── config/config.xml      # MergeTree 调优 + 压缩策略
├── frontend/
│   ├── index.html
│   ├── bed_crossbow_3d.js     # Three.js 3D 模型 + GPU 弓弦蒙皮
│   ├── power_panel.js         # 数据面板 + 2D 轨迹
│   └── app.js                 # 3 行入口
├── nginx/default.conf         # 前端 Gzip + /api 反代 + /metrics ACL
└── simulator/sensor_simulator.py  # 4 档拉力 / 4 箭镞 / 可调老化
```

---

## ✅ 功能回归清单

部署完成后按以下 checklist 验证：

- [ ] **Nginx 健康**：`curl -i http://localhost/healthz` → `200 nginx-ok`
- [ ] **服务健康**：`curl -i http://localhost/api/v1/health` → `status:ok`
- [ ] **Go pprof**：容器内 `wget -qO- http://127.0.0.1:9090/debug/pprof/` 返回 HTML
- [ ] **ClickHouse 初始化**：`clickhouse-client --port 9000 -q "SHOW TABLES FROM ballistics"` → 出现 10 张表+4 MV
- [ ] **MQTT 连通**：`mosquitto_sub -h localhost -p 1883 -t 'ballistics/#' -v`
- [ ] **模拟器发送**：`docker compose logs -f bs-sim` 出现 `T=... δ=... v₀=...` 行
- [ ] **数据入库**：CH 执行 `SELECT count() FROM ballistics.sensor_data` 逐步增长
- [ ] **仿真 & 穿甲**：前端点 "🏹 模拟发射" → 右侧数据更新 + 3D 箭矢动画
- [ ] **告警触发**：把 `.env` 中 `SIM_LEVEL=extreme` 重启 sim，10 发内应有 `arm_crack_risk` MQTT 告警
- [ ] **Prometheus 指标**：`curl -s http://localhost:9090/metrics | grep ballistics_` 出现全部指标家族
- [ ] **前端 JS Gzip**：Chrome DevTools → Network → `app.js` Response Headers 含 `Content-Encoding: gzip`

---

## 📝 License

内部军事史研究用途，动力学参数引用宋代《武经总要》复原研究。
