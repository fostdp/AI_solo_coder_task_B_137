package power_comparator

import (
	"testing"

	"ballistics-system/backend/config"
	"ballistics-system/backend/models"
)

type mockSimEngine struct {
	lastLaunchAngle  float64
	lastAzimuthAngle float64
}

func (m *mockSimEngine) SimulateWithCrossbowType(crossbowType string, crossbowCfg *config.CrossbowTypeConfig, launchAngle, azimuthAngle float64) *models.SimulationResult {
	m.lastLaunchAngle = launchAngle
	m.lastAzimuthAngle = azimuthAngle
	v := crossbowCfg.TypicalVelocity
	ke := 0.5 * crossbowCfg.ArrowMass * v * v
	return &models.SimulationResult{
		InitialVelocity: v,
		ImpactVelocity:  v * 0.85,
		KineticEnergy:   ke * 0.72,
		Range:           crossbowCfg.TypicalRange,
		FlightTime:      crossbowCfg.TypicalRange / v,
		MaxHeight:       crossbowCfg.TypicalRange * 0.15,
		ImpactSpinRate:  crossbowCfg.SpinRate * 0.9,
		ImpactGyroStab:  0.8,
	}
}

type mockPenAnalyzer struct {
	lastArrowHeadType string
}

func (m *mockPenAnalyzer) ArmorTypeKeys() []string {
	return []string{"lamellar", "chainmail", "plate"}
}

func (m *mockPenAnalyzer) AnalyzeWithSpin(impactVelocity, arrowMass, arrowDiameter, arrowLength, spinRate float64, armorType, arrowHeadType string, obliquityDeg float64) *models.PenetrationResult {
	m.lastArrowHeadType = arrowHeadType
	return &models.PenetrationResult{
		PenetrationDepth: 5.0,
		Success:          impactVelocity > 50,
	}
}

func setupTestPC() (*PowerComparator, *mockSimEngine, *mockPenAnalyzer) {
	dynCfg := &config.DynamicsConfig{
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
		},
	}
	sim := &mockSimEngine{}
	pen := &mockPenAnalyzer{}
	pc := NewPowerComparator(dynCfg, sim, pen)
	return pc, sim, pen
}

func TestListCrossbows_ReturnsAllTypes(t *testing.T) {
	pc, _, _ := setupTestPC()
	list := pc.ListCrossbows()
	if len(list) != 3 {
		t.Fatalf("期望3种弩机, 得到%d", len(list))
	}
}

func TestListCrossbows_SortedByDrawForce(t *testing.T) {
	pc, _, _ := setupTestPC()
	list := pc.ListCrossbows()
	for i := 1; i < len(list); i++ {
		prev := list[i-1]["draw_force_n"].(float64)
		curr := list[i]["draw_force_n"].(float64)
		if curr <= prev {
			t.Errorf("拉力未升序排列: 索引%d=%.0f, 索引%d=%.0f", i-1, prev, i, curr)
		}
	}
}

func TestCompare_AllTypes(t *testing.T) {
	pc, _, _ := setupTestPC()
	resp := pc.Compare(nil, "bodkin", 45)
	if len(resp.Crossbows) != 3 {
		t.Fatalf("期望3个对比结果, 得到%d", len(resp.Crossbows))
	}
	for i := 1; i < len(resp.Crossbows); i++ {
		if resp.Crossbows[i].PowerIndex >= resp.Crossbows[i-1].PowerIndex {
			t.Errorf("PowerIndex应降序: 索引%d=%.2f, 索引%d=%.2f",
				i-1, resp.Crossbows[i-1].PowerIndex, i, resp.Crossbows[i].PowerIndex)
		}
	}
}

func TestCompare_SpecificTypes(t *testing.T) {
	pc, _, _ := setupTestPC()
	resp := pc.Compare([]string{"arm_stretched", "bed_crossbow_triple"}, "bodkin", 45)
	if len(resp.Crossbows) != 2 {
		t.Fatalf("期望2个对比结果, 得到%d", len(resp.Crossbows))
	}
	types := map[string]bool{}
	for _, item := range resp.Crossbows {
		types[item.CrossbowType] = true
	}
	if !types["arm_stretched"] || !types["bed_crossbow_triple"] {
		t.Error("结果应包含arm_stretched和bed_crossbow_triple")
	}
	if types["leg_stretched"] {
		t.Error("结果不应包含leg_stretched")
	}
}

func TestCompare_DefaultAngle(t *testing.T) {
	pc, sim, _ := setupTestPC()
	pc.Compare(nil, "bodkin", 0)
	if sim.lastLaunchAngle != 45.0 {
		t.Errorf("launchAngle=0时应默认45°, 得到%f", sim.lastLaunchAngle)
	}
}

func TestCompare_DefaultArrowHead(t *testing.T) {
	pc, _, pen := setupTestPC()
	pc.Compare(nil, "", 45)
	if pen.lastArrowHeadType != "bodkin" {
		t.Errorf("arrowHeadType为空时应默认bodkin, 得到%s", pen.lastArrowHeadType)
	}
}

func TestCompare_EmptyResult_FilteredOut(t *testing.T) {
	pc, _, _ := setupTestPC()
	resp := pc.Compare([]string{"nonexistent_type"}, "bodkin", 45)
	if len(resp.Crossbows) != 0 {
		t.Errorf("不存在的类型应返回空结果, 得到%d项", len(resp.Crossbows))
	}
}

func TestCompare_PowerIndexMonotonic(t *testing.T) {
	pc, _, _ := setupTestPC()
	resp := pc.Compare(nil, "bodkin", 45)
	for i := 1; i < len(resp.Crossbows); i++ {
		if resp.Crossbows[i].KineticEnergy >= resp.Crossbows[i-1].KineticEnergy {
			t.Errorf("KE应随PowerIndex降序递减: 索引%d KE=%.1f, 索引%d KE=%.1f",
				i-1, resp.Crossbows[i-1].KineticEnergy, i, resp.Crossbows[i].KineticEnergy)
		}
		if resp.Crossbows[i].PowerIndex >= resp.Crossbows[i-1].PowerIndex {
			t.Errorf("PowerIndex应单调递减: 索引%d=%.2f, 索引%d=%.2f",
				i-1, resp.Crossbows[i-1].PowerIndex, i, resp.Crossbows[i].PowerIndex)
		}
	}
}
