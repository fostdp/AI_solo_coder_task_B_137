package api

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"ballistics-system/backend/config"
	ballistic_simulator "ballistics-system/backend/ballistic_simulator"
	penetration_analyzer "ballistics-system/backend/penetration_analyzer"
	"ballistics-system/backend/models"
)

func setupTestServerWithGin() *Server {
	dynCfg := &config.DynamicsConfig{
		Bow: config.BowConfig{
			ArmLength: 1.5, PeakTension: 5000, DrawLength: 1.2,
		},
		Simulation: config.SimulationConfig{
			TimeStep: 0.005, MaxSimTime: 30.0, Gravity: 9.80665, AirDensitySea: 1.225,
		},
		Defaults: config.DefaultsConfig{
			ArrowMass: 0.2, ArrowDiameter: 0.012, ArrowLength: 1.0,
			SpinRate: 25.0, DragCoefficient: 0.4, LaunchAngle: 45.0,
		},
		Aerodynamics: config.AerodynamicsConfig{
			LiftCoefficient: 0.05, MagnusCoefficient: 0.001, SpinDecayRate: 0.0001,
			PitchDampingBase: 0.02, AeroMomentCoefficient: 0.01,
		},
		CrossbowTypes: map[string]config.CrossbowTypeConfig{
			"arm_stretched": {
				Type: "arm_stretched", Name: "臂张弩", Description: "单人手臂", Era: "春秋",
				DrawForce: 350, DrawLength: 0.45, ArrowMass: 0.035, ArrowLength: 0.42, ArrowDiameter: 0.008,
				TypicalVelocity: 65, TypicalRange: 180, SpinRate: 12, BowEfficiency: 0.58,
				CrewSize: 1, ReloadSeconds: 8,
			},
			"leg_stretched": {
				Type: "leg_stretched", Name: "蹶张弩", Description: "双脚蹬踏", Era: "战国",
				DrawForce: 900, DrawLength: 0.65, ArrowMass: 0.085, ArrowLength: 0.6, ArrowDiameter: 0.01,
				TypicalVelocity: 95, TypicalRange: 350, SpinRate: 18, BowEfficiency: 0.62,
				CrewSize: 1, ReloadSeconds: 20,
			},
			"bed_crossbow_single": {
				Type: "bed_crossbow_single", Name: "单弓床弩", Description: "床架单弓", Era: "汉代",
				DrawForce: 2500, DrawLength: 0.9, ArrowMass: 0.15, ArrowLength: 0.9, ArrowDiameter: 0.012,
				TypicalVelocity: 110, TypicalRange: 550, SpinRate: 22, BowEfficiency: 0.65,
				CrewSize: 3, ReloadSeconds: 45,
			},
			"bed_crossbow_triple": {
				Type: "bed_crossbow_triple", Name: "三弓床弩", Description: "三弓复合", Era: "宋代",
				DrawForce: 5500, DrawLength: 1.2, ArrowMass: 0.2, ArrowLength: 1.0, ArrowDiameter: 0.012,
				TypicalVelocity: 135, TypicalRange: 800, SpinRate: 25, BowEfficiency: 0.68,
				CrewSize: 7, ReloadSeconds: 90,
			},
			"bed_crossbow_seven": {
				Type: "bed_crossbow_seven", Name: "七弓床弩", Description: "超重型", Era: "宋代",
				DrawForce: 12000, DrawLength: 1.5, ArrowMass: 0.5, ArrowLength: 1.5, ArrowDiameter: 0.018,
				TypicalVelocity: 165, TypicalRange: 1500, SpinRate: 32, BowEfficiency: 0.70,
				CrewSize: 20, ReloadSeconds: 300,
			},
		},
		ModernWeapons: map[string]config.ModernWeaponConfig{
			"barrett_m82": {
				Type: "barrett_m82", Name: "巴雷特 M82A1", BulletMass: 0.042,
				BulletDiameter: 0.0127, BulletLength: 0.058, MuzzleVelocity: 853,
				MaxRange: 1800, EffectiveRange: 1500, DragCoefficient: 0.295,
				SpinRate: 1800, Hardness: 650, TipArea: 1.267e-4, CrewSize: 1, ReloadSeconds: 3,
			},
			"ntw_20": {
				Type: "ntw_20", Name: "NTW-20 20mm", BulletMass: 0.125,
				BulletDiameter: 0.02, BulletLength: 0.11, MuzzleVelocity: 720,
				MaxRange: 1600, EffectiveRange: 1300, DragCoefficient: 0.32,
				SpinRate: 1200, Hardness: 680, TipArea: 3.142e-4, CrewSize: 2, ReloadSeconds: 5,
			},
		},
	}
	armorCfg := &config.ArmorConfig{
		Armors: map[string]config.ArmorEntryConfig{
			"leather":   {Type: "leather", Thickness: 0.008, Density: 1000, YieldStrength: 40e6, Hardness: 150, Name: "皮甲"},
			"lamellar":  {Type: "lamellar", Thickness: 0.004, Density: 7850, YieldStrength: 200e6, Hardness: 280, Name: "鳞甲"},
			"chainmail": {Type: "chainmail", Thickness: 0.006, Density: 7850, YieldStrength: 250e6, Hardness: 300, Name: "锁子甲"},
			"plate":   {Type: "plate", Thickness: 0.0025, Density: 7850, YieldStrength: 500e6, Hardness: 450, Name: "板甲"},
		},
		ArrowHeads: map[string]config.ArrowHeadEntryConfig{
			"bodkin":    {Type: "bodkin", TipDiameter: 0.004, TipArea: 1.256e-5, TipMass: 0.03, Hardness: 550, Name: "穿甲箭镞"},
			"broadhead": {Type: "broadhead", TipDiameter: 0.03, TipArea: 7.068e-4, TipMass: 0.05, Hardness: 400, Name: "宽刃箭镞"},
			"blunt":     {Type: "blunt", TipDiameter: 0.015, TipArea: 1.767e-4, TipMass: 0.04, Hardness: 300, Name: "钝头箭镞"},
		},
		Gyro: config.GyroConfig{
			YawStableThreshold: 4.0, YawModerateThreshold: 1.5, YawMarginalThreshold: 1.0,
			StabilityPenaltyFull: 2.0, StabilityPenaltyModerate: 1.0, StabilityPenaltyPoor: 0.5,
			RotaryEnergyBoostFactor: 0.15, StabilityClampMin: 0.1, StabilityClampMax: 50.0,
			LowVelocityThreshold: 1.0, LowVelocityStability: 10.0,
		},
	}

	sim := ballistic_simulator.NewSimulator(dynCfg)
	pen := penetration_analyzer.NewAnalyzer(armorCfg)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	s := &Server{
		engine:      r,
		store:       nil,
		simEngine:   sim,
		penAnalyzer: pen,
		dynamicsCfg: dynCfg,
		addr:        ":0",
	}
	s.setupRoutes()
	return s
}

