package era_comparator

import (
	"math"
	"testing"

	"ballistics-system/backend/config"
	"ballistics-system/backend/models"
)

type mockSimEngine struct{}

func (m *mockSimEngine) SolveElevationForDistance(distance, velocity, arrowMass, arrowDiameter, arrowLength, spinRate float64) (float64, *models.SimulationResult) {
	impactVel := velocity * 0.8
	return 35.0, &models.SimulationResult{
		ImpactVelocity: impactVel,
		KineticEnergy:  0.5 * arrowMass * impactVel * impactVel,
		ImpactSpinRate: spinRate * 0.9,
		Range:          distance,
	}
}

type mockPenAnalyzer struct{}

func (m *mockPenAnalyzer) ArmorTypeKeys() []string {
	return []string{"leather", "plate"}
}

func (m *mockPenAnalyzer) AnalyzeWithSpin(impactVelocity, arrowMass, arrowDiameter, arrowLength, spinRate float64, armorType, arrowHeadType string, obliquityDeg float64) *models.PenetrationResult {
	depth := 0.005
	success := true
	if armorType == "plate" {
		depth = 0.002
		success = impactVelocity > 100
	}
	return &models.PenetrationResult{
		ArmorType:        armorType,
		ImpactVelocity:   impactVelocity,
		ArrowMass:        arrowMass,
		ArrowHeadType:    arrowHeadType,
		PenetrationDepth: depth,
		Success:          success,
	}
}

func (m *mockPenAnalyzer) AnalyzeModernBullet(impactVelocity float64, weaponCfg *config.ModernWeaponConfig, armorType string, obliquityDeg float64) *models.PenetrationResult {
	return &models.PenetrationResult{
		ArmorType:        armorType,
		ImpactVelocity:   impactVelocity,
		PenetrationDepth: 0.02,
		Success:          true,
	}
}

func setupTestEC() *EraComparator {
	dynCfg := &config.DynamicsConfig{
		Simulation: config.SimulationConfig{
			Gravity:       9.80665,
			AirDensitySea: 1.225,
		},
		CrossbowTypes: map[string]config.CrossbowTypeConfig{
			"bed_crossbow_triple": {
				Type: "bed_crossbow_triple", Name: "三弓床弩", Description: "宋代三弓床弩",
				Era: "古代", DrawForce: 5500, DrawLength: 1.2,
				ArrowMass: 0.2, ArrowLength: 1.0, ArrowDiameter: 0.012,
				TypicalVelocity: 135, TypicalRange: 800, SpinRate: 25,
				BowEfficiency: 0.68, CrewSize: 7, ReloadSeconds: 90,
			},
			"arm_stretched": {
				Type: "arm_stretched", Name: "臂张弩", Description: "臂张弩",
				Era: "古代", DrawForce: 350, DrawLength: 0.45,
				ArrowMass: 0.035, ArrowLength: 0.42, ArrowDiameter: 0.008,
				TypicalVelocity: 65, TypicalRange: 180, SpinRate: 12,
				BowEfficiency: 0.58, CrewSize: 1, ReloadSeconds: 8,
			},
		},
		ModernWeapons: map[string]config.ModernWeaponConfig{
			"barrett_m82": {
				Type: "barrett_m82", Name: "巴雷特 M82A1", Description: "巴雷特M82反器材步枪",
				Era: "现代", Standard: "NATO",
				BulletMass: 0.042, BulletDiameter: 0.0127, BulletLength: 0.058,
				MuzzleVelocity: 853, MaxRange: 1800, EffectiveRange: 1500,
				DragCoefficient: 0.295, SpinRate: 1800, Hardness: 650,
				TipArea: 1.267e-4, CrewSize: 1, ReloadSeconds: 3,
			},
			"ntw_20": {
				Type: "ntw_20", Name: "NTW-20 20mm", Description: "NTW-20 20mm反器材步枪",
				Era: "现代", Standard: "20mm",
				BulletMass: 0.125, BulletDiameter: 0.02, BulletLength: 0.11,
				MuzzleVelocity: 720, MaxRange: 1600, EffectiveRange: 1300,
				DragCoefficient: 0.32, SpinRate: 1200, Hardness: 680,
				TipArea: 3.142e-4, CrewSize: 2, ReloadSeconds: 5,
			},
		},
	}
	return NewEraComparator(dynCfg, &mockSimEngine{}, &mockPenAnalyzer{})
}

func TestListModernWeapons_ReturnsAll(t *testing.T) {
	ec := setupTestEC()
	weapons := ec.ListModernWeapons()
	if len(weapons) != 2 {
		t.Fatalf("应返回2个现代武器, 得%d", len(weapons))
	}
	types := make(map[string]bool)
	for _, w := range weapons {
		if v, ok := w["type"].(string); ok {
			types[v] = true
		}
	}
	if !types["barrett_m82"] {
		t.Error("应包含barrett_m82")
	}
	if !types["ntw_20"] {
		t.Error("应包含ntw_20")
	}
}

func TestCompare_AllWeapons(t *testing.T) {
	ec := setupTestEC()
	resp := ec.Compare(nil, nil, "bodkin", 1000)
	if resp == nil {
		t.Fatal("Compare不应返回nil")
	}
	ancientCount := 0
	modernCount := 0
	for _, w := range resp.Weapons {
		if w.IsModern {
			modernCount++
		} else {
			ancientCount++
		}
	}
	if ancientCount != 2 {
		t.Errorf("应包含2个古代弩机, 得%d", ancientCount)
	}
	if modernCount != 2 {
		t.Errorf("应包含2个现代武器, 得%d", modernCount)
	}
	if len(resp.ArmorTypes) != 2 {
		t.Errorf("应包含2种铠甲类型, 得%d", len(resp.ArmorTypes))
	}
}

