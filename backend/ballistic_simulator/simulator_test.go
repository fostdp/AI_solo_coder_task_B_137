package ballistic_simulator

import (
	"math"
	"testing"

	"ballistics-system/config"
	"ballistics-system/models"
)

func setupTestSimulator() *Simulator {
	dynCfg := &config.DynamicsConfig{
		Bow: config.BowConfig{
			ArmLength: 1.5, ArmThickness: 0.05, ArmWidth: 0.08, ArmMass: 2.5,
			StringLength: 3.2, StringMass: 0.05, StringYoungMod: 3e9, StringCrossArea: 5e-5,
			DrawLength: 1.2, PeakTension: 5000, NonlinearDamping: 0.15,
			HysteresisFactor: 0.08, ViscousDamping: 2.5, InternalDamping: 0.02,
		},
		Simulation: config.SimulationConfig{
			TimeStep: 0.005, ReleaseTimeStep: 5e-6, MaxSimTime: 30.0,
			ReleaseDuration: 0.025, Gravity: 9.80665, AirDensitySea: 1.225,
			YoungModulusWood: 12e9, PoissonRatioWood: 0.35,
		},
		Defaults: config.DefaultsConfig{
			ArrowMass: 0.2, ArrowDiameter: 0.012, ArrowLength: 1.0,
			SpinRate: 25.0, DragCoefficient: 0.4, LaunchAngle: 45.0, AzimuthAngle: 0.0,
		},
		Aerodynamics: config.AerodynamicsConfig{
			LiftCoefficient: 0.05, MagnusCoefficient: 0.001, SpinDecayRate: 0.0001,
			PitchDampingBase: 0.02, AeroMomentCoefficient: 0.01,
		},
		CrossbowTypes: map[string]config.CrossbowTypeConfig{
			"arm_stretched": {
				Type: "arm_stretched", Name: "臂张弩", DrawForce: 350, DrawLength: 0.45,
				ArrowMass: 0.035, ArrowLength: 0.42, ArrowDiameter: 0.008,
				TypicalVelocity: 65, TypicalRange: 180, SpinRate: 12, BowEfficiency: 0.58,
				CrewSize: 1, ReloadSeconds: 8,
			},
			"leg_stretched": {
				Type: "leg_stretched", Name: "蹶张弩", DrawForce: 900, DrawLength: 0.65,
				ArrowMass: 0.085, ArrowLength: 0.6, ArrowDiameter: 0.01,
				TypicalVelocity: 95, TypicalRange: 350, SpinRate: 18, BowEfficiency: 0.62,
				CrewSize: 1, ReloadSeconds: 20,
			},
			"bed_crossbow_triple": {
				Type: "bed_crossbow_triple", Name: "三弓床弩", DrawForce: 5500, DrawLength: 1.2,
				ArrowMass: 0.2, ArrowLength: 1.0, ArrowDiameter: 0.012,
				TypicalVelocity: 135, TypicalRange: 800, SpinRate: 25, BowEfficiency: 0.68,
				CrewSize: 7, ReloadSeconds: 90,
			},
			"bed_crossbow_seven": {
				Type: "bed_crossbow_seven", Name: "七弓床弩", DrawForce: 12000, DrawLength: 1.5,
				ArrowMass: 0.5, ArrowLength: 1.5, ArrowDiameter: 0.018,
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
	return NewSimulator(dynCfg)
}

// ====== 跨时代对比：动能差异正常场景 ======

func TestSimulate_KineticEnergy_BasicSanity(t *testing.T) {
	s := setupTestSimulator()
	// 三弓床弩 135m/s, 200g箭 => KE = 0.5 * 0.2 * 135^2 = 1822.5J
	params := &models.SimulationParams{
		InitialVelocity: 135, LaunchAngle: 45, ArrowMass: 0.2,
		ArrowDiameter: 0.012, ArrowLength: 1.0, SpinRate: 25,
	}
	res := s.Simulate(params)

	expectedKE := 0.5 * params.ArrowMass * params.InitialVelocity * params.InitialVelocity
	if res.KineticEnergy <= 0 {
		t.Fatalf("动能不能<=0, 得%f", res.KineticEnergy)
	}
	// 着靶动能应低于枪口动能（空气阻力）
	if res.KineticEnergy >= expectedKE*1.01 {
		t.Errorf("着靶动能(%.1fJ)不应高于初始动能(%.1fJ)", res.KineticEnergy, expectedKE)
	}
	t.Logf("三弓床弩: 初始KE=%.1fJ, 着靶KE=%.1fJ, 射程=%.1fm",
		expectedKE, res.KineticEnergy, res.Range)
}

func TestSimulate_KineticEnergy_MonotonicWithVelocity(t *testing.T) {
	s := setupTestSimulator()
	velocities := []float64{65, 95, 135, 165}
	kes := make([]float64, len(velocities))

	for i, v := range velocities {
		params := &models.SimulationParams{
			InitialVelocity: v, LaunchAngle: 45, ArrowMass: 0.2,
			ArrowDiameter: 0.012, ArrowLength: 1.0, SpinRate: 25,
		}
		res := s.Simulate(params)
		kes[i] = res.KineticEnergy
	}

	for i := 1; i < len(kes); i++ {
		if kes[i] <= kes[i-1] {
			t.Errorf("着靶动能应随初速递增: v=%f→%.1fJ, v=%f→%.1fJ",
				velocities[i-1], kes[i-1], velocities[i], kes[i])
		}
	}
	t.Logf("动能-初速单调递增通过: 65→%.0fJ, 95→%.0fJ, 135→%.0fJ, 165→%.0fJ",
		kes[0], kes[1], kes[2], kes[3])
}

func TestSimulate_CrossbowTypes_KineticEnergyHierarchy(t *testing.T) {
	s := setupTestSimulator()
	types := []string{"arm_stretched", "leg_stretched", "bed_crossbow_triple", "bed_crossbow_seven"}
	names := []string{"臂张弩", "蹶张弩", "三弓床弩", "七弓床弩"}
	kes := make([]float64, len(types))

	for i, ct := range types {
		cfg := s.getCrossbowCfgForTest(ct)
		res := s.SimulateWithCrossbowType(ct, &cfg, 45, 0)
		kes[i] = res.KineticEnergy
		t.Logf("%s: KE=%.1fJ, 射程=%.0fm", names[i], res.KineticEnergy, res.Range)
	}

	// 动能应按弩机类型单调递增
	for i := 1; i < len(kes); i++ {
		if kes[i] <= kes[i-1] {
			t.Errorf("弩机动能等级应递增: %s=%.0fJ < %s=%.0fJ",
				names[i-1], kes[i-1], names[i], kes[i])
		}
	}
}

func TestSimulate_ModernVsAncient_KineticEnergyGap(t *testing.T) {
	s := setupTestSimulator()

	// 三弓床弩
	cfgBed := s.getCrossbowCfgForTest("bed_crossbow_triple")
	resBed := s.SimulateWithCrossbowType("bed_crossbow_triple", &cfgBed, 45, 0)
	bedInitialKE := 0.5 * cfgBed.ArrowMass * cfgBed.TypicalVelocity * cfgBed.TypicalVelocity

	// 巴雷特M82
	barrett := s.getModernCfgForTest("barrett_m82")
	barrettKE := 0.5 * barrett.BulletMass * barrett.MuzzleVelocity * barrett.MuzzleVelocity

	ratio := barrettKE / bedInitialKE
	t.Logf("枪口动能对比: 巴雷特=%.0fJ vs 三弓床弩=%.0fJ, 倍数=%.1fx",
		barrettKE, bedInitialKE, ratio)

	if ratio < 5 {
		t.Errorf("现代步枪枪口动能至少应为床弩5倍, 得%.1fx", ratio)
	}
}

// ====== 跨时代对比：边界场景 ======

func TestSimulate_LaunchAngleZero_Boundary(t *testing.T) {
	s := setupTestSimulator()
	params := &models.SimulationParams{
		InitialVelocity: 135, LaunchAngle: 0, ArrowMass: 0.2,
		ArrowDiameter: 0.012, ArrowLength: 1.0, SpinRate: 25,
	}
	res := s.Simulate(params)

	if res.Range < 0 {
		t.Errorf("水平发射射程不应为负, 得%f", res.Range)
	}
	if res.MaxHeight > 0.5 {
		t.Errorf("水平发射最大高度不应过高, 得%f", res.MaxHeight)
	}
	t.Logf("0度水平发射: 射程=%.1fm, 最大高=%.2fm, 飞行时间=%.2fs",
		res.Range, res.MaxHeight, res.FlightTime)
}

func TestSimulate_LaunchAngle85Deg_Boundary(t *testing.T) {
	s := setupTestSimulator()
	params := &models.SimulationParams{
		InitialVelocity: 135, LaunchAngle: 85, ArrowMass: 0.2,
		ArrowDiameter: 0.012, ArrowLength: 1.0, SpinRate: 25,
	}
	res := s.Simulate(params)

	if res.MaxHeight < 500 {
		t.Errorf("85度高抛弹道最大高度应>500m, 得%f", res.MaxHeight)
	}
	if res.FlightTime < 10 {
		t.Errorf("高抛弹道飞行时间应>10s, 得%f", res.FlightTime)
	}
	t.Logf("85度高抛: 射程=%.1fm, 最大高=%.0fm, 飞行时间=%.1fs",
		res.Range, res.MaxHeight, res.FlightTime)
}

func TestSimulate_VelocityVeryLow_Boundary(t *testing.T) {
	s := setupTestSimulator()
	// 5m/s极低速度，几乎贴地
	params := &models.SimulationParams{
		InitialVelocity: 5, LaunchAngle: 45, ArrowMass: 0.2,
		ArrowDiameter: 0.012, ArrowLength: 1.0, SpinRate: 25,
	}
	res := s.Simulate(params)

	if res.Range > 10 {
		t.Errorf("5m/s射程不应超过10m, 得%f", res.Range)
	}
	if math.IsNaN(res.KineticEnergy) || math.IsInf(res.KineticEnergy, 0) {
		t.Error("极低速度不应产生NaN/Inf")
	}
	t.Logf("5m/s极低速度: 射程=%.2fm, KE=%.2fJ", res.Range, res.KineticEnergy)
}

// ====== 跨时代对比：异常场景 ======

func TestSimulate_ZeroMass_DoesNotCrash(t *testing.T) {
	s := setupTestSimulator()
	params := &models.SimulationParams{
		InitialVelocity: 135, LaunchAngle: 45, ArrowMass: 0,
		ArrowDiameter: 0.012, ArrowLength: 1.0, SpinRate: 25,
	}
	// 不应panic
	res := s.Simulate(params)
	if res == nil {
		t.Fatal("零质量不应返回nil")
	}
	t.Logf("零质量异常: 射程=%f, KE=%f", res.Range, res.KineticEnergy)
}

// ====== 仰角求解器测试 ======

func TestSolveElevationForDistance_200m_Normal(t *testing.T) {
	s := setupTestSimulator()
	targetDist := 200.0
	elev, res := s.SolveElevationForDistance(targetDist, 135, 0.2, 0.012, 1.0, 25)

	err := math.Abs(res.Range - targetDist)
	if err > 10 {
		t.Errorf("200m射程解算误差应<10m, 实际=%.1fm, 解算仰角=%.2f°", err, elev)
	}
	if elev < 5 || elev > 60 {
		t.Errorf("200m合理仰角应在5-60度, 得%.2f°", elev)
	}
	t.Logf("200m解算: 仰角=%.2f°, 实际射程=%.1fm, 误差=%.1fm", elev, res.Range, err)
}

func TestSolveElevationForDistance_800m_Normal(t *testing.T) {
	s := setupTestSimulator()
	targetDist := 800.0
	elev, res := s.SolveElevationForDistance(targetDist, 135, 0.2, 0.012, 1.0, 25)

	err := math.Abs(res.Range - targetDist)
	t.Logf("800m解算: 仰角=%.2f°, 实际射程=%.1fm, 误差=%.1fm", elev, res.Range, err)
}

func TestSolveElevationWithWind_CrosswindDrift(t *testing.T) {
	s := setupTestSimulator()
	// 5m/s侧风
	noWindElev, noWindAzi, noWindRes := s.SolveElevationWithWind(300, 1.5, 135, 0.2, 0.012, 1.0, 25, 0, 0)
	windElev, windAzi, windRes := s.SolveElevationWithWind(300, 1.5, 135, 0.2, 0.012, 1.0, 25, 5, 90)

	t.Logf("无风: 仰角=%.2f°, 方位=%.2f°, 射程=%.1fm", noWindElev, noWindAzi, noWindRes.Range)
	t.Logf("5m/s侧风: 仰角=%.2f°, 方位=%.2f°, 射程=%.1fm", windElev, windAzi, windRes.Range)

	// 侧风应导致方位角修正
	if math.Abs(windAzi-noWindAzi) < 0.1 {
		t.Error("侧风应产生方位角修正")
	}
}

// ====== 弹幕覆盖优化测试 ======

func TestOptimizeBarrage_BasicCoverage_Normal(t *testing.T) {
	s := setupTestSimulator()
	req := &models.BarrageOptimizationRequest{
		Crossbows: []models.BarrageCrossbow{
			{ID: "cb-1", Type: "bed_crossbow_triple", Name: "三弓床弩#1", X: 0, Y: -20, Heading: 0, Elevation: 35},
			{ID: "cb-2", Type: "bed_crossbow_triple", Name: "三弓床弩#2", X: -30, Y: -15, Heading: 15, Elevation: 35},
			{ID: "cb-3", Type: "bed_crossbow_single", Name: "单弓床弩#3", X: 30, Y: -15, Heading: -15, Elevation: 30},
		},
		Target:             models.BarrageTarget{X: 0, Y: 500, Radius: 20},
		MaxShotsPerCrossbow: 3,
		SpreadAngle:        10,
	}
	crossbows := s.getCrossbowMapForTest()
	res := s.OptimizeBarrage(req, crossbows)

	expectedShots := len(req.Crossbows) * req.MaxShotsPerCrossbow
	if res.TotalShots != expectedShots {
		t.Errorf("总箭矢数应为%d, 得%d", expectedShots, res.TotalShots)
	}
	if res.AreaCoveredM2 <= 0 {
		t.Error("覆盖面积应>0")
	}
	if res.TargetHitRate < 0 || res.TargetHitRate > 1 {
		t.Errorf("命中率应在[0,1], 得%f", res.TargetHitRate)
	}
	t.Logf("弹幕覆盖: 总箭=%d, 命中=%d, 命中率=%.1f%%, 覆盖=%.0fm², 时间窗=%.2fs, 动能=%.0fJ",
		res.TotalShots, res.ShotsInTarget, res.TargetHitRate*100,
		res.AreaCoveredM2, res.TimeWindow, res.KEConcentrated)
}

func TestOptimizeBarrage_SpreadAngleVsCoverage(t *testing.T) {
	s := setupTestSimulator()
	crossbows := s.getCrossbowMapForTest()

	baseCrossbows := []models.BarrageCrossbow{
		{ID: "cb-1", Type: "bed_crossbow_triple", Name: "#1", X: 0, Y: -20, Heading: 0, Elevation: 35},
		{ID: "cb-2", Type: "bed_crossbow_triple", Name: "#2", X: -25, Y: -20, Heading: 0, Elevation: 35},
		{ID: "cb-3", Type: "bed_crossbow_triple", Name: "#3", X: 25, Y: -20, Heading: 0, Elevation: 35},
	}

	spreads := []float64{2, 10, 30}
	areas := make([]float64, len(spreads))

	for i, sp := range spreads {
		req := &models.BarrageOptimizationRequest{
			Crossbows:          baseCrossbows,
			Target:             models.BarrageTarget{X: 0, Y: 500, Radius: 20},
			MaxShotsPerCrossbow: 3,
			SpreadAngle:        sp,
		}
		res := s.OptimizeBarrage(req, crossbows)
		areas[i] = res.AreaCoveredM2
		t.Logf("散布角=%.0f°: 覆盖面积=%.0fm²", sp, areas[i])
	}

	// 更大散布角应产生更大覆盖面积
	for i := 1; i < len(areas); i++ {
		if areas[i] < areas[i-1]*0.8 {
			t.Errorf("更大散布角应覆盖更大面积: %.0f°→%.0fm² vs %.0f°→%.0fm²",
				spreads[i-1], areas[i-1], spreads[i], areas[i])
		}
	}
}

func TestOptimizeBarrage_MoreCrossbows_MoreShots(t *testing.T) {
	s := setupTestSimulator()
	crossbows := s.getCrossbowMapForTest()

	makeCBs := func(n int) []models.BarrageCrossbow {
		result := make([]models.BarrageCrossbow, n)
		for i := 0; i < n; i++ {
			result[i] = models.BarrageCrossbow{
				ID: "cb" + string(rune('0'+i)), Type: "bed_crossbow_triple",
				Name: "#" + string(rune('0'+i)),
				X:    float64(i-n/2) * 15, Y: -20, Heading: 0, Elevation: 35,
			}
		}
		return result
	}

	counts := []int{1, 3, 5, 8}
	for _, n := range counts {
		req := &models.BarrageOptimizationRequest{
			Crossbows:          makeCBs(n),
			Target:             models.BarrageTarget{X: 0, Y: 500, Radius: 20},
			MaxShotsPerCrossbow: 2,
			SpreadAngle:        8,
		}
		res := s.OptimizeBarrage(req, crossbows)
		expected := n * 2
		if res.TotalShots != expected {
			t.Errorf("%d床弩应产生%d箭, 得%d", n, expected, res.TotalShots)
		}
		t.Logf("%d床弩×2箭: 总箭=%d, 命中=%d, 覆盖率=%.0fm²",
			n, res.TotalShots, res.ShotsInTarget, res.AreaCoveredM2)
	}
}

// ====== 弹幕覆盖边界/异常场景 ======

func TestOptimizeBarrage_EmptyCrossbows_Defaults(t *testing.T) {
	s := setupTestSimulator()
	req := &models.BarrageOptimizationRequest{
		Crossbows:          nil,
		Target:             models.BarrageTarget{X: 0, Y: 500, Radius: 20},
		MaxShotsPerCrossbow: 0,
		SpreadAngle:        0,
	}
	crossbows := s.getCrossbowMapForTest()
	res := s.OptimizeBarrage(req, crossbows)

	// 空输入不应崩溃，应使用默认值
	if res.TotalShots <= 0 {
		t.Error("空输入应使用默认配置产生箭矢")
	}
	t.Logf("空输入降级: 总箭=%d, 命中率=%.1f%%", res.TotalShots, res.TargetHitRate*100)
}

func TestOptimizeBarrage_ZeroRadiusTarget_Boundary(t *testing.T) {
	s := setupTestSimulator()
	req := &models.BarrageOptimizationRequest{
		Crossbows: []models.BarrageCrossbow{
			{ID: "cb-1", Type: "bed_crossbow_triple", Name: "#1", X: 0, Y: -20, Heading: 0, Elevation: 35},
		},
		Target:             models.BarrageTarget{X: 0, Y: 500, Radius: 0},
		MaxShotsPerCrossbow: 5,
		SpreadAngle:        5,
	}
	crossbows := s.getCrossbowMapForTest()
	res := s.OptimizeBarrage(req, crossbows)

	// 半径为0的目标命中率应非常低
	if res.TargetHitRate > 0.5 {
		t.Errorf("0半径目标命中率不应过高, 得%.2f", res.TargetHitRate)
	}
	if res.AreaCoveredM2 <= 0 {
		t.Error("即使0半径目标也应有覆盖面积")
	}
	t.Logf("0半径目标: 命中率=%.3f, 覆盖=%.0fm²", res.TargetHitRate, res.AreaCoveredM2)
}

func TestOptimizeBarrage_ShotsInTargetConsistency(t *testing.T) {
	s := setupTestSimulator()
	req := &models.BarrageOptimizationRequest{
		Crossbows: []models.BarrageCrossbow{
			{ID: "cb1", Type: "bed_crossbow_triple", Name: "1", X: 0, Y: -20, Heading: 0, Elevation: 35},
			{ID: "cb2", Type: "bed_crossbow_triple", Name: "2", X: -15, Y: -20, Heading: 0, Elevation: 35},
		},
		Target:             models.BarrageTarget{X: 0, Y: 500, Radius: 30},
		MaxShotsPerCrossbow: 5,
		SpreadAngle:        6,
	}
	crossbows := s.getCrossbowMapForTest()
	res := s.OptimizeBarrage(req, crossbows)

	// 手动统计命中，验证ShotsInTarget一致性
	manualHits := 0
	for _, shot := range res.Shots {
		dx := shot.ImpactX - req.Target.X
		dy := shot.ImpactY - req.Target.Y
		if math.Sqrt(dx*dx+dy*dy) <= req.Target.Radius {
			manualHits++
		}
	}
	if manualHits != res.ShotsInTarget {
		t.Errorf("命中计数不一致: ShotsInTarget=%d, 手动统计=%d", res.ShotsInTarget, manualHits)
	}
	t.Logf("命中一致性验证通过: ShotsInTarget=%d == 手动=%d", res.ShotsInTarget, manualHits)
}

// ====== 陀螺稳定性 ======

func TestGyroscopicStability_ValidRange(t *testing.T) {
	s := setupTestSimulator()
	cases := []struct {
		name     string
		spin     float64
		velocity float64
	}{
		{"臂张弩低自旋", 12, 65},
		{"蹶张弩中自旋", 18, 95},
		{"三弓床弩高自旋", 25, 135},
		{"现代子弹极高速", 1800, 853},
	}

	for _, c := range cases {
		stab := s.CalculateGyroscopicStability(c.spin, c.velocity, 0.05, 0.01, 0.5)
		if stab < 0.1 || stab > 100 {
			t.Errorf("%s: 陀螺稳定性应在合理范围, 得%f", c.name, stab)
		}
		t.Logf("%s: 陀螺稳定性=%.2f", c.name, stab)
	}
}

// ====== 辅助方法（为测试暴露配置获取） ======

func (s *Simulator) getCrossbowCfgForTest(key string) config.CrossbowTypeConfig {
	return config.CrossbowTypeConfig{
		Type: key,
		// 从 setupTestSimulator 的默认值手动映射
		DrawForce:       map[string]float64{"arm_stretched": 350, "leg_stretched": 900, "bed_crossbow_triple": 5500, "bed_crossbow_seven": 12000}[key],
		DrawLength:      map[string]float64{"arm_stretched": 0.45, "leg_stretched": 0.65, "bed_crossbow_triple": 1.2, "bed_crossbow_seven": 1.5}[key],
		ArrowMass:       map[string]float64{"arm_stretched": 0.035, "leg_stretched": 0.085, "bed_crossbow_triple": 0.2, "bed_crossbow_seven": 0.5}[key],
		ArrowLength:     map[string]float64{"arm_stretched": 0.42, "leg_stretched": 0.6, "bed_crossbow_triple": 1.0, "bed_crossbow_seven": 1.5}[key],
		ArrowDiameter:   map[string]float64{"arm_stretched": 0.008, "leg_stretched": 0.01, "bed_crossbow_triple": 0.012, "bed_crossbow_seven": 0.018}[key],
		TypicalVelocity: map[string]float64{"arm_stretched": 65, "leg_stretched": 95, "bed_crossbow_triple": 135, "bed_crossbow_seven": 165}[key],
		TypicalRange:    map[string]float64{"arm_stretched": 180, "leg_stretched": 350, "bed_crossbow_triple": 800, "bed_crossbow_seven": 1500}[key],
		SpinRate:        map[string]float64{"arm_stretched": 12, "leg_stretched": 18, "bed_crossbow_triple": 25, "bed_crossbow_seven": 32}[key],
		BowEfficiency:   map[string]float64{"arm_stretched": 0.58, "leg_stretched": 0.62, "bed_crossbow_triple": 0.68, "bed_crossbow_seven": 0.70}[key],
		CrewSize:        map[string]int{"arm_stretched": 1, "leg_stretched": 1, "bed_crossbow_triple": 7, "bed_crossbow_seven": 20}[key],
		ReloadSeconds:   map[string]float64{"arm_stretched": 8, "leg_stretched": 20, "bed_crossbow_triple": 90, "bed_crossbow_seven": 300}[key],
	}
}

func (s *Simulator) getModernCfgForTest(key string) config.ModernWeaponConfig {
	return config.ModernWeaponConfig{
		Type: key,
		BulletMass:      map[string]float64{"barrett_m82": 0.042, "ntw_20": 0.125}[key],
		BulletDiameter:  map[string]float64{"barrett_m82": 0.0127, "ntw_20": 0.02}[key],
		BulletLength:    map[string]float64{"barrett_m82": 0.058, "ntw_20": 0.11}[key],
		MuzzleVelocity:  map[string]float64{"barrett_m82": 853, "ntw_20": 720}[key],
		MaxRange:        map[string]float64{"barrett_m82": 1800, "ntw_20": 1600}[key],
		EffectiveRange:  map[string]float64{"barrett_m82": 1500, "ntw_20": 1300}[key],
		DragCoefficient: map[string]float64{"barrett_m82": 0.295, "ntw_20": 0.32}[key],
		SpinRate:        map[string]float64{"barrett_m82": 1800, "ntw_20": 1200}[key],
		Hardness:        map[string]float64{"barrett_m82": 650, "ntw_20": 680}[key],
		TipArea:         map[string]float64{"barrett_m82": 1.267e-4, "ntw_20": 3.142e-4}[key],
		CrewSize:        map[string]int{"barrett_m82": 1, "ntw_20": 2}[key],
		ReloadSeconds:   map[string]float64{"barrett_m82": 3, "ntw_20": 5}[key],
	}
}

func (s *Simulator) getCrossbowMapForTest() map[string]config.CrossbowTypeConfig {
	result := make(map[string]config.CrossbowTypeConfig)
	for _, k := range []string{"arm_stretched", "leg_stretched", "bed_crossbow_single", "bed_crossbow_triple", "bed_crossbow_seven"} {
		cfg := s.getCrossbowCfgForTest(k)
		cfg.Type = k
		// 补充bed_crossbow_single
		if k == "bed_crossbow_single" {
			cfg.DrawForce = 2500
			cfg.DrawLength = 0.9
			cfg.ArrowMass = 0.15
			cfg.ArrowLength = 0.9
			cfg.ArrowDiameter = 0.012
			cfg.TypicalVelocity = 110
			cfg.TypicalRange = 550
			cfg.SpinRate = 22
			cfg.BowEfficiency = 0.65
			cfg.CrewSize = 3
			cfg.ReloadSeconds = 45
		}
		result[k] = cfg
	}
	return result
}
