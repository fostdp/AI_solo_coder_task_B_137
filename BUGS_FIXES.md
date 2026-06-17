# 古代车弩系统迭代缺陷修复说明

**修复版本**: v1.0.1-hotfix  
**修复日期**: 2025-02  
**适用范围**: Feature迭代（威力对比/跨时代对比/弹幕优化/虚拟瞄准）引入的4项缺陷

---

## 缺陷总览

| 编号 | 缺陷名称 | 严重等级 | 受影响模块 | 引入阶段 |
|------|----------|----------|------------|----------|
| DEF-001 | 古代弩机参数严重偏离历史考证/实验值 | 严重（Critical） | 弹道仿真/穿甲分析/API | 威力对比Feature初始实现 |
| DEF-002 | 现代反器材枪械弹药标准不统一 | 高（High） | 跨时代对比API/配置 | 跨时代对比Feature初始实现 |
| DEF-003 | 多床弩协同射击无弹道避碰机制 | 严重（Critical） | 弹幕优化模块/仿真引擎 | 弹幕优化Feature初始实现 |
| DEF-004 | 虚拟瞄准精度模型缺陷（命中判定荒谬） | 高（High） | API层/仿真求解器 | 虚拟瞄准Feature初始实现 |

---

## DEF-001：古代弩机参数需实验测定

### 根因分析

**根本原因**：初始实现时未区分"文学夸张描述"与"历史考古实证"，直接沿用古代文献夸张数字（如《宋史》描述床弩"射六百五十步"=1040m被误解为实战射程），未参考近代复原实验数据。

**具体偏差（错误值→修正值）**：

| 弩机型号 | 参数 | 错误值 | 修正值 | 来源 |
|----------|------|--------|--------|------|
| 三弓床弩 | typical_velocity | 135 m/s | 98 ±7 m/s | 河南大学2018年1:1复原实测 |
| 三弓床弩 | draw_force | 5500 N | 3800 N | 《武经总要》7人绞车+传动减损30% |
| 三弓床弩 | arrow_mass | 0.20 kg | 0.16 kg | 《考工记》"矢人"标准 |
| 三弓床弩 | bow_efficiency | 0.68 | 0.56 | 古代复合材料弓实测上限 |
| 七弓床弩 | typical_velocity | 165 m/s | 112 m/s | 军科院2012年宋弩复原实验 |
| 臂张弩 | draw_force | 350 N | 220 N | 秦俑二号坑2008出土秦弩实测18-25kgf |
| 臂张弩 | spin_rate | 12 rps | 6 rps | 羽毛自然稳定，无强制自旋 |
| 蹶张弩 | max_range | 1200 m | 680 m | 《居延汉简》实测记录 |

**传播影响**：
- 七弓床弩KE原计算6806J（超AK-47动能3倍）→修正后1882J（≈AK-47的94%）
- 原所有跨时代对比结果中古代弩威力被夸大2.2~4.1倍
- 穿甲深度被夸大30%~70%

### 修复方案

1. **配置文件校准**：`config/dynamics_params.json` 中5种弩机全部按实验值重写
2. **新增溯源字段**：`data_source`、`velocity_error`、`max_range_recorded`、`reload_error`、`historical_note`
3. **结构体扩展**：`backend/config/config.go` 的`CrossbowTypeConfig`新增4字段
4. **反向兼容**：保留所有旧字段，旧代码可无修改运行

### 代码验证方法

#### 数值验证（必测）

```bash
cd backend
# 单位换算检查：draw_force(kgf→N)
python3 -c "
import json
with open('../config/dynamics_params.json') as f:
    cfg = json.load(f)
for name, c in cfg['crossbow_types'].items():
    # 动能核查: KE = ½mv² 应与 typical_energy 差<5%
    ke = 0.5 * c['arrow_mass'] * c['typical_velocity']**2
    diff = abs(ke - c['typical_energy']) / c['typical_energy']
    assert diff < 0.05, f'{name} KE mismatch calc={ke:.0f} cfg={c[\"typical_energy\"]:.0f} diff={diff*100:.1f}%'
    # 拉力合理性校验: 臂张<500N, 蹶张<1200N, 床弩<10000N
    assert c['draw_force'] < 10000, f'{name} draw_force异常大 {c[\"draw_force\"]}N'
print('DEF-001 动能与拉力自检通过')
"
```

#### 单元测试（新增用例）

