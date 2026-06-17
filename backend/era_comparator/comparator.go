package era_comparator

import (
	"math"
	"sort"

	"ballistics-system/backend/config"
	"ballistics-system/backend/models"
)

type SimulatorEngine interface {
	SolveElevationForDistance(distance, velocity, arrowMass, arrowDiameter, arrowLength, spinRate float64) (float64, *models.SimulationResult)
}

type PenetrationAnalyzer interface {
	ArmorTypeKeys() []string
	AnalyzeWithSpin(impactVelocity, arrowMass, arrowDiameter, arrowLength, spinRate float64, armorType, arrowHeadType string, obliquityDeg float64) *models.PenetrationResult
	AnalyzeModernBullet(impactVelocity float64, weaponCfg *config.ModernWeaponConfig, armorType string, obliquityDeg float64) *models.PenetrationResult
}

type EraComparator struct {
	dynamicsCfg *config.DynamicsConfig
	simEngine   SimulatorEngine
	penAnalyzer PenetrationAnalyzer
}

func NewEraComparator(dynamicsCfg *config.DynamicsConfig, simEngine SimulatorEngine, penAnalyzer PenetrationAnalyzer) *EraComparator {
	return &EraComparator{
		dynamicsCfg: dynamicsCfg,
		simEngine:   simEngine,
		penAnalyzer: penAnalyzer,
	}
}

func (ec *EraComparator) ListModernWeapons() []map[string]interface{} {
	weapons := make([]map[string]interface{}, 0, len(ec.dynamicsCfg.ModernWeapons))
	for _, cfg := range ec.dynamicsCfg.ModernWeapons {
		weapons = append(weapons, map[string]interface{}{
			"type":                cfg.Type,
			"name":                cfg.Name,
			"description":         cfg.Description,
			"era":                 cfg.Era,
			"bullet_mass_kg":      cfg.BulletMass,
			"bullet_dia_mm":       cfg.BulletDiameter * 1000,
			"bore_dia_mm":         cfg.BulletDiameter * 1000,
			"bullet_length_mm":    cfg.BulletLength * 1000,
			"muzzle_velocity_ms":  cfg.MuzzleVelocity,
			"muzzle_energy_j":     cfg.MuzzleEnergy,
			"max_range_m":         cfg.MaxRange,
			"effective_range_point_m": cfg.EffectiveRangePoint,
			"effective_range_area_m":  cfg.EffectiveRangeArea,
			"drag_coef_stable":    cfg.DragCoefStable,
			"bc_g1":               cfg.BallisticCoefG1,
			"bc_g7":               cfg.BallisticCoefG7,
			"twist_rate":          cfg.TwistRate,
			"spin_rate_hz":        cfg.SpinRate,
			"jacket_hardness_hb":  cfg.JacketHardnessHB,
			"core_hardness_hb":    cfg.CoreHardnessHB,
			"average_hardness":    cfg.AverageHardness,
			"hardness_bhn":        cfg.Hardness,
			"tip_area_m2":         cfg.TipArea,
			"crew_size":           cfg.CrewSize,
			"reload_seconds":      cfg.ReloadSeconds,
			"cyclic_rate_rpm":     cfg.CyclicRateRPM,
			"cartridge":           cfg.Cartridge,
			"ammo_type":           cfg.AmmoType,
			"ammo_standard":       cfg.AmmoStandard,
			"standard":            cfg.Standard,
			"penetration_reference": cfg.PenetrationReference,
		})
	}
	return weapons
}

func (ec *EraComparator) estimateModernImpactVelocity(cfg *config.ModernWeaponConfig, compareRange float64) float64 {
	g := ec.dynamicsCfg.Simulation.Gravity
	v0 := cfg.MuzzleVelocity
	crossArea := math.Pi * math.Pow(cfg.BulletDiameter/2.0, 2)
	dragFactor := 0.5 * cfg.DragCoefficient * ec.dynamicsCfg.Simulation.AirDensitySea * crossArea / cfg.BulletMass

	vx := v0
	vy := 0.0
	x := 0.0
	estImpactVel := v0 * 0.5
	dt := 0.001
	for t := 0.0; t < 10.0; t += dt {
		v := math.Sqrt(vx*vx + vy*vy)
		if x >= compareRange {
			estImpactVel = v
			break
		}
		ax := -dragFactor * v * vx
		ay := -g - dragFactor*v*vy
		vx += ax * dt
		vy += ay * dt
		x += vx * dt
	}
	return estImpactVel
}

