package power_comparator

import (
	"sort"

	"ballistics-system/backend/config"
	"ballistics-system/backend/models"
)

type SimulatorEngine interface {
	SimulateWithCrossbowType(crossbowType string, crossbowCfg *config.CrossbowTypeConfig, launchAngle, azimuthAngle float64) *models.SimulationResult
}

type PenetrationAnalyzer interface {
	ArmorTypeKeys() []string
	AnalyzeWithSpin(impactVelocity, arrowMass, arrowDiameter, arrowLength, spinRate float64, armorType, arrowHeadType string, obliquityDeg float64) *models.PenetrationResult
}

type PowerComparator struct {
	dynamicsCfg *config.DynamicsConfig
	simEngine   SimulatorEngine
	penAnalyzer PenetrationAnalyzer
}

func NewPowerComparator(dynamicsCfg *config.DynamicsConfig, simEngine SimulatorEngine, penAnalyzer PenetrationAnalyzer) *PowerComparator {
	return &PowerComparator{
		dynamicsCfg: dynamicsCfg,
		simEngine:   simEngine,
		penAnalyzer: penAnalyzer,
	}
}

func (pc *PowerComparator) ListCrossbows() []map[string]interface{} {
	crossbows := make([]map[string]interface{}, 0, len(pc.dynamicsCfg.CrossbowTypes))
	for _, cfg := range pc.dynamicsCfg.CrossbowTypes {
		crossbows = append(crossbows, map[string]interface{}{
			"type":             cfg.Type,
			"name":             cfg.Name,
			"description":      cfg.Description,
			"era":              cfg.Era,
			"draw_force_n":     cfg.DrawForce,
			"draw_length_m":    cfg.DrawLength,
			"arrow_mass_kg":    cfg.ArrowMass,
			"arrow_length_m":   cfg.ArrowLength,
			"arrow_dia_mm":     cfg.ArrowDiameter * 1000,
			"typical_v_ms":     cfg.TypicalVelocity,
			"typical_range_m":  cfg.TypicalRange,
			"spin_rate_hz":     cfg.SpinRate,
			"bow_efficiency":   cfg.BowEfficiency,
			"crew_size":        cfg.CrewSize,
			"reload_seconds":   cfg.ReloadSeconds,
			"data_source":      cfg.DataSource,
			"max_range_recorded": cfg.MaxRangeRecorded,
			"velocity_error":   cfg.VelocityError,
			"historical_note":  cfg.HistoricalNote,
		})
	}
	sort.Slice(crossbows, func(i, j int) bool {
		return crossbows[i]["draw_force_n"].(float64) < crossbows[j]["draw_force_n"].(float64)
	})
	return crossbows
}

func (pc *PowerComparator) Compare(crossbowTypes []string, arrowHeadType string, launchAngle float64) *models.CrossbowComparisonResponse {
	if launchAngle == 0 {
		launchAngle = 45.0
	}
	if arrowHeadType == "" {
		arrowHeadType = "bodkin"
	}

	resultList := make([]models.CrossbowComparisonItem, 0)
	armorTypes := pc.penAnalyzer.ArmorTypeKeys()
	var maxKE float64 = 0

	includeAll := len(crossbowTypes) == 0
	typeSet := make(map[string]bool)
	for _, t := range crossbowTypes {
		typeSet[t] = true
	}

	for key, cfg := range pc.dynamicsCfg.CrossbowTypes {
		if !includeAll && !typeSet[key] {
			continue
		}

		simResult := pc.simEngine.SimulateWithCrossbowType(key, &cfg, launchAngle, 0.0)
		pens := make(map[string]float64)
		penSuccess := make(map[string]bool)
		for _, armorKey := range armorTypes {
			pen := pc.penAnalyzer.AnalyzeWithSpin(
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

		shotsPerMin := 60.0 / cfg.ReloadSeconds
		kePerMin := simResult.KineticEnergy * shotsPerMin
		item := models.CrossbowComparisonItem{
			CrossbowType:       cfg.Type,
			CrossbowName:       cfg.Name,
			Description:        cfg.Description,
			Era:                cfg.Era,
			DrawForce:          cfg.DrawForce,
			DrawLength:         cfg.DrawLength,
			ArrowMass:          cfg.ArrowMass,
			ArrowDiameter:      cfg.ArrowDiameter,
			ArrowLength:        cfg.ArrowLength,
			SpinRate:           cfg.SpinRate,
			BowEfficiency:      cfg.BowEfficiency,
			CrewSize:           cfg.CrewSize,
			ReloadSeconds:      cfg.ReloadSeconds,
			InitialVelocity:    simResult.InitialVelocity,
			Range:              simResult.Range,
			FlightTime:         simResult.FlightTime,
			MaxHeight:          simResult.MaxHeight,
			ImpactVelocity:     simResult.ImpactVelocity,
			KineticEnergy:      simResult.KineticEnergy,
			ImpactSpinRate:     simResult.ImpactSpinRate,
			ImpactGyroStab:     simResult.ImpactGyroStab,
			Penetrations:       pens,
			PenetrationSuccess: penSuccess,
			ShotsPerMinute:     shotsPerMin,
			KEPerMinute:        kePerMin,
		}
		if item.KineticEnergy > maxKE {
			maxKE = item.KineticEnergy
		}
		resultList = append(resultList, item)
	}

	for i := range resultList {
		if maxKE > 0 {
			resultList[i].PowerIndex = (resultList[i].KineticEnergy / maxKE) * (resultList[i].Range / 1500.0) * 100
		}
	}

	sort.Slice(resultList, func(i, j int) bool {
		return resultList[i].PowerIndex > resultList[j].PowerIndex
	})

	return &models.CrossbowComparisonResponse{
		Crossbows:  resultList,
		ArmorTypes: armorTypes,
	}
}