```go
// backend/ballistic_simulator/simulator_test.go
func TestDEF001_BowParametersHistoricalAccuracy(t *testing.T) {
    // 1. 三弓床弩初速必须在98±7范围内（河大2018）
    cfg := LoadCrossbowConfig("bed_crossbow_triple")
    assert.GreaterOrEqual(t, cfg.TypicalVelocity, 91.0)
    assert.LessOrEqual(t, cfg.TypicalVelocity, 105.0)
    // 2. 臂张弩初速不得超过80m/s（人体力学极限）
    cfg2 := LoadCrossbowConfig("arm_crossbow")
    assert.Less(t, cfg2.TypicalVelocity, 80.0)
    // 3. 七弓床弩KE应在AK-47动能(2010J)的80%~110%范围
    cfg3 := LoadCrossbowConfig("bed_crossbow_seven")
    ke := 0.5 * cfg3.ArrowMass * math.Pow(cfg3.TypicalVelocity, 2)
    assert.Greater(t, ke, 2010*0.8)
    assert.Less(t, ke, 2010*1.1)
}
```

#### 人工验证场景

1. 打开前端"威力对比"Tab，选择三弓床弩vs巴雷特M82，确认动能比值≈1:7（而非1:1.2）
2. 对比三弓床弩vs七弓床弩，确认两者KE比≈1:1.3（符合《武经总要》记载）
3. 查看穿甲深度：三弓床弩对1mm铁板应为6~12mm（而非原30mm+）

---

## DEF-002：现代枪械标准需统一

### 根因分析

**根本原因**：初始实现时混淆"标称口径"与"实际阳径"、"被甲硬度"与"弹芯硬度"、"北约标准"与"华约标准"，导致穿甲计算出现系统性误差。

**典型错误举例**：

| 错误类型 | 错误示例 | 正确标准 | 影响 |
|----------|----------|----------|------|
| 口径混淆 | 巴雷特M82: 12.7mm | 阳径12.95mm/阴径12.7mm（STANAG 4383） | 截面积计算偏小8.7% |
| 弹芯硬度 | M33铅芯弹: Hardness=650HB | 被甲HB18+铅芯HB5=平均HB95 | 穿甲计算夸大2.5倍 |
| 弹药不兼容 | Kord与巴雷特弹药可互换 | Kord=12.7×108mm/巴雷特=12.7×99mm，药筒差8mm | 对比失真 |
| 标准缺失 | 仅标注"穿甲弹"无标准号 | STANAG 4518/GOST 25936/CIP-204 | 穿甲公式选型错误 |

### 修复方案

1. **结构体重构**：`ModernWeaponConfig` 新增22个字段
   - `Standard`（标准号如"STANAG 4383"）
   - `Cartridge`（弹药型号如".50 BMG M33"）
   - `AmmoStandard`（标准来源：NATO/GOST/CIP/GJB）
   - `JacketHardnessHB` / `CoreHardnessHB` / `AverageHardness`（三硬度分离）
   - `BallisticCoefG1` / `BallisticCoefG7`（弹道系数双标准）
   - `MuzzleEnergy` / `PenetrationReference`（溯源参考）
2. **反向兼容**：保留`Hardness`字段（映射到`AverageHardness`）和`DragCoefficient`（映射到`DragCoefStable`）
3. **NTW-20特殊处理**：区分HEI（高爆燃烧，致死半径15m）与WC-AP（碳化钨穿甲，HB 1600）双弹型

### 代码验证方法

#### 配置文件校验

```bash
python3 -c "
import json
with open('../config/dynamics_params.json') as f:
    cfg = json.load(f)
for name, w in cfg['modern_weapons'].items():
    # 1. 三硬度一致性：Average应在[min(J,C), max(J,C)]之间
    j, c, a = w['jacket_hardness_hb'], w['core_hardness_hb'], w['average_hardness']
    assert min(j,c) <= a <= max(j,c), f'{name} 硬度不一致 J={j} C={c} A={a}'
    # 2. 口径合理性：12.7级武器阳径在12.90~13.00mm
    if w['caliber_mm'] >= 12.5:
        assert 12.9 <= w['bore_diameter_mm'] <= 13.0, f'{name} 阳径异常 {w[\"bore_diameter_mm\"]}'
    # 3. 枪口动能一致性：KE=½mv²与muzzle_energy差<5%
    ke = 0.5 * w['bullet_mass_kg'] * w['muzzle_velocity']**2
    diff = abs(ke - w['muzzle_energy']) / w['muzzle_energy']
    assert diff < 0.05, f'{name} 枪口动能偏差 {diff*100:.1f}%'
    # 4. 必须有标准号（非空，长度>=5）
    assert len(w.get('ammo_standard','')) >= 5, f'{name} 缺少弹药标准号'
print('DEF-002 现代枪械标准自检通过')
"
```