func TestCompare_FilterCrossbows(t *testing.T) {
	ec := setupTestEC()
	resp := ec.Compare([]string{"bed_crossbow_triple"}, nil, "bodkin", 1000)
	crossbowNames := make(map[string]bool)
	for _, w := range resp.Weapons {
		if !w.IsModern {
			crossbowNames[w.WeaponType] = true
		}
	}
	if len(crossbowNames) != 1 {
		t.Errorf("应只包含1个弩机类型, 得%d", len(crossbowNames))
	}
	if !crossbowNames["bed_crossbow_triple"] {
		t.Error("应包含bed_crossbow_triple")
	}
	if crossbowNames["arm_stretched"] {
		t.Error("不应包含arm_stretched")
	}
}

func TestCompare_FilterModern(t *testing.T) {
	ec := setupTestEC()
	resp := ec.Compare(nil, []string{"barrett_m82"}, "bodkin", 1000)
	modernNames := make(map[string]bool)
	for _, w := range resp.Weapons {
		if w.IsModern {
			modernNames[w.WeaponType] = true
		}
	}
	if len(modernNames) != 1 {
		t.Errorf("应只包含1个现代武器类型, 得%d", len(modernNames))
	}
	if !modernNames["barrett_m82"] {
		t.Error("应包含barrett_m82")
	}
	if modernNames["ntw_20"] {
		t.Error("不应包含ntw_20")
	}
}

func TestCompare_DefaultRange(t *testing.T) {
	ec := setupTestEC()
	resp0 := ec.Compare(nil, nil, "bodkin", 0)
	resp1000 := ec.Compare(nil, nil, "bodkin", 1000)
	if len(resp0.Weapons) != len(resp1000.Weapons) {
		t.Fatalf("武器数量应一致: default=%d, 1000=%d", len(resp0.Weapons), len(resp1000.Weapons))
	}
	for i := range resp0.Weapons {
		if resp0.Weapons[i].WeaponType != resp1000.Weapons[i].WeaponType {
			continue
		}
		if math.Abs(resp0.Weapons[i].ImpactVelocity-resp1000.Weapons[i].ImpactVelocity) > 1e-6 {
			t.Errorf("compareRange=0应默认1000: %s ImpactVelocity default=%.2f, 1000=%.2f",
				resp0.Weapons[i].WeaponType, resp0.Weapons[i].ImpactVelocity, resp1000.Weapons[i].ImpactVelocity)
		}
		if math.Abs(resp0.Weapons[i].ImpactKE-resp1000.Weapons[i].ImpactKE) > 1e-6 {
			t.Errorf("compareRange=0应默认1000: %s ImpactKE default=%.2f, 1000=%.2f",
				resp0.Weapons[i].WeaponType, resp0.Weapons[i].ImpactKE, resp1000.Weapons[i].ImpactKE)
		}
	}
}

func TestCompare_ModernHigherKE(t *testing.T) {
	ec := setupTestEC()
	resp := ec.Compare(nil, nil, "bodkin", 1000)
	var maxAncientKE float64
	var minModernKE float64 = math.MaxFloat64
	for _, w := range resp.Weapons {
		if w.IsModern {
			if w.ImpactKE < minModernKE {
				minModernKE = w.ImpactKE
			}
		} else {
			if w.ImpactKE > maxAncientKE {
				maxAncientKE = w.ImpactKE
			}
		}
	}
	if minModernKE <= maxAncientKE {
		t.Errorf("现代武器最小ImpactKE(%.1fJ)应高于古代弩机最大ImpactKE(%.1fJ)", minModernKE, maxAncientKE)
	}
}

func TestEstimateModernImpactVelocity(t *testing.T) {
	ec := setupTestEC()
	barrett := ec.dynamicsCfg.ModernWeapons["barrett_m82"]
	ntw := ec.dynamicsCfg.ModernWeapons["ntw_20"]
	cases := []struct {
		name string
		cfg  config.ModernWeaponConfig
		rng  float64
	}{
		{"barrett_500m", barrett, 500},
		{"barrett_1000m", barrett, 1000},
		{"ntw_500m", ntw, 500},
		{"ntw_1000m", ntw, 1000},
	}
	for _, c := range cases {
		v0 := c.cfg.MuzzleVelocity
		v := ec.estimateModernImpactVelocity(&c.cfg, c.rng)
		if v < v0*0.3 || v > v0*0.99 {
			t.Errorf("%s: 估算着速应在v0*0.3~v0*0.99范围内, v0=%.1f, v=%.1f, ratio=%.3f",
				c.name, v0, v, v/v0)
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Errorf("%s: 估算着速不应为NaN/Inf", c.name)
		}
		t.Logf("%s: v0=%.1f, impactVel=%.1f, ratio=%.3f", c.name, v0, v, v/v0)
	}
}

func TestCompare_PowerRatioWithBedCrossbow(t *testing.T) {
	ec := setupTestEC()
	resp := ec.Compare([]string{"bed_crossbow_triple"}, nil, "bodkin", 1000)
	for _, w := range resp.Weapons {
		if w.WeaponType == "bed_crossbow_triple" {
			if math.Abs(w.PowerRatio-1.0) > 0.01 {
				t.Errorf("三弓床弩PowerRatio应约为1.0, 得%.4f", w.PowerRatio)
			}
			t.Logf("三弓床弩: PowerRatio=%.4f, ImpactKE=%.1fJ", w.PowerRatio, w.ImpactKE)
			return
		}
	}
	t.Error("结果中未找到bed_crossbow_triple")
}
