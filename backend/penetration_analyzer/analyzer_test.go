package penetration_analyzer

import (
	"math"
	"testing"

	"ballistics-system/backend/config"
)

func setupTestAnalyzer() *Analyzer {
	armorCfg := &config.ArmorConfig{
		Armors: map[string]config.ArmorEntryConfig{
			"leather": {
				Type: "leather", Thickness: 0.008, Density: 1000,
				YieldStrength: 40e6, Hardness: 150, Name: "皮甲",
			},
			"lamellar": {
				Type: "lamellar", Thickness: 0.004, Density: 7850,
				YieldStrength: 200e6, Hardness: 280, Name: "鳞甲",
			},
			"chainmail": {
				Type: "chainmail", Thickness: 0.006, Density: 7850,
				YieldStrength: 250e6, Hardness: 300, Name: "锁子甲",
			},
			"plate": {
				Type: "plate", Thickness: 0.0025, Density: 7850,
				YieldStrength: 500e6, Hardness: 450, Name: "板甲",
			},
		},
		ArrowHeads: map[string]config.ArrowHeadEntryConfig{
			"bodkin": {
				Type: "bodkin", TipDiameter: 0.004, TipArea: 1.256e-5,
				TipMass: 0.03, Hardness: 550, Name: "穿甲箭镞",
			},
			"broadhead": {
				Type: "broadhead", TipDiameter: 0.03, TipArea: 7.068e-4,
				TipMass: 0.05, Hardness: 400, Name: "宽刃箭镞",
			},
			"blunt": {
				Type: "blunt", TipDiameter: 0.015, TipArea: 1.767e-4,
				TipMass: 0.04, Hardness: 300, Name: "钝头箭镞",
			},
		},
		Gyro: config.GyroConfig{
			YawStableThreshold:       4.0,
			YawModerateThreshold:     1.5,
			YawMarginalThreshold:     1.0,
			StabilityPenaltyFull:     2.0,
			StabilityPenaltyModerate: 1.0,
			StabilityPenaltyPoor:     0.5,
			RotaryEnergyBoostFactor:  0.15,
			StabilityClampMin:        0.1,
			StabilityClampMax:        50.0,
			LowVelocityThreshold:     1.0,
			LowVelocityStability:     10.0,
		},
	}
	return NewAnalyzer(armorCfg)
}

// ====== 威力对比：穿甲深度正常场景 ======

func TestAnalyze_ArmStretchedVsLeather_Normal(t *testing.T) {
	a := setupTestAnalyzer()
	// 臂张弩典型速度65m/s, 箭重35g
	res := a.AnalyzeWithSpin(65, 0.035, 0.008, 0.42, 12, "leather", "bodkin", 0)

	if res.PenetrationDepth <= 0 {
		t.Fatalf("臂张弩射皮甲穿深应>0, 得%f", res.PenetrationDepth)
	}
	if !res.Success {
		t.Errorf("臂张弩65m/s应穿透8mm皮甲, 穿深=%fmm", res.PenetrationDepth*1000)
	}
	t.Logf("臂张弩→皮甲: 穿深=%.2fmm, 成功=%v", res.PenetrationDepth*1000, res.Success)
}

func TestAnalyze_LegStretchedVsLamellar_Normal(t *testing.T) {
	a := setupTestAnalyzer()
	// 蹶张弩典型速度95m/s, 箭重85g
	res := a.AnalyzeWithSpin(95, 0.085, 0.01, 0.6, 18, "lamellar", "bodkin", 0)

	if res.PenetrationDepth <= 0 {
		t.Fatalf("蹶张弩射鳞甲穿深应>0")
	}
	ke := 0.5 * 0.085 * 95 * 95
	t.Logf("蹶张弩(KE=%.0fJ)→鳞甲4mm: 穿深=%.2fmm, 成功=%v", ke, res.PenetrationDepth*1000, res.Success)
}

func TestAnalyze_BedCrossbowTripleVsPlate_Normal(t *testing.T) {
	a := setupTestAnalyzer()
	// 三弓床弩典型速度135m/s, 箭重200g
	res := a.AnalyzeWithSpin(135, 0.2, 0.012, 1.0, 25, "plate", "bodkin", 0)

	ke := 0.5 * 0.2 * 135 * 135
	if ke < 1500 {
		t.Errorf("三弓床弩动能不能低于1500J, 得%.0fJ", ke)
	}
	t.Logf("三弓床弩(KE=%.0fJ)→板甲2.5mm: 穿深=%.2fmm, 成功=%v", ke, res.PenetrationDepth*1000, res.Success)
}