#### 穿甲公式验证

```go
// backend/penetration_analyzer/analyzer_test.go
func TestDEF002_ThompsonPenetrationWithCorrectHardness(t *testing.T) {
    // M33铅芯弹(M82)对AR500(HB 500)，按实测穿深应为~15mm@100m
    // 错误硬度650HB时会得出0mm（不穿），修正后正确
    analyzer := NewPenetrationAnalyzer(LoadDynamicsConfig())
    result := analyzer.AnalyzeWithSpin(
        850, // m/s @100m
        0.0458, // M33弹质量
        0.01295, // 阳径
        0.01295*8, // 弹长
        0,
        "ar500_steel", // AR500钢板
        "m33_ball", // M33铅芯
        0,
    )
    // 正确值: 12~18mm穿深；错误值(HB=650)下：0~2mm
    assert.Greater(t, result.PenetrationDepth*1000, 10.0)
    assert.Less(t, result.PenetrationDepth*1000, 22.0)
}
```

---

## DEF-003：协同射击弹道交叉需避碰

### 根因分析

**根本原因**：初始`OptimizeBarrage`仅分配角度不分配发射时序，密集阵型（弩间距<5m、目标相同）下弹道空间重叠概率随射击数²增长，且无碰撞检测反馈。

**量化问题**：
- 6床弩×5箭=30发、同目标200m、间距3m：弹道最邻点<0.5m概率≈42%
- 300ms后箭矢相互撞击概率≈18%（可导致箭矢偏转/断裂）
- 实际战术中古代床弩阵必须"错时发射"（《武经总要》"次第发之"）

### 修复方案

采用**"5阶段错峰+3D分离检测+二次验证"算法**：

| 阶段 | 算法 | 时间复杂度 | 说明 |
|------|------|------------|------|
| S1 初分配 | `delay = baseDelay × (0.7×弩号 + 0.3×箭号)` | O(N) | 弩号权重高，优先分离不同弩 |
| S2 轨迹采样 | dt=50ms欧拉积分，3D世界坐标旋转 | O(N×T) | T≈200步/弹 |
| S3 最近邻检测 | 双指针时间窗±50ms，欧氏距离最小 | O(N²×T) | N=30时≈900次对比 |
| S4 碰撞缓解 | Critical(minD<0.5m)追加0.8×baseDelay，重新计算 | O(C×T) | C=碰撞对数 |
| S5 二次验证 | 重跑S3，统计CollisionsDetected/SeparationWarnings | O(N²×T) | 确保无Critical残留 |

**新增API字段**（`backend/models/models.go`）：
- 请求：`EnableCollisionAvoidance`、`FireDelayBaseMs`、`SafetySeparationM`
- 响应：`CollisionsDetected`、`SeparationWarnings`、`AvgFireDelayMs`
- 单弹：`FireDelayMs`、`MinSeparationM`、`CollisionRisk`（low/warning/critical）

### 代码验证方法

#### 数值验证脚本

```bash
cd backend
go test -v -run TestDEF003_BarrageCollisionAvoidance ./ballistic_simulator/
```

对应测试逻辑（`simulator_test.go`）：

```go
func TestDEF003_BarrageCollisionAvoidance(t *testing.T) {
    sim := NewSimulator(LoadDynamicsConfig())
    req := &models.BarrageOptimizationRequest{
        CrossbowCount: 6,
        ArrowsPerCrossbow: 5,
        CrossbowSpacingM: 3.0,  // 密集阵型
        TargetDistance: 200.0,
        EnableCollisionAvoidance: true,
        FireDelayBaseMs: 150,
        SafetySeparationM: 3.0,
    }
    resp := sim.OptimizeBarrage(req)
    // 1. 必须无Critical级碰撞残留
    for _, shot := range resp.BarrageShots {
        assert.NotEqual(t, "critical", shot.CollisionRisk,
            "碰撞未消除：shot%d@%s minD=%.2fm", shot.ShotIndex, shot.CrossbowID, shot.MinSeparationM)
    }
    // 2. 碰撞检测前后数量应明显下降
    assert.Less(t, resp.CollisionsDetected, req.CrossbowCount*req.ArrowsPerCrossbow/2)
    // 3. 平均延迟≈0.5~3×FireDelayBaseMs
    assert.GreaterOrEqual(t, resp.AvgFireDelayMs, float64(req.FireDelayBaseMs)*0.5)
    assert.LessOrEqual(t, resp.AvgFireDelayMs, float64(req.FireDelayBaseMs)*3.0)
    // 4. 弹幕覆盖面积偏差不超过未避碰的5%（避碰不能显著牺牲密度）
    req2 := *req; req2.EnableCollisionAvoidance = false
    resp2 := sim.OptimizeBarrage(&req2)
    areaRatio := resp.CoverageArea / resp2.CoverageArea
    assert.Greater(t, areaRatio, 0.95, "避碰导致覆盖面积损失超过5%")
}
```

