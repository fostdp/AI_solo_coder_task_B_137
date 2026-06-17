package vr_crossbow

import (
	"context"
	"math"
	"strings"
	"testing"

	"ballistics-system/backend/config"
	"ballistics-system/backend/models"
)

type mockSimEngine struct{}

func (m *mockSimEngine) SolveElevationWithWind(distance, height, velocity, arrowMass, arrowDiameter, arrowLength, spinRate, windSpeed, windDirDeg float64) (float64, float64, *models.SimulationResult) {
	return 35.0, 0.0, &models.SimulationResult{
		InitialVelocity: velocity,
		LaunchAngle:     35.0,
		FlightTime:      3.5,
		MaxHeight:       50.0,
		Range:           200.0,
		ImpactVelocity:  120.0,
		KineticEnergy:   0.5 * arrowMass * 120.0 * 120.0,
		ImpactSpinRate:  spinRate,
		ImpactGyroStab:  0.9,
		DriftLateral:    0.5,
	}
}

func (m *mockSimEngine) RunSimWithWindDirect(params *models.SimulationParams, targetDist, targetHeight, windX, windZ float64) (float64, float64, float64, float64, float64, float64) {
	return 200.0, 3.5, 0.5, 50.0, 120.0, 0.0
}

type mockPenAnalyzer struct{}

func (m *mockPenAnalyzer) AnalyzeWithSpin(impactVelocity, arrowMass, arrowDiameter, arrowLength, spinRate float64, armorType, arrowHeadType string, obliquityDeg float64) *models.PenetrationResult {
	return &models.PenetrationResult{
		ArmorType:        armorType,
		ArmorThickness:   5.0,
		ImpactVelocity:   impactVelocity,
		ArrowMass:        arrowMass,
		ArrowHeadType:    arrowHeadType,
		PenetrationDepth: 3.5,
		ResidualVelocity: 60.0,
		EnergyAbsorbed:   500.0,
		Success:          true,
		ImpactSpinRate:   spinRate,
		GyroStability:    0.85,
		YawAngle:         2.0,
		EffectiveArea:    1e-4,
		StabilityFactor:  0.9,
	}
}

type mockStore struct{}

func (m *mockStore) InsertSimulationResult(ctx context.Context, result *models.SimulationResult) error {
	return nil
}

func setupTestVR() *VRCrossbow {
	dynamicsCfg := &config.DynamicsConfig{
		CrossbowTypes: map[string]config.CrossbowTypeConfig{
			"bed_crossbow_triple": {
				Type:            "bed_crossbow_triple",
				Name:            "三弓床弩",
				TypicalVelocity: 135.0,
				ArrowMass:       0.2,
				ArrowDiameter:   0.012,
				ArrowLength:     1.0,
				SpinRate:        11.0,
			},
		},
	}
	vr := &VRCrossbow{}
	return vr.NewVRCrossbowWithSeed(dynamicsCfg, &mockSimEngine{}, &mockPenAnalyzer{}, &mockStore{}, 42)
}

func TestListAimTargets_ReturnsPresets(t *testing.T) {
	vr := setupTestVR()
	targets := vr.ListAimTargets()
	if len(targets) != 6 {
		t.Errorf("expected 6 targets, got %d", len(targets))
	}
}

func TestListAimTargets_DifficultyLevels(t *testing.T) {
	vr := setupTestVR()
	targets := vr.ListAimTargets()
	difficulties := map[string]bool{}
	for _, tgt := range targets {
		difficulties[tgt.Difficulty] = true
	}
	for _, d := range []string{"easy", "medium", "hard", "expert", "legendary"} {
		if !difficulties[d] {
			t.Errorf("missing difficulty level: %s", d)
		}
	}
}