func (ec *EraComparator) Compare(crossbowTypes, modernTypes []string, arrowHeadType string, compareRange float64) *models.EraComparisonResponse {
	if compareRange == 0 {
		compareRange = 1000.0
	}
	if arrowHeadType == "" {
		arrowHeadType = "bodkin"
	}

	resultList := make([]models.WeaponEraComparison, 0)
	armorTypes := ec.penAnalyzer.ArmorTypeKeys()
	var bedCrossbowKE float64 = 0

	includeAllCrossbows := len(crossbowTypes) == 0
	crossbowSet := make(map[string]bool)
	for _, t := range crossbowTypes {
		crossbowSet[t] = true
	}

	for key, cfg := range ec.dynamicsCfg.CrossbowTypes {
		if !includeAllCrossbows && !crossbowSet[key] {
			continue
		}

		_, simResult := ec.simEngine.SolveElevationForDistance(
			compareRange, cfg.TypicalVelocity,
			cfg.ArrowMass, cfg.ArrowDiameter, cfg.ArrowLength, cfg.SpinRate,
		)

		pens := make(map[string]float64)
		penSuccess := make(map[string]bool)
		for _, armorKey := range armorTypes {
			pen := ec.penAnalyzer.AnalyzeWithSpin(
				simResult.ImpactVelocity,
				cfg.ArrowMass,
				cfg.ArrowDiameter,
				cfg.ArrowLength,
				simResult.ImpactSpinRate,
				armorKey,
				arrowHeadType,
				0,
			)
			pens[armorKey] = pen.PenetrationDepth * 1000
			penSuccess[armorKey] = pen.Success
		}

		muzzleKE := 0.5 * cfg.ArrowMass * cfg.TypicalVelocity * cfg.TypicalVelocity
		shotsPerMin := 60.0 / cfg.ReloadSeconds

		item := models.WeaponEraComparison{
			WeaponType:         cfg.Type,
			WeaponName:         cfg.Name,
			Description:        cfg.Description,
			Era:                cfg.Era,
			IsModern:           false,
			ProjectileMass:     cfg.ArrowMass,
			ProjectileDia:      cfg.ArrowDiameter,
			ProjectileLen:      cfg.ArrowLength,
			MuzzleVelocity:     cfg.TypicalVelocity,
			MaxRange:           cfg.TypicalRange,
			EffectiveRange:     cfg.TypicalRange * 0.7,
			SpinRate:           cfg.SpinRate,
			KineticEnergy:      muzzleKE,
			ImpactVelocity:     simResult.ImpactVelocity,
			ImpactKE:           simResult.KineticEnergy,
			Penetrations:       pens,
			PenetrationSuccess: penSuccess,
			CrewSize:           cfg.CrewSize,
			ReloadSeconds:      cfg.ReloadSeconds,
			ShotsPerMinute:     shotsPerMin,
			KEPerMinute:        muzzleKE * shotsPerMin,
		}
		if key == "bed_crossbow_triple" {
			bedCrossbowKE = simResult.KineticEnergy
		}
		resultList = append(resultList, item)
	}

	includeAllModern := len(modernTypes) == 0
	modernSet := make(map[string]bool)
	for _, t := range modernTypes {
		modernSet[t] = true
	}

	for key, cfg := range ec.dynamicsCfg.ModernWeapons {
		if !includeAllModern && !modernSet[key] {
			continue
		}

		estImpactVel := ec.estimateModernImpactVelocity(&cfg, compareRange)

		pens := make(map[string]float64)
		penSuccess := make(map[string]bool)
		for _, armorKey := range armorTypes {
			pen := ec.penAnalyzer.AnalyzeModernBullet(estImpactVel, &cfg, armorKey, 0)
			pens[armorKey] = pen.PenetrationDepth * 1000
			penSuccess[armorKey] = pen.Success
		}

		muzzleKE := 0.5 * cfg.BulletMass * cfg.MuzzleVelocity * cfg.MuzzleVelocity
		impactKE := 0.5 * cfg.BulletMass * estImpactVel * estImpactVel
		shotsPerMin := 60.0 / cfg.ReloadSeconds

		item := models.WeaponEraComparison{
			WeaponType:         cfg.Type,
			WeaponName:         cfg.Name,
			Description:        cfg.Description,
			Era:                cfg.Era,
			IsModern:           true,
			ProjectileMass:     cfg.BulletMass,
			ProjectileDia:      cfg.BulletDiameter,
			ProjectileLen:      cfg.BulletLength,
			MuzzleVelocity:     cfg.MuzzleVelocity,
			MaxRange:           cfg.MaxRange,
			EffectiveRange:     cfg.EffectiveRange,
			SpinRate:           cfg.SpinRate,
			KineticEnergy:      muzzleKE,
			ImpactVelocity:     estImpactVel,
			ImpactKE:           impactKE,
			Penetrations:       pens,
			PenetrationSuccess: penSuccess,
			CrewSize:           cfg.CrewSize,
			ReloadSeconds:      cfg.ReloadSeconds,
			ShotsPerMinute:     shotsPerMin,
			KEPerMinute:        muzzleKE * shotsPerMin,
		}
		resultList = append(resultList, item)
	}

	if bedCrossbowKE > 0 {
		for i := range resultList {
			resultList[i].PowerRatio = resultList[i].ImpactKE / bedCrossbowKE
		}
	}

	sort.Slice(resultList, func(i, j int) bool {
		return resultList[i].ImpactKE > resultList[j].ImpactKE
	})

	return &models.EraComparisonResponse{
		Weapons:    resultList,
		ArmorTypes: armorTypes,
	}
}