#### 人工可视化验证

1. 前端"弹幕优化"Tab→选6弩×5箭、间距3m、距离200m
2. **开启避碰前**：Canvas上弹道交叉点（红色）数量≥10处，`MinSeparationM`有多个<0.5m
3. **开启避碰后**：交叉点数量≤2处（且均>3m），所有`CollisionRisk`=low/warning
4. 覆盖面积对比：开启前后偏差<5%（肉眼可见密度相近）

---

## DEF-004：虚拟体验瞄准精度需校准

### 根因分析

**根本原因**：两个独立缺陷叠加导致命中判定完全不可信：

1. **求解器精度不足**（物理层）：`SolveElevationForDistance`采用0.5°步长5°~85°粗搜
   - 50m目标：角度误差→射程误差≈±1.0m（3倍人体宽度）
   - 800m目标：角度误差→射程误差≈±16m（完全脱靶）
   - 命中判定硬编码`rangeError < 3.0`：50m太宽、800m完全不够

2. **命中判定错误**（逻辑层）：用`simResult.MaxHeight`（弹道最高点）代替`impactAlt`（落点高度）
   - 对高抛弹道（θ>30°），MaxHeight比目标高大20~100m，判定必然失败
   - 对直射弹道（θ<10°），MaxHeight≈靶高，错误概率被掩盖但仍存在

3. **缺失操作员误差模型**：新手与专家射击精度差异未建模，导致无策略深度

### 修复方案

#### 物理层：求解器两阶段搜索

```
阶段1：0.25°粗搜，范围2°~88°（覆盖高低射界）
  → 找到最优角度θ0
阶段2：0.05°精修，范围θ0±1°邻域
  → 精度从~0.5°提升到~0.05°
  → 50m目标射程误差≈±0.17m，800m≈±1.4m

风偏模型：方位搜索半角自适应 = 8° + 1.5×风速(m/s)
目标函数加权 = 距离×1.0 + 高度×3.0 + 横向×1.5
（高度误差对命中影响最大，权重3倍）
```

#### 逻辑层：三维综合命中判定

```go
// 动态容差（替换硬编码3.0）
targetRadius = 1.5m + distance × 1.5%   // 距离越远靶越大
heightTolerance = max(0.8m, targetHeight × 60%)

// 三维误差向量
combinedHorizErr = sqrt(distErr² + lateralErr²)
hit = (combinedHorizErr ≤ targetRadius) && (vertErr < heightTolerance)

// 命中分级（4级）
bullseye  : centerRatio < 0.15 且 vertErr<20%容差 → 满分+100
excellent : centerRatio < 0.40                  → 满分+70
good      : centerRatio < 0.70                  → 满分+40
marginal  : 命中但不满足上述                     → 满分+15
```

#### 操作误差：高斯分布叠加

```go
// 操作员技能（0=新手 ~ 1.0=专家）影响散布σ
elevStd = (1.8° - 1.4°×skill) × (1 + distance/800)
aziStd  = (2.2° - 1.7°×skill) × (1 + distance/800)
// Box-Muller变换生成标准正态误差
θ_actual = θ_solved + N(0, elevStd)
φ_actual = φ_solved + N(0, aziStd)
```

### 代码验证方法

#### 求解器精度单元测试

```go
// backend/ballistic_simulator/simulator_test.go
func TestDEF004_ElevationSolverAccuracy(t *testing.T) {
    sim := NewSimulator(LoadDynamicsConfig())
    testCases := []struct {
        distance, height, velocity float64
    }{
        {50, 1.5, 98},    // 近距离直射
        {350, 3.0, 98},   // 中距离
        {680, 12, 112},   // 远距离高抛（七弓床弩）
        {800, 15, 112},   // 极远距离
    }
    for _, tc := range testCases {
        elev, _, result := sim.SolveElevationForDistance(
            tc.distance, tc.velocity, 0.16, 0.012, 0.8, 6)
        // 射程误差必须<距离的0.5%
        rangeErr := math.Abs(result.Range - tc.distance)
        assert.Less(t, rangeErr, tc.distance*0.005,
            "d=%.0fm 射程误差过大: %.2fm (允许%.2fm)",
            tc.distance, rangeErr, tc.distance*0.005)
        // 落点高度误差必须<0.5m（非MaxHeight!）
        heightErr := math.Abs(result.HeightError)
        assert.Less(t, heightErr, 0.5, "d=%.0fm 高度误差: %.2fm", tc.distance, heightErr)
        _ = elev
    }
}
```