func TestAimShoot_CalibrationRun(t *testing.T) {
	vr := setupTestVR()
	req := &models.AimShootRequest{
		Target: models.AimTarget{
			Distance:  200,
			Height:    1.7,
			Name:      "敌军步兵",
			ArmorType: "lamellar",
		},
		CrossbowType:   "bed_crossbow_triple",
		ArrowType:      "bodkin",
		OperatorSkill:  0.6,
		CalibrationRun: true,
	}
	resp, err := vr.AimShoot(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Score != 0 {
		t.Errorf("calibration run score should be 0, got %d", resp.Score)
	}
	if !strings.Contains(resp.Message, "校准") {
		t.Errorf("calibration run message should contain '校准', got: %s", resp.Message)
	}
}

func TestAimShoot_DefaultCrossbowType(t *testing.T) {
	vr := setupTestVR()
	req := &models.AimShootRequest{
		Target: models.AimTarget{
			Distance:  200,
			Height:    1.7,
			Name:      "敌军步兵",
			ArmorType: "lamellar",
		},
		CrossbowType:  "",
		ArrowType:     "bodkin",
		OperatorSkill: 0.6,
	}
	resp, err := vr.AimShoot(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected Success=true with default crossbow type")
	}
}

func TestAimShoot_DefaultArrowType(t *testing.T) {
	vr := setupTestVR()
	req := &models.AimShootRequest{
		Target: models.AimTarget{
			Distance:  200,
			Height:    1.7,
			Name:      "敌军步兵",
			ArmorType: "lamellar",
		},
		CrossbowType:  "bed_crossbow_triple",
		ArrowType:     "",
		OperatorSkill: 0.6,
	}
	resp, err := vr.AimShoot(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected Success=true with default arrow type")
	}
}

func TestAimShoot_DefaultOperatorSkill(t *testing.T) {
	vr := setupTestVR()
	req := &models.AimShootRequest{
		Target: models.AimTarget{
			Distance:  200,
			Height:    1.7,
			Name:      "敌军步兵",
			ArmorType: "lamellar",
		},
		CrossbowType:  "bed_crossbow_triple",
		ArrowType:     "bodkin",
		OperatorSkill: 0,
	}
	resp, err := vr.AimShoot(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected Success=true with default operator skill")
	}
}

func TestAimShoot_OperatorSkillCapped(t *testing.T) {
	vr := setupTestVR()
	req := &models.AimShootRequest{
		Target: models.AimTarget{
			Distance:  200,
			Height:    1.7,
			Name:      "敌军步兵",
			ArmorType: "lamellar",
		},
		CrossbowType:  "bed_crossbow_triple",
		ArrowType:     "bodkin",
		OperatorSkill: 1.5,
	}
	resp, err := vr.AimShoot(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected Success=true with capped operator skill")
	}
}

func TestAimShoot_ResponseFields(t *testing.T) {
	vr := setupTestVR()
	req := &models.AimShootRequest{
		Target: models.AimTarget{
			Distance:  200,
			Height:    1.7,
			Name:      "敌军步兵",
			ArmorType: "lamellar",
		},
		CrossbowType:  "bed_crossbow_triple",
		ArrowType:     "bodkin",
		OperatorSkill: 0.6,
	}
	resp, err := vr.AimShoot(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Errorf("expected Success=true")
	}
	if resp.HitQuality == "" {
		t.Errorf("expected HitQuality to be non-empty")
	}
	if resp.RequiredElevation <= 0 {
		t.Errorf("expected RequiredElevation > 0, got %f", resp.RequiredElevation)
	}
}

func TestAimShoot_RandGaussian_Distribution(t *testing.T) {
	vr := setupTestVR()
	n := 10000
	sum := 0.0
	sumSq := 0.0
	for i := 0; i < n; i++ {
		v := vr.randGaussian(0, 1)
		sum += v
		sumSq += v * v
	}
	mean := sum / float64(n)
	stddev := math.Sqrt(sumSq/float64(n) - mean*mean)
	if math.Abs(mean) > 0.3 {
		t.Errorf("mean too far from 0: got %f, tolerance 0.3", mean)
	}
	if math.Abs(stddev-1.0) > 0.3 {
		t.Errorf("stddev too far from 1.0: got %f, tolerance 0.3", stddev)
	}
}

func TestAimShoot_ScoreRange(t *testing.T) {
	vr := setupTestVR()
	req := &models.AimShootRequest{
		Target: models.AimTarget{
			Distance:  200,
			Height:    1.7,
			Name:      "敌军步兵",
			ArmorType: "lamellar",
		},
		CrossbowType:  "bed_crossbow_triple",
		ArrowType:     "bodkin",
		OperatorSkill: 0.6,
	}
	resp, err := vr.AimShoot(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	upperBound := resp.MaxPossibleScore + 100
	if resp.Score < 0 || resp.Score > upperBound {
		t.Errorf("score %d not in range [0, %d]", resp.Score, upperBound)
	}
}
