package vr_crossbow

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"ballistics-system/backend/config"
	"ballistics-system/backend/models"
)

type ClickHouseStore interface {
	InsertSimulationResult(ctx context.Context, result *models.SimulationResult) error
}

type SimulatorEngine interface {
	SolveElevationWithWind(distance, height, velocity, arrowMass, arrowDiameter, arrowLength, spinRate, windSpeed, windDirDeg float64) (float64, float64, *models.SimulationResult)
	RunSimWithWindDirect(params *models.SimulationParams, targetDist, targetHeight, windX, windZ float64) (float64, float64, float64, float64, float64, float64)
}

type PenetrationAnalyzer interface {
	AnalyzeWithSpin(impactVelocity, arrowMass, arrowDiameter, arrowLength, spinRate float64, armorType, arrowHeadType string, obliquityDeg float64) *models.PenetrationResult
}

type VRCrossbow struct {
	dynamicsCfg *config.DynamicsConfig
	simEngine   SimulatorEngine
	penAnalyzer PenetrationAnalyzer
	store       ClickHouseStore
	randSrc     *rand.Rand
}

func NewVRCrossbow(dynamicsCfg *config.DynamicsConfig, simEngine SimulatorEngine, penAnalyzer PenetrationAnalyzer, store ClickHouseStore) *VRCrossbow {
	return &VRCrossbow{
		dynamicsCfg: dynamicsCfg,
		simEngine:   simEngine,
		penAnalyzer: penAnalyzer,
		store:       store,
		randSrc:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (vr *VRCrossbow) NewVRCrossbowWithSeed(dynamicsCfg *config.DynamicsConfig, simEngine SimulatorEngine, penAnalyzer PenetrationAnalyzer, store ClickHouseStore, seed int64) *VRCrossbow {
	return &VRCrossbow{
		dynamicsCfg: dynamicsCfg,
		simEngine:   simEngine,
		penAnalyzer: penAnalyzer,
		store:       store,
		randSrc:     rand.New(rand.NewSource(seed)),
	}
}

func (vr *VRCrossbow) randGaussian(mean, stddev float64) float64 {
	u1 := float64(vr.randSrc.Int63()) / (1 << 63)
	u2 := float64(vr.randSrc.Int63()) / (1 << 63)
	if u1 < 1e-12 {
		u1 = 1e-12
	}
	z := math.Sqrt(-2.0*math.Log(u1)) * math.Cos(2.0*math.Pi*u2)
	return mean + stddev*z
}

func (vr *VRCrossbow) ListAimTargets() []models.AimTargetPreset {
	return []models.AimTargetPreset{
		{ID: "training", Name: "训练靶 (50m)", Distance: 50, Height: 1.5, ArmorType: "leather", Difficulty: "easy", Points: 10, Icon: "🎯"},
		{ID: "soldier", Name: "敌军步兵 (200m)", Distance: 200, Height: 1.7, ArmorType: "lamellar", Difficulty: "medium", Points: 50, Icon: "🛡️"},
		{ID: "rider", Name: "敌方骑兵 (350m)", Distance: 350, Height: 2.0, ArmorType: "mail", Difficulty: "hard", Points: 100, Icon: "🐴"},
		{ID: "gate", Name: "城门木盾 (500m)", Distance: 500, Height: 3.0, ArmorType: "leather", Difficulty: "hard", Points: 150, Icon: "🚪"},
		{ID: "tower", Name: "瞭望塔守卫 (650m)", Distance: 650, Height: 8.0, ArmorType: "lamellar", Difficulty: "expert", Points: 250, Icon: "🏰"},
		{ID: "commander", Name: "敌将 (800m)", Distance: 800, Height: 1.7, ArmorType: "plate", Difficulty: "legendary", Points: 500, Icon: "👑"},
	}
}

func (vr *VRCrossbow) AimShoot(req *models.AimShootRequest) (*models.AimShootResponse, error) {
	if req.CrossbowType == "" {
		req.CrossbowType = "bed_crossbow_triple"
	}
	if req.ArrowType == "" {
		req.ArrowType = "bodkin"
	}
	if req.Target.Distance <= 0 {
		req.Target.Distance = 200
	}
	if req.OperatorSkill <= 0 {
		req.OperatorSkill = 0.6
	}
	if req.OperatorSkill > 1.0 {
		req.OperatorSkill = 1.0
	}

	cfg, ok := vr.dynamicsCfg.CrossbowTypes[req.CrossbowType]
	if !ok {
		cfg = vr.dynamicsCfg.CrossbowTypes["bed_crossbow_triple"]
	}

	requiredElev, requiredAzimuth, simResult := vr.simEngine.SolveElevationWithWind(
		req.Target.Distance,
		req.Target.Height,
		cfg.TypicalVelocity,
		cfg.ArrowMass,
		cfg.ArrowDiameter,
		cfg.ArrowLength,
		cfg.SpinRate,
		req.WindSpeed,
		req.WindDir,
	)

	useElev := requiredElev
	useAzi := requiredAzimuth
	elevErrorDeg := 0.0
	aziErrorDeg := 0.0

	if !req.CalibrationRun {
		baseElevStd := 1.8 - req.OperatorSkill*1.4
		baseAziStd := 2.2 - req.OperatorSkill*1.7
		if baseElevStd < 0.05 {
			baseElevStd = 0.05
		}
		if baseAziStd < 0.06 {
			baseAziStd = 0.06
		}
		distFactor := 1.0 + req.Target.Distance/800.0
		elevErrorDeg = vr.randGaussian(0, baseElevStd*distFactor)
		aziErrorDeg = vr.randGaussian(0, baseAziStd*distFactor)

		if req.UserElevation != 0 {
			useElev = req.UserElevation + elevErrorDeg
		} else {
			useElev = requiredElev + elevErrorDeg
		}
		if req.UserAzimuth != 0 {
			useAzi = req.UserAzimuth + aziErrorDeg
		} else {
			useAzi = requiredAzimuth + aziErrorDeg
		}
	}

	actualRange := simResult.Range
	lateral := 0.0
	if simResult != nil {
		lateral = simResult.DriftLateral
	}
	if req.UserElevation != 0 || req.UserAzimuth != 0 || !req.CalibrationRun {
		elevSim := useElev
		aziSim := useAzi
		windDirRad := req.WindDir * math.Pi / 180.0
		windX := req.WindSpeed * math.Cos(windDirRad)
		windZ := req.WindSpeed * math.Sin(windDirRad)
		params := &models.SimulationParams{
			InitialVelocity: cfg.TypicalVelocity,
			LaunchAngle:     elevSim,
			AzimuthAngle:    aziSim,
			ArrowMass:       cfg.ArrowMass,
			ArrowDiameter:   cfg.ArrowDiameter,
			ArrowLength:     cfg.ArrowLength,
			SpinRate:        cfg.SpinRate,
			AirDensity:      1.225,
			DragCoefficient: 0.4,
		}
		r, _, lat, ft, mh, iv := vr.simEngine.RunSimWithWindDirect(params, req.Target.Distance, req.Target.Height, windX, windZ)
		actualRange = r
		lateral = lat
		simResult = &models.SimulationResult{
			InitialVelocity: cfg.TypicalVelocity,
			LaunchAngle:     elevSim,
			FlightTime:      ft,
			MaxHeight:       mh,
			Range:           r,
			ImpactVelocity:  iv,
			KineticEnergy:   0.5 * cfg.ArrowMass * iv * iv,
			ImpactSpinRate:  cfg.SpinRate * math.Exp(-0.02*ft),
		}
	}

	targetRadius := 1.5 + req.Target.Distance*0.015
	if req.CalibrationRun {
		targetRadius = 0.1 + req.Target.Distance*0.003
	}
	heightTolerance := math.Max(0.8, req.Target.Height*0.6)

	distErr := math.Abs(actualRange - req.Target.Distance)
	latErr := math.Abs(lateral)
	combinedHorizErr := math.Sqrt(distErr*distErr + latErr*latErr)
	vertErr := 0.0
	if simResult != nil {
		vertErr = simResult.HeightError
	}

	hit := combinedHorizErr <= targetRadius && vertErr < heightTolerance
	hitQuality := "miss"
	centerRatio := combinedHorizErr / math.Max(0.1, targetRadius)
	if hit {
		if centerRatio < 0.15 && vertErr < heightTolerance*0.2 {
			hitQuality = "bullseye"
		} else if centerRatio < 0.4 {
			hitQuality = "excellent"
		} else if centerRatio < 0.7 {
			hitQuality = "good"
		} else {
			hitQuality = "marginal"
		}
	}

	armorType := req.Target.ArmorType
	if armorType == "" {
		armorType = "leather"
	}
	penResult := vr.penAnalyzer.AnalyzeWithSpin(
		simResult.ImpactVelocity,
		cfg.ArrowMass,
		cfg.ArrowDiameter,
		cfg.ArrowLength,
		simResult.ImpactSpinRate,
		armorType,
		req.ArrowType,
		0,
	)

	basePoints := map[string]int{
		"training": 10, "soldier": 50, "rider": 100, "gate": 150, "tower": 250, "commander": 500,
	}
	distanceToID := map[float64]string{
		50: "training", 200: "soldier", 350: "rider", 500: "gate", 650: "tower", 800: "commander",
	}

	targetID := ""
	if req.Target.Name != "" {
		if id, ok := distanceToID[req.Target.Distance]; ok {
			targetID = id
		}
	}
	if targetID == "" {
		if req.Target.Distance <= 80 {
			targetID = "training"
		} else if req.Target.Distance <= 250 {
			targetID = "soldier"
		} else if req.Target.Distance <= 400 {
			targetID = "rider"
		} else if req.Target.Distance <= 570 {
			targetID = "gate"
		} else if req.Target.Distance <= 720 {
			targetID = "tower"
		} else {
			targetID = "commander"
		}
	}
	maxScore := basePoints[targetID]

	score := 0
	message := ""
	if req.CalibrationRun {
		score = 0
		message = fmt.Sprintf("校准射击: 理想仰角=%.2f°, 方位角=%.2f°, 射程误差=%.2fm",
			requiredElev, requiredAzimuth, distErr)
	} else if hit && penResult.Success {
		qualityBonus := map[string]int{"bullseye": 100, "excellent": 70, "good": 40, "marginal": 15}[hitQuality]
		score = maxScore + qualityBonus
		if hitQuality == "bullseye" {
			message = "正中靶心！完全穿透目标！"
		} else if hitQuality == "excellent" {
			message = "精准命中！完全穿透铠甲。"
		} else {
			message = "命中并穿透目标。"
		}
	} else if hit && !penResult.Success {
		score = int(float64(maxScore) * 0.35)
		message = "命中目标，但未能穿透铠甲。"
	} else if distErr < targetRadius*2.5 {
		score = int(float64(maxScore) * 0.1)
		if score < 1 {
			score = 1
		}
		message = fmt.Sprintf("射偏 %.2fm，接近目标。", combinedHorizErr)
	} else {
		message = fmt.Sprintf("未命中。距离偏差 %.2fm，横向偏差 %.2fm。", distErr, latErr)
	}

	aziRad := useAzi * math.Pi / 180.0
	impactX := actualRange * math.Cos(aziRad)
	impactY := actualRange * math.Sin(aziRad)
	windDriftX := impactX - req.Target.Distance
	windDriftY := impactY

	resp := &models.AimShootResponse{
		Success:           true,
		Hit:               hit,
		HitQuality:        hitQuality,
		RequiredElevation: requiredElev,
		RequiredAzimuth:   requiredAzimuth,
		ActualRange:       actualRange,
		FlightTime:        simResult.FlightTime,
		MaxHeight:         simResult.MaxHeight,
		ImpactVelocity:    simResult.ImpactVelocity,
		KineticEnergy:     simResult.KineticEnergy,
		ImpactX:           impactX,
		ImpactY:           impactY,
		ImpactZ:           -req.Target.Height + vertErr,
		WindDriftX:        windDriftX,
		WindDriftY:        windDriftY + lateral,
		RangeErrorM:       distErr,
		HeightErrorM:      vertErr,
		LateralErrorM:     latErr,
		TargetToleranceM:  targetRadius,
		PenetrationDepth:  penResult.PenetrationDepth * 1000,
		PenetrationSuccess: penResult.Success,
		Trajectory:        simResult.Trajectory,
		Message:           message,
		Score:             score,
		MaxPossibleScore:  maxScore,
		OperatorAppliedErrorElev: elevErrorDeg,
		OperatorAppliedErrorAzi:  aziErrorDeg,
	}

	if vr.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		simResult.DeviceID = "aim-game-" + req.CrossbowType
		simResult.ArmorType = armorType
		simResult.PenetrationDepth = penResult.PenetrationDepth
		simResult.PenetrationSuccess = penResult.Success
		_ = vr.store.InsertSimulationResult(ctx, simResult)
	}

	return resp, nil
}