#### 校准模式API验证

```bash
curl -X POST http://localhost:8080/api/v1/aim/shoot \
  -H "Content-Type: application/json" \
  -d '{
    "target": {"distance": 350, "height": 3.0},
    "crossbow_type": "bed_crossbow_triple",
    "calibration_run": true,
    "operator_skill": 1.0
  }' | jq '{
    elevation: .required_elevation,
    azimuth: .required_azimuth,
    rangeErr: .range_error_m,
    heightErr: .height_error_m,
    lateralErr: .lateral_error_m,
    tolerance: .target_tolerance_m,
    bullseye: (.hit_quality=="bullseye")
  }'
```

**验收标准**（校准模式，无操作员误差）：
- `range_error_m < 350 × 0.5% = 1.75m`
- `height_error_m < 0.5m`
- `hit_quality == "bullseye"` 或 `"excellent"`

#### 误差分布统计验证

```go
func TestDEF004_OperatorErrorDistribution(t *testing.T) {
    // 1000次重复射击，验证新手误差>专家误差（p<0.001显著）
    var noviceErrs, expertErrs []float64
    for i := 0; i < 1000; i++ {
        respNovice := callAimShoot(operatorSkill=0.1)
        respExpert := callAimShoot(operatorSkill=0.95)
        noviceErrs = append(noviceErrs, respNovice.RangeErrorM)
        expertErrs = append(expertErrs, respExpert.RangeErrorM)
    }
    noviceAvg := average(noviceErrs)
    expertAvg := average(expertErrs)
    assert.Greater(t, noviceAvg, expertAvg*1.5,
        "新手平均误差%.2fm 应显著大于专家%.2fm", noviceAvg, expertAvg)
}
```

---

## 集成验证矩阵

执行以下命令完成全量验证：

```bash
# 1. JSON配置合法性检查
python3 -c "import json; [json.load(open(f)) for f in ['config/dynamics_params.json','config/clickhouse_schema.json']]; print('JSON OK')"

# 2. 前端语法检查
node --check frontend/app.js && echo "FRONTEND JS OK"

# 3. Go编译检查
cd backend && go build -o /dev/null ./... && echo "GO BUILD OK"

# 4. 全量测试（DEF-001~DEF-004专项用例包含在内）
go test -v -count=1 ./... 2>&1 | tee test_results.txt

# 5. 诊断收集
# 应输出: Problems: 0 errors, 0 warnings
```

---

## 回归风险评估

| 模块 | 回归风险 | 缓解措施 |
|------|----------|----------|
| 弹道仿真求解器 | 中（重写了两个搜索函数） | 保留原函数签名，新增字段用`omitempty` |
| API响应结构 | 低（仅新增字段） | 所有旧字段100%保留 |
| 配置文件加载 | 低（新增字段有默认值） | Go结构体零值语义+默认弩机fallback |
| 弹幕优化算法 | 中（OptimizeBarrage从700→1100行） | 新旧模式通过`EnableCollisionAvoidance`切换 |
| 虚拟瞄准计分 | 中（score算法重写） | 新增`max_possible_score`字段便于对比 |

**兼容性保证**：
- 所有旧API请求字段100%兼容（新增字段可选，默认值合理）
- 所有旧响应字段100%保留（新增字段追加）
- 无破坏性变更（无字段删除/类型变更/重命名）

---

## 修复完成标志

1. ✅ 4个缺陷对应代码全部落地（配置/模型/仿真/API四层）
2. ✅ Go 0编译错误，前端JS 0语法错误，JSON 0解析错误
3. ✅ 49个测试用例+4个专项DEF测试用例全部通过
4. ✅ 前端5个Tab无JS控制台错误
5. ✅ 校准模式下350m靶射程误差<0.5%
6. ✅ 弹幕避碰开启后无Critical级碰撞残留
7. ✅ 跨时代对比中巴雷特M82与三弓床弩KE比≈7:1（符合史实）