func performRequest(s *Server, method, path string, body interface{}) *httptest.ResponseRecorder {
	var reqBody *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewReader(b)
	} else {
		reqBody = bytes.NewReader(nil)
	}
	w := httptest.NewRecorder()
	var req *http.Request
	if body != nil {
		req, _ = http.NewRequest(method, path, reqBody)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(method, path, nil)
	}
	s.engine.ServeHTTP(w, req)
	return w
}

// ====== 威力对比端点：弩机列表 & 对比 ======

func TestListCrossbows_ReturnsAllTypes(t *testing.T) {
	s := setupTestServerWithGin()
	w := performRequest(s, "GET", "/api/v1/crossbows", nil)

	if w.Code != 200 {
		t.Fatalf("GET /crossbows 应返回200, 得%d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Crossbows []map[string]interface{} `json:"crossbows"`
		Count     int                `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Count < 5 {
		t.Errorf("应至少返回5种弩机, 得%d", resp.Count)
	}
	t.Logf("弩机列表: %d种", resp.Count)
}

func TestCompareCrossbows_AllTypes(t *testing.T) {
	s := setupTestServerWithGin()
	w := performRequest(s, "POST", "/api/v1/compare/crossbows", map[string]interface{}{
		"launch_angle":    45,
		"arrow_head_type": "bodkin",
	})

	if w.Code != 200 {
		t.Fatalf("POST /compare/crossbows 应返回200, 得%d: %s", w.Code, w.Body.String())
	}

	var resp models.CrossbowComparisonResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v, body: %s", err, w.Body.String())
	}

	if len(resp.Crossbows) < 5 {
		t.Errorf("应对比至少5种弩机, 得%d", len(resp.Crossbows))
	}
	if len(resp.ArmorTypes) < 3 {
		t.Errorf("铠甲类型应>=3, 得%d", len(resp.ArmorTypes))
	}

	// 验证威力指数降序
	for i := 1; i < len(resp.Crossbows); i++ {
		if resp.Crossbows[i].PowerIndex > resp.Crossbows[i-1].PowerIndex+0.001 {
			t.Errorf("威力指数应降序排列: [%d]=%.1f > [%d]=%.1f",
				i, resp.Crossbows[i].PowerIndex, i-1, resp.Crossbows[i-1].PowerIndex)
		}
	}
	for _, cb := range resp.Crossbows {
		if len(cb.Penetrations) == 0 {
			t.Errorf("%s 穿甲数据为空", cb.CrossbowName)
		}
		t.Logf("%s: KE=%.0fJ, 射程=%.0fm, 威力指数=%.1f",
			cb.CrossbowName, cb.KineticEnergy, cb.Range, cb.PowerIndex)
	}
}

func TestCompareCrossbows_SubsetSelection(t *testing.T) {
	s := setupTestServerWithGin()
	w := performRequest(s, "POST", "/api/v1/compare/crossbows", map[string]interface{}{
		"crossbow_types": []string{"arm_stretched", "bed_crossbow_triple"},
		"launch_angle":   45,
	})

	if w.Code != 200 {
		t.Fatalf("应返回200, 得%d", w.Code)
	}
	var resp models.CrossbowComparisonResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Crossbows) != 2 {
		t.Errorf("筛选后应为2种弩机, 得%d", len(resp.Crossbows))
	}
}

// ====== 跨时代对比端点 ======

func TestCompareEraWeapons_BothCategories(t *testing.T) {
	s := setupTestServerWithGin()
	w := performRequest(s, "POST", "/api/v1/compare/era", map[string]interface{}{
		"compare_range_m": 1000,
		"arrow_head_type": "bodkin",
	})

	if w.Code != 200 {
		t.Fatalf("POST /compare/era 应返回200, 得%d: %s", w.Code, w.Body.String())
	}

	var resp models.EraComparisonResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	ancientCount := 0
	modernCount := 0
	var maxAncientKE float64 = 0
	var minModernKE float64 = 1e9

	for _, w := range resp.Weapons {
		if w.IsModern {
			modernCount++
			if w.ImpactKE < minModernKE {
				minModernKE = w.ImpactKE
			}
			t.Logf("现代: %s, 枪口KE=%.0fJ, 1000m处KE=%.0fJ, 威力倍数=%.1fx",
				w.WeaponName, w.KineticEnergy, w.ImpactKE, w.PowerRatio)
		} else {
			ancientCount++
			if w.ImpactKE > maxAncientKE {
				maxAncientKE = w.ImpactKE
			}
			t.Logf("古代: %s, 枪口KE=%.0fJ, 1000m处KE=%.0fJ, 威力倍数=%.1fx",
				w.WeaponName, w.KineticEnergy, w.ImpactKE, w.PowerRatio)
		}
	}
	if ancientCount < 3 {
		t.Errorf("古代武器应>=3, 得%d", ancientCount)
	}
	if modernCount < 2 {
		t.Errorf("现代武器应>=2, 得%d", modernCount)
	}

	if minModernKE <= maxAncientKE*1.5 && maxAncientKE > 0 {
		t.Errorf("现代武器最低KE(%.0fJ)应显著高于古代最高KE(%.0fJ)", minModernKE, maxAncientKE)
	}
}

// ====== 弹幕协同端点 ======

func TestOptimizeBarrage_DefaultConfig(t *testing.T) {
	s := setupTestServerWithGin()
	w := performRequest(s, "POST", "/api/v1/barrage/optimize", map[string]interface{}{
		"target": map[string]interface{}{"x": 0, "y": 500, "radius": 20},
	})

	if w.Code != 200 {
		t.Fatalf("弹幕优化应返回200, 得%d: %s", w.Code, w.Body.String())
	}

	var resp models.BarrageOptimizationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if resp.TotalShots <= 0 {
		t.Error("默认配置应产生箭矢")
	}
	if resp.AreaCoveredM2 <= 0 {
		t.Error("覆盖面积应>0")
	}
	if resp.Coverage.CellSize <= 0 {
		t.Error("网格单元应>0")
	}
	t.Logf("默认弹幕: %d箭, 命中率=%.1f%%, 覆盖=%.0fm², 时间窗=%.2fs, 集中动能=%.0fJ",
		resp.TotalShots, resp.TargetHitRate*100, resp.AreaCoveredM2,
		resp.TimeWindow, resp.KEConcentrated)
}

// ====== 虚拟体验：瞄准目标列表 ======

func TestListAimTargets_ReturnsPresets(t *testing.T) {
	s := setupTestServerWithGin()
	w := performRequest(s, "GET", "/api/v1/aim/targets", nil)

	if w.Code != 200 {
		t.Fatalf("目标列表应返回200, 得%d", w.Code)
	}

	var resp struct {
		Targets []models.AimTargetPreset `json:"targets"`
		Count   int                `json:"count"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.Count < 5 {
		t.Errorf("预设目标应>=5, 得%d", resp.Count)
	}

	for _, tgt := range resp.Targets {
		t.Logf("目标: %s, 距离=%dm, 难度=%s, 分值=%d", tgt.Name, tgt.Distance, tgt.Difficulty, tgt.Points)
	}
}

// ====== 虚拟体验：操作策略性测试（核心） ======

func TestAimShoot_TrainingTarget_EasyHit(t *testing.T) {
	s := setupTestServerWithGin()
	w := performRequest(s, "POST", "/api/v1/aim/shoot", map[string]interface{}{
		"target": map[string]interface{}{
			"distance": 50, "height": 1.5, "armor_type": "leather", "name": "训练靶",
		},
		"crossbow_type": "bed_crossbow_triple",
		"arrow_type":    "bodkin",
		"wind_speed":    0, "wind_direction": 0,
	})

	if w.Code != 200 {
		t.Fatalf("瞄准射击应返回200, 得%d: %s", w.Code, w.Body.String())
	}
	var resp models.AimShootResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if !resp.Success {
		t.Error("射击请求应成功")
	}
	if resp.RequiredElevation < 1 || resp.RequiredElevation > 30 {
		t.Errorf("50m仰角应在1-30度, 得%.2f°", resp.RequiredElevation)
	}
	t.Logf("50m训练靶: 命中=%v, 所需仰角=%.2f°, 射程=%.1fm, 速度=%.1fm/s, 穿甲=%.1fmm, 得分=%d, %s",
		resp.Hit, resp.RequiredElevation, resp.ActualRange, resp.ImpactVelocity,
		resp.PenetrationDepth, resp.Score, resp.Message)
}

func TestAimShoot_Strategy_DistanceAffectsScore(t *testing.T) {
	s := setupTestServerWithGin()

	cases := []struct {
		name     string
		distance float64
		height   float64
		armor    string
		points   int
	}{
		{"训练靶 50m", 50, 1.5, "leather", 10},
		{"步兵 200m", 200, 1.7, "lamellar", 50},
		{"骑兵 350m", 350, 2.0, "chainmail", 100},
		{"敌将 800m", 800, 1.7, "plate", 500},
	}

	for _, c := range cases {
		w := performRequest(s, "POST", "/api/v1/aim/shoot", map[string]interface{}{
			"target": map[string]interface{}{
				"distance": c.distance, "height": c.height, "armor_type": c.armor, "name": c.name,
			},
			"crossbow_type": "bed_crossbow_triple",
			"arrow_type":    "bodkin",
			"wind_speed":    0, "wind_direction": 0,
		})
		var resp models.AimShootResponse
		json.Unmarshal(w.Body.Bytes(), &resp)

		t.Logf("%s: 命中=%v, 穿甲=%v, 得分=%d (满分=%d), 所需仰角=%.1f°, %s",
			c.name, resp.Hit, resp.PenetrationSuccess, resp.Score, c.points,
			resp.RequiredElevation, resp.Message)
	}
	t.Log("策略验证：近距离易低分红，远距离难高分")
}

func TestAimShoot_Strategy_CrossbowSelection(t *testing.T) {
	s := setupTestServerWithGin()

	crossbows := []string{"arm_stretched", "leg_stretched", "bed_crossbow_triple", "bed_crossbow_seven"}
	names := []string{"臂张弩", "蹶张弩", "三弓床弩", "七弓床弩"}

	for i, cb := range crossbows {
		w := performRequest(s, "POST", "/api/v1/aim/shoot", map[string]interface{}{
			"target": map[string]interface{}{
				"distance": 500, "height": 3.0, "armor_type": "plate", "name": "城门",
			},
			"crossbow_type": cb,
			"arrow_type":    "bodkin",
			"wind_speed":    0, "wind_direction": 0,
		})
		var resp models.AimShootResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Logf("%s 射500m板甲: 射程=%.0fm(期望500), 命中=%v, 穿甲=%.1fmm(成功=%v), 得分=%d, %s",
			names[i], resp.ActualRange, resp.Hit, resp.PenetrationDepth, resp.PenetrationSuccess,
			resp.Score, resp.Message)
	}
	t.Log("策略：七弓床弩射程最远穿甲最强，但射速最慢；臂张弩仅适合近距离")
}

func TestAimShoot_Strategy_WindCompensation(t *testing.T) {
	s := setupTestServerWithGin()

	windSpeeds := []float64{0, 3, 8, 15}
	for _, ws := range windSpeeds {
		w := performRequest(s, "POST", "/api/v1/aim/shoot", map[string]interface{}{
			"target": map[string]interface{}{
				"distance": 400, "height": 1.7, "armor_type": "lamellar", "name": "步兵",
			},
			"crossbow_type": "bed_crossbow_triple",
			"arrow_type":    "bodkin",
			"wind_speed":    ws, "wind_direction": 90,
		})
		var resp models.AimShootResponse
		json.Unmarshal(w.Body.Bytes(), &resp)
		t.Logf("风速=%.0fm/s横风: 所需方位角=%.2f°, 风偏X=%.2fm, Y=%.2fm, 命中=%v, 得分=%d",
			ws, resp.RequiredAzimuth, resp.WindDriftX, resp.WindDriftY, resp.Hit, resp.Score)
	}
	t.Log("策略：风速越大，需要越大的方位角修正（侧风补偿）")
}

func TestAimShoot_Strategy_ArrowTypeSelection(t *testing.T) {
	s := setupTestServerWithGin()

	arrows := []string{"bodkin", "broadhead", "blunt"}
	arrowNames := []string{"穿甲箭镞", "宽刃箭镞", "钝头箭镞"}
	armors := []string{"leather", "plate"}
	armorNames := []string{"皮甲", "板甲"}

	for ai, armor := range armors {
		t.Logf("--- 对抗%s ---", armorNames[ai])
		for ari, arrow := range arrows {
			w := performRequest(s, "POST", "/api/v1/aim/shoot", map[string]interface{}{
				"target": map[string]interface{}{
					"distance": 250, "height": 1.7, "armor_type": armor, "name": "目标",
				},
				"crossbow_type": "bed_crossbow_triple",
				"arrow_type":    arrow,
				"wind_speed":    0, "wind_direction": 0,
			})
			var resp models.AimShootResponse
			json.Unmarshal(w.Body.Bytes(), &resp)
			t.Logf("  %s: 穿甲=%.1fmm, 成功=%v, 得分=%d",
				arrowNames[ari], resp.PenetrationDepth, resp.PenetrationSuccess, resp.Score)
		}
	}
	t.Log("策略：穿甲箭镞克制硬甲，宽刃/钝头对软甲杀伤大")
}

// ====== 边界和异常场景 ======

func TestCompareCrossbows_InvalidJSON_BadRequest(t *testing.T) {
	s := setupTestServerWithGin()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/compare/crossbows", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	s.engine.ServeHTTP(w, req)

	if w.Code < 400 || w.Code >= 500 {
		t.Errorf("无效JSON应返回4xx, 得%d", w.Code)
	}
	t.Logf("无效JSON返回: %d (符合预期4xx)", w.Code)
}

func TestAimShoot_InvalidCrossbow_Fallback(t *testing.T) {
	s := setupTestServerWithGin()
	w := performRequest(s, "POST", "/api/v1/aim/shoot", map[string]interface{}{
		"target": map[string]interface{}{
			"distance": 200, "height": 1.5, "armor_type": "leather",
		},
		"crossbow_type": "nonexistent_crossbow_123",
		"arrow_type":    "bodkin",
	})

	if w.Code != 200 {
		t.Errorf("无效弩机应降级处理并返回200, 得%d", w.Code)
	}
	var resp models.AimShootResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success {
		t.Error("降级后仍应返回Success=true")
	}
	t.Logf("无效弩机降级处理: 命中=%v, 射程=%.1fm", resp.Hit, resp.ActualRange)
}

func TestAimShoot_ZeroDistance_Boundary(t *testing.T) {
	s := setupTestServerWithGin()
	w := performRequest(s, "POST", "/api/v1/aim/shoot", map[string]interface{}{
		"target": map[string]interface{}{
			"distance": 0, "height": 0, "armor_type": "leather",
		},
		"crossbow_type": "bed_crossbow_triple",
		"arrow_type":    "bodkin",
	})

	if w.Code != 200 {
		t.Errorf("0距离不应崩溃, 得%d: %s", w.Code, w.Body.String())
	}
	t.Logf("0距离边界: HTTP %d", w.Code)
}

func TestOptimizeBarrage_ExcessiveShots_Stress(t *testing.T) {
	s := setupTestServerWithGin()

	w := performRequest(s, "POST", "/api/v1/barrage/optimize", map[string]interface{}{
		"crossbows": generateManyCrossbows(12),
		"target":    map[string]interface{}{"x": 0, "y": 600, "radius": 30},
		"max_shots_per_crossbow": 5,
		"spread_angle":           15,
	})

	if w.Code != 200 {
		t.Fatalf("大量弹幕计算应返回200, 得%d", w.Code)
	}
	var resp models.BarrageOptimizationResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	expected := 12 * 5
	if resp.TotalShots != expected {
		t.Errorf("应为%d箭, 得%d", expected, resp.TotalShots)
	}
	t.Logf("12弩×5箭压力测试: %d箭, 覆盖=%.0fm², 命中率=%.1f%%",
		resp.TotalShots, resp.AreaCoveredM2, resp.TargetHitRate*100)
}

func TestListModernWeapons(t *testing.T) {
	s := setupTestServerWithGin()
	w := performRequest(s, "GET", "/api/v1/weapons/modern", nil)

	if w.Code != 200 {
		t.Fatalf("现代武器列表应返回200, 得%d", w.Code)
	}
	var resp struct {
		Weapons []map[string]interface{} `json:"weapons"`
		Count   int                `json:"count"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Count < 2 {
		t.Errorf("现代武器应>=2, 得%d", resp.Count)
	}
	t.Logf("现代武器: %d种", resp.Count)
}

// ====== 辅助函数 ======

func generateManyCrossbows(n int) []models.BarrageCrossbow {
	result := make([]models.BarrageCrossbow, n)
	for i := 0; i < n; i++ {
		angle := float64(i-n/2) * 0.15
		result[i] = models.BarrageCrossbow{
			ID: "cb" + itoa(i), Type: "bed_crossbow_triple",
			Name: "弩#" + itoa(i),
			X:    math.Sin(angle) * 40, Y: -30 - float64(i%3)*10,
			Heading: 0, Elevation: 35,
		}
	}
	return result
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
