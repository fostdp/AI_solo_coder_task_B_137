package salvo_optimizer

import (
	"testing"

	"ballistics-system/backend/config"
	"ballistics-system/backend/models"
)

type mockSimEngine struct{}

func (m *mockSimEngine) Simulate(params *models.SimulationParams) *models.SimulationResult {
	return &models.SimulationResult{
		InitialVelocity: params.InitialVelocity,
		LaunchAngle:     params.LaunchAngle,
		FlightTime:      5.0,
		MaxHeight:       50.0,
		Range:           200.0,
		ImpactVelocity:  80.0,
		KineticEnergy:   500.0,
	}
}

func (m *mockSimEngine) SolveElevationForDistance(distance, velocity, arrowMass, arrowDiameter, arrowLength, spinRate float64) (float64, *models.SimulationResult) {
	return 45.0, &models.SimulationResult{
		InitialVelocity: velocity,
		LaunchAngle:     45.0,
		FlightTime:      5.0,
		Range:           distance,
		KineticEnergy:   500.0,
	}
}

func setupTestSO() (*SalvoOptimizer, map[string]config.CrossbowTypeConfig) {
	dynCfg := &config.DynamicsConfig{
		Simulation: config.SimulationConfig{
			Gravity:       9.81,
			AirDensitySea: 1.225,
			MaxSimTime:    60.0,
		},
		Defaults: config.DefaultsConfig{
			DragCoefficient: 0.3,
		},
	}
	engine := &mockSimEngine{}
	so := NewSalvoOptimizer(dynCfg, engine)

	crossbowConfigs := map[string]config.CrossbowTypeConfig{
		"bed_crossbow_triple": {
			Type:            "bed_crossbow_triple",
			Name:            "三弓床弩",
			DrawForce:       5500,
			ArrowMass:       0.2,
			ArrowDiameter:   0.015,
			ArrowLength:     0.9,
			TypicalVelocity: 135,
			SpinRate:        50.0,
			DrawLength:      0.6,
			BowEfficiency:   0.7,
			CrewSize:        3,
			ReloadSeconds:   30,
		},
		"bed_crossbow_single": {
			Type:            "bed_crossbow_single",
			Name:            "单弓床弩",
			DrawForce:       2500,
			ArrowMass:       0.15,
			ArrowDiameter:   0.012,
			ArrowLength:     0.7,
			TypicalVelocity: 110,
			SpinRate:        40.0,
			DrawLength:      0.5,
			BowEfficiency:   0.65,
			CrewSize:        2,
			ReloadSeconds:   20,
		},
	}

	return so, crossbowConfigs
}

func TestOptimize_BasicBarrage(t *testing.T) {
	so, cbCfgs := setupTestSO()
	req := &models.BarrageOptimizationRequest{
		Crossbows: []models.BarrageCrossbow{
			{ID: "cb1", Type: "bed_crossbow_triple", Name: "弩1", X: -20, Y: 0, Heading: 0, Elevation: 35},
			{ID: "cb2", Type: "bed_crossbow_triple", Name: "弩2", X: 0, Y: 0, Heading: 0, Elevation: 35},
			{ID: "cb3", Type: "bed_crossbow_triple", Name: "弩3", X: 20, Y: 0, Heading: 0, Elevation: 35},
		},
		Target:                    models.BarrageTarget{X: 200, Y: 200, Radius: 50},
		MaxShotsPerCrossbow:       2,
		SpreadAngle:               8.0,
		EnableCollisionAvoidance:  true,
		FireDelayBaseMs:           120.0,
		SafetySeparationM:         3.0,
	}
	resp := so.Optimize(req, cbCfgs)
	if resp.TotalShots != 6 {
		t.Errorf("TotalShots = %d, want 6", resp.TotalShots)
	}
}