func TestAnalyze_PenetrationDepth_MonotonicWithVelocity(t *testing.T) {
	a := setupTestAnalyzer()
	velocities := []float64{50, 80, 110, 140, 170}
	depths := make([]float64, len(velocities))

	for i, v := range velocities {
		res := a.AnalyzeWithSpin(v, 0.15, 0.012, 0.9, 22, "chainmail", "bodkin", 0)
		depths[i] = res.PenetrationDepth
	}

	for i := 1; i < len(depths); i++ {
		if depths[i] <= depths[i-1] {
			t.Errorf("穿深应随速度单调递增: v=%f→%.5f, v=%f→%.5f",
				velocities[i-1], depths[i-1], velocities[i], depths[i])
		}
	}
	t.Log("穿深随速度单调递增验证通过")
}

func TestAnalyze_ArrowHeadType_Dominance(t *testing.T) {
	a := setupTestAnalyzer()
	v := 120.0
	types := []string{"bodkin", "broadhead", "blunt"}
	results := make(map[string]float64)

	for _, at := range types {
		res := a.AnalyzeWithSpin(v, 0.15, 0.012, 0.9, 22, "plate", at, 0)
		results[at] = res.PenetrationDepth
	}

	// 穿甲箭镞对硬甲穿深应最大
	if results["bodkin"] <= results["broadhead"] || results["bodkin"] <= results["blunt"] {
		t.Errorf("穿甲箭镞应对板甲穿深最高: bodkin=%.2fmm, broadhead=%.2fmm, blunt=%.2fmm",
			results["bodkin"]*1000, results["broadhead"]*1000, results["blunt"]*1000)
	}
	t.Logf("箭镞穿深对比(板甲): bodkin=%.2fmm > broadhead=%.2fmm > blunt=%.2fmm",
		results["bodkin"]*1000, results["broadhead"]*1000, results["blunt"]*1000)
}

// ====== 威力对比：穿甲深度边界场景 ======

func TestAnalyze_VelocityZero_Boundary(t *testing.T) {
	a := setupTestAnalyzer()
	res := a.AnalyzeWithSpin(0, 0.2, 0.012, 1.0, 25, "leather", "bodkin", 0)

	if res.PenetrationDepth < 0 {
		t.Errorf("零速度穿深不应为负, 得%f", res.PenetrationDepth)
	}
	if res.Success {
		t.Error("零速度不应穿透任何铠甲")
	}
	if res.ImpactVelocity != 0 {
		t.Errorf("ImpactVelocity应记录0, 得%f", res.ImpactVelocity)
	}
	t.Logf("零速度边界: 穿深=%.6fmm, 成功=%v", res.PenetrationDepth*1000, res.Success)
}

func TestAnalyze_VelocityExtremeHigh_Boundary(t *testing.T) {
	a := setupTestAnalyzer()
	// 现代狙击级速度 850m/s
	res := a.AnalyzeWithSpin(850, 0.042, 0.0127, 0.058, 1800, "plate", "bodkin", 0)

	if math.IsNaN(res.PenetrationDepth) || math.IsInf(res.PenetrationDepth, 0) {
		t.Errorf("极高速度不应产生NaN/Inf穿深")
	}
	if res.PenetrationDepth <= 0 {
		t.Errorf("850m/s子弹穿深板甲应>0, 得%f", res.PenetrationDepth)
	}
	t.Logf("850m/s极高速度: 穿深=%.2fmm, 成功=%v", res.PenetrationDepth*1000, res.Success)
}

func TestAnalyze_ArmorThicknessExplicit_Boundary(t *testing.T) {
	a := setupTestAnalyzer()
	// 指定厚度10mm板甲（远超默认2.5mm）
	res := a.AnalyzeWithSpin(135, 0.2, 0.012, 1.0, 25, "plate", "bodkin", 0.010)

	if res.Success {
		t.Error("三弓床弩不应穿透10mm厚板甲")
	}
	if res.ArmorThickness != 0.010 {
		t.Errorf("ArmorThickness应记录显式值0.010, 得%f", res.ArmorThickness)
	}
	t.Logf("10mm厚板甲边界: 穿深=%.2fmm, 成功=%v", res.PenetrationDepth*1000, res.Success)
}

func TestAnalyze_SpinRateZero_Boundary(t *testing.T) {
	a := setupTestAnalyzer()
	resWithSpin := a.AnalyzeWithSpin(120, 0.15, 0.012, 0.9, 25, "chainmail", "bodkin", 0)
	resNoSpin := a.AnalyzeWithSpin(120, 0.15, 0.012, 0.9, 0, "chainmail", "bodkin", 0)

	// 无自旋穿深应不高于有自旋（旋转能量贡献）
	if resNoSpin.PenetrationDepth > resWithSpin.PenetrationDepth*1.1 {
		t.Errorf("无自旋穿深不应显著高于有自旋: no_spin=%.2fmm, with_spin=%.2fmm",
			resNoSpin.PenetrationDepth*1000, resWithSpin.PenetrationDepth*1000)
	}
	t.Logf("自旋边界: 有自旋=%.2fmm, 无自旋=%.2fmm",
		resWithSpin.PenetrationDepth*1000, resNoSpin.PenetrationDepth*1000)
}