func TestOptimize_DefaultCrossbows(t *testing.T) {
	so, cbCfgs := setupTestSO()
	req := &models.BarrageOptimizationRequest{
		Crossbows:           []models.BarrageCrossbow{},
		Target:              models.BarrageTarget{X: 200, Y: 200, Radius: 50},
		MaxShotsPerCrossbow: 2,
		SpreadAngle:         8.0,
	}
	resp := so.Optimize(req, cbCfgs)
	if resp.TotalShots != 6 {
		t.Errorf("TotalShots = %d, want 6 (3 default crossbows x 2 shots)", resp.TotalShots)
	}
}

func TestOptimize_DefaultShotsPerCrossbow(t *testing.T) {
	so, cbCfgs := setupTestSO()
	req := &models.BarrageOptimizationRequest{
		Crossbows: []models.BarrageCrossbow{
			{ID: "cb1", Type: "bed_crossbow_triple", Name: "弩1", X: 0, Y: 0, Heading: 0, Elevation: 35},
		},
		Target:              models.BarrageTarget{X: 200, Y: 200, Radius: 50},
		MaxShotsPerCrossbow: 0,
		SpreadAngle:         8.0,
	}
	resp := so.Optimize(req, cbCfgs)
	if resp.TotalShots != 2 {
		t.Errorf("TotalShots = %d, want 2 (default MaxShotsPerCrossbow)", resp.TotalShots)
	}
}

func TestOptimize_DefaultSpreadAngle(t *testing.T) {
	so, cbCfgs := setupTestSO()
	req := &models.BarrageOptimizationRequest{
		Crossbows: []models.BarrageCrossbow{
			{ID: "cb1", Type: "bed_crossbow_triple", Name: "弩1", X: 0, Y: 0, Heading: 0, Elevation: 35},
		},
		Target:              models.BarrageTarget{X: 200, Y: 200, Radius: 50},
		MaxShotsPerCrossbow: 2,
		SpreadAngle:         0,
	}
	resp := so.Optimize(req, cbCfgs)
	if resp.TotalShots != 2 {
		t.Errorf("TotalShots = %d, want 2", resp.TotalShots)
	}
}

func TestOptimize_WorkerCount(t *testing.T) {
	dynCfg := &config.DynamicsConfig{
		Simulation: config.SimulationConfig{
			Gravity:       9.81,
			AirDensitySea: 1.225,
			MaxSimTime:    60.0,
		},
		Defaults: config.DefaultsConfig{
			DragCoefficient: 0.3,
		},
	}
	engine := &mockSimEngine{}

	so1 := NewSalvoOptimizerWithWorkers(dynCfg, engine, 0)
	so2 := NewSalvoOptimizerWithWorkers(dynCfg, engine, 20)

	cbCfgs := map[string]config.CrossbowTypeConfig{
		"bed_crossbow_triple": {
			Type:            "bed_crossbow_triple",
			Name:            "三弓床弩",
			DrawForce:       5500,
			ArrowMass:       0.2,
			ArrowDiameter:   0.015,
			ArrowLength:     0.9,
			TypicalVelocity: 135,
			SpinRate:        50.0,
		},
	}

	req := &models.BarrageOptimizationRequest{
		Crossbows: []models.BarrageCrossbow{
			{ID: "cb1", Type: "bed_crossbow_triple", Name: "弩1", X: 0, Y: 0, Heading: 0, Elevation: 35},
		},
		Target:              models.BarrageTarget{X: 200, Y: 200, Radius: 50},
		MaxShotsPerCrossbow: 2,
		SpreadAngle:         8.0,
	}

	resp1 := so1.Optimize(req, cbCfgs)
	if resp1.TotalShots != 2 {
		t.Errorf("workerCount clamped to 1: TotalShots = %d, want 2", resp1.TotalShots)
	}

	resp2 := so2.Optimize(req, cbCfgs)
	if resp2.TotalShots != 2 {
		t.Errorf("workerCount clamped to 16: TotalShots = %d, want 2", resp2.TotalShots)
	}
}