// ====== 威力对比：穿甲深度异常场景 ======

func TestAnalyze_InvalidArmorType_Fallback(t *testing.T) {
	a := setupTestAnalyzer()
	res := a.AnalyzeWithSpin(100, 0.1, 0.01, 0.5, 20, "nonexistent_armor", "bodkin", 0)

	// 应降级到默认皮甲，不应崩溃
	if res == nil {
		t.Fatal("无效铠甲类型不应返回nil")
	}
	if res.ArmorType != "nonexistent_armor" {
		t.Errorf("ArmorType应保留请求值, 得%s", res.ArmorType)
	}
	if math.IsNaN(res.PenetrationDepth) {
		t.Error("降级后穿深不应为NaN")
	}
	t.Logf("无效铠甲降级: 穿深=%.2fmm", res.PenetrationDepth*1000)
}

func TestAnalyze_InvalidArrowHead_Fallback(t *testing.T) {
	a := setupTestAnalyzer()
	res := a.AnalyzeWithSpin(100, 0.1, 0.01, 0.5, 20, "leather", "fake_arrow", 0)

	if res == nil {
		t.Fatal("无效箭镞类型不应返回nil")
	}
	if res.ArrowHeadType != "fake_arrow" {
		t.Errorf("ArrowHeadType应保留请求值, 得%s", res.ArrowHeadType)
	}
	t.Logf("无效箭镞降级: 穿深=%.2fmm", res.PenetrationDepth*1000)
}

func TestAnalyze_NegativeMass_EdgeCase(t *testing.T) {
	a := setupTestAnalyzer()
	// 负质量为异常输入，应不崩溃
	res := a.AnalyzeWithSpin(100, -0.1, 0.01, 0.5, 20, "leather", "bodkin", 0)

	if res == nil {
		t.Fatal("负质量不应返回nil")
	}
	t.Logf("负质量异常: 穿深=%f, KE相关计算由调用方保证输入合法性", res.PenetrationDepth)
}

// ====== 现代子弹穿甲对比 ======

func TestAnalyzeModernBullet_BarrettVsPlate_Normal(t *testing.T) {
	a := setupTestAnalyzer()
	barrett := &config.ModernWeaponConfig{
		Type: "barrett_m82", BulletMass: 0.042, BulletDiameter: 0.0127,
		BulletLength: 0.058, SpinRate: 1800, Hardness: 650, TipArea: 1.267e-4,
	}
	// 1000m处剩余速度估算约600m/s
	res := a.AnalyzeModernBullet(600, barrett, "plate", 0)

	if !res.Success {
		t.Errorf("巴雷特600m/s应穿透2.5mm板甲, 穿深=%.2fmm", res.PenetrationDepth*1000)
	}
	if res.PenetrationDepth < 0.005 {
		t.Errorf("巴雷特穿深板甲应>5mm, 得%.2fmm", res.PenetrationDepth*1000)
	}
	t.Logf("巴雷特M82→板甲: 穿深=%.2fmm, 成功=%v", res.PenetrationDepth*1000, res.Success)
}

func TestAnalyzeModernBullet_vs_Crossbow_PenetrationGap(t *testing.T) {
	a := setupTestAnalyzer()

	// 三弓床弩135m/s 200g箭 vs 巴雷特853m/s 42g弹，对比板甲穿深
	crossbow := a.AnalyzeWithSpin(135, 0.2, 0.012, 1.0, 25, "plate", "bodkin", 0)
	barrett := &config.ModernWeaponConfig{
		Type: "barrett_m82", BulletMass: 0.042, BulletDiameter: 0.0127,
		BulletLength: 0.058, SpinRate: 1800, Hardness: 650, TipArea: 1.267e-4,
	}
	modern := a.AnalyzeModernBullet(853, barrett, "plate", 0)

	ratio := modern.PenetrationDepth / crossbow.PenetrationDepth
	t.Logf("穿深倍数对比(现代/古代): 巴雷特=%.2fmm / 三弓床弩=%.2fmm = %.1fx",
		modern.PenetrationDepth*1000, crossbow.PenetrationDepth*1000, ratio)

	if ratio < 2 {
		t.Errorf("现代步枪穿深至少应为床弩2倍以上, 得%.1fx", ratio)
	}
}

func TestArmorTypeKeys_ReturnsAll(t *testing.T) {
	a := setupTestAnalyzer()
	keys := a.ArmorTypeKeys()

	if len(keys) != 4 {
		t.Errorf("应有4种铠甲类型, 得%d", len(keys))
	}
	hasLeather := false
	for _, k := range keys {
		if k == "leather" {
			hasLeather = true
		}
	}
	if !hasLeather {
		t.Error("应包含leather类型")
	}
	t.Logf("铠甲类型列表: %v", keys)
}