func TestOptimize_CollisionAvoidance(t *testing.T) {
	so, cbCfgs := setupTestSO()
	req := &models.BarrageOptimizationRequest{
		Crossbows: []models.BarrageCrossbow{
			{ID: "cb1", Type: "bed_crossbow_triple", Name: "弩1", X: -5, Y: 0, Heading: 0, Elevation: 35},
			{ID: "cb2", Type: "bed_crossbow_triple", Name: "弩2", X: 5, Y: 0, Heading: 0, Elevation: 35},
		},
		Target:                   models.BarrageTarget{X: 200, Y: 200, Radius: 50},
		MaxShotsPerCrossbow:      2,
		SpreadAngle:              8.0,
		EnableCollisionAvoidance: true,
		FireDelayBaseMs:          120.0,
		SafetySeparationM:        3.0,
	}
	resp := so.Optimize(req, cbCfgs)
	if resp.CollisionsDetected < 0 {
		t.Errorf("CollisionsDetected = %d, want >= 0", resp.CollisionsDetected)
	}
	if resp.SeparationWarnings < 0 {
		t.Errorf("SeparationWarnings = %d, want >= 0", resp.SeparationWarnings)
	}
}

func TestOptimize_TargetHitRate(t *testing.T) {
	so, cbCfgs := setupTestSO()
	req := &models.BarrageOptimizationRequest{
		Crossbows: []models.BarrageCrossbow{
			{ID: "cb1", Type: "bed_crossbow_triple", Name: "弩1", X: 0, Y: 0, Heading: 0, Elevation: 35},
		},
		Target:              models.BarrageTarget{X: 200, Y: 200, Radius: 50},
		MaxShotsPerCrossbow: 2,
		SpreadAngle:         8.0,
	}
	resp := so.Optimize(req, cbCfgs)
	if resp.TargetHitRate < 0 || resp.TargetHitRate > 1 {
		t.Errorf("TargetHitRate = %f, want in [0, 1]", resp.TargetHitRate)
	}
}

func TestOptimize_AreaCovered(t *testing.T) {
	so, cbCfgs := setupTestSO()
	req := &models.BarrageOptimizationRequest{
		Crossbows: []models.BarrageCrossbow{
			{ID: "cb1", Type: "bed_crossbow_triple", Name: "弩1", X: 0, Y: 0, Heading: 0, Elevation: 35},
		},
		Target:              models.BarrageTarget{X: 200, Y: 200, Radius: 50},
		MaxShotsPerCrossbow: 2,
		SpreadAngle:         8.0,
	}
	resp := so.Optimize(req, cbCfgs)
	if resp.AreaCoveredM2 < 0 {
		t.Errorf("AreaCoveredM2 = %f, want >= 0", resp.AreaCoveredM2)
	}
}

func TestOptimize_ShotFields(t *testing.T) {
	so, cbCfgs := setupTestSO()
	req := &models.BarrageOptimizationRequest{
		Crossbows: []models.BarrageCrossbow{
			{ID: "cb1", Type: "bed_crossbow_triple", Name: "弩1", X: 0, Y: 0, Heading: 0, Elevation: 35},
		},
		Target:              models.BarrageTarget{X: 200, Y: 200, Radius: 50},
		MaxShotsPerCrossbow: 2,
		SpreadAngle:         8.0,
	}
	resp := so.Optimize(req, cbCfgs)
	for i, shot := range resp.Shots {
		if shot.CrossbowID == "" {
			t.Errorf("shot[%d].CrossbowID is empty", i)
		}
		if shot.ArrivalTime <= 0 {
			t.Errorf("shot[%d].ArrivalTime = %f, want > 0", i, shot.ArrivalTime)
		}
		if shot.FlightTime <= 0 {
			t.Errorf("shot[%d].FlightTime = %f, want > 0", i, shot.FlightTime)
		}
	}
}
