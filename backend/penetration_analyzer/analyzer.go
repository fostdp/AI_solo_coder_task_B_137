package penetration_analyzer

import (
	"math"
	"time"

	"ballistics-system/backend/config"
	"ballistics-system/backend/models"
)

type Analyzer struct {
	armors     map[string]config.ArmorEntryConfig
	arrowHeads map[string]config.ArrowHeadEntryConfig
	gyro       config.GyroConfig
	Metrics    PenMetricsHooks
}

type PenMetricsHooks interface {
	IncPenetration()
	ObservePenDuration(d float64)
	ObservePenetrationDepth(mm float64)
	IncActivePen()
	DecActivePen()
	SetPendingPen(n int)
}

type noopPenMetrics struct{}

func (noopPenMetrics) IncPenetration()              {}
func (noopPenMetrics) ObservePenDuration(float64)    {}
func (noopPenMetrics) ObservePenetrationDepth(float64) {}
func (noopPenMetrics) IncActivePen()                {}
func (noopPenMetrics) DecActivePen()                {}
func (noopPenMetrics) SetPendingPen(int)            {}

func NewAnalyzer(armorCfg *config.ArmorConfig) *Analyzer {
	return &Analyzer{
		armors:     armorCfg.Armors,
		arrowHeads: armorCfg.ArrowHeads,
		gyro:       armorCfg.Gyro,
		Metrics:    noopPenMetrics{},
	}
}

func (a *Analyzer) GetArmorConfig(armorType string) (config.ArmorEntryConfig, bool) {
	cfg, ok := a.armors[armorType]
	if !ok {
		return a.armors["leather"], false
	}
	return cfg, true
}

func (a *Analyzer) GetArrowConfig(arrowType string) (config.ArrowHeadEntryConfig, bool) {
	cfg, ok := a.arrowHeads[arrowType]
	if !ok {
		return a.arrowHeads["bodkin"], false
	}
	return cfg, true
}

func (a *Analyzer) calculateGyroscopicStability(spinRate, velocity, mass, diameter, length float64) float64 {
	if velocity < a.gyro.LowVelocityThreshold {
		return a.gyro.LowVelocityStability
	}
	if length == 0 {
		length = 1.0
	}
	axialMOI := 0.5 * mass * math.Pow(diameter/2.0, 2)
	transverseMOI := (1.0 / 12.0) * mass * (3.0*math.Pow(diameter/2.0, 2) + length*length)
	angularMomentum := axialMOI * spinRate * 2.0 * math.Pi
	aerodynamicMoment := 0.5 * 1.225 * velocity * velocity * math.Pow(diameter, 2) * length * 0.01
	if aerodynamicMoment < 1e-9 {
		return a.gyro.LowVelocityStability
	}
	stability := (angularMomentum * angularMomentum) / (2.0 * axialMOI * transverseMOI * aerodynamicMoment)
	return math.Min(math.Max(stability, a.gyro.StabilityClampMin), a.gyro.StabilityClampMax)
}

func (a *Analyzer) calculateYawAngle(gyroStability float64) float64 {
	if gyroStability >= a.gyro.YawStableThreshold {
		return 0.002
	}
	if gyroStability >= a.gyro.YawModerateThreshold {
		return 0.005 + 0.01*(a.gyro.YawModerateThreshold-gyroStability)/(a.gyro.YawStableThreshold-a.gyro.YawModerateThreshold)
	}
	if gyroStability >= a.gyro.YawMarginalThreshold {
		return 0.015 + 0.05*(a.gyro.YawMarginalThreshold-gyroStability)/(a.gyro.YawModerateThreshold-a.gyro.YawMarginalThreshold)
	}
	return 0.065 + 0.20*(a.gyro.YawMarginalThreshold-gyroStability)
}

func (a *Analyzer) calculateEffectiveArea(baseArea, yawAngle, diameter, length float64) float64 {
	if length == 0 {
		length = 1.0
	}
	cosYaw := math.Cos(yawAngle)
	sinYaw := math.Abs(math.Sin(yawAngle))
	return baseArea*cosYaw + diameter*length*sinYaw
}

func (a *Analyzer) calculateStabilityPenalty(gyroStability float64) float64 {
	if gyroStability >= a.gyro.StabilityPenaltyFull {
		return 1.0
	}
	if gyroStability >= a.gyro.StabilityPenaltyModerate {
		return 0.75 + 0.25*(gyroStability-a.gyro.StabilityPenaltyModerate)/(a.gyro.StabilityPenaltyFull-a.gyro.StabilityPenaltyModerate)
	}
	if gyroStability >= a.gyro.StabilityPenaltyPoor {
		return 0.40 + 0.35*(gyroStability-a.gyro.StabilityPenaltyPoor)/(a.gyro.StabilityPenaltyModerate-a.gyro.StabilityPenaltyPoor)
	}
	return 0.15 + 0.25*gyroStability*2.0
}

func (a *Analyzer) calculateRotationalEnergy(mass, diameter, spinRate float64) float64 {
	axialMOI := 0.5 * mass * math.Pow(diameter/2.0, 2)
	angularVel := spinRate * 2.0 * math.Pi
	return 0.5 * axialMOI * angularVel * angularVel
}

func (a *Analyzer) AnalyzeWithSpin(impactVelocity, arrowMass, arrowDiameter, arrowLength, spinRate float64, armorType string, arrowHeadType string, armorThickness float64) *models.PenetrationResult {
	armorCfg, _ := a.GetArmorConfig(armorType)
	arrowCfg, _ := a.GetArrowConfig(arrowHeadType)

	armorThickness_ := armorCfg.Thickness
	if armorThickness > 0 {
		armorThickness_ = armorThickness
	}

	gyroStability := a.calculateGyroscopicStability(spinRate, impactVelocity, arrowMass, arrowDiameter, arrowLength)
	yawAngle := a.calculateYawAngle(gyroStability)
	effectiveArea := a.calculateEffectiveArea(arrowCfg.TipArea, yawAngle, arrowDiameter, arrowLength)
	stabFactor := a.calculateStabilityPenalty(gyroStability)

	translationalKE := 0.5 * arrowMass * impactVelocity * impactVelocity
	rotationalKE := a.calculateRotationalEnergy(arrowMass, arrowDiameter, spinRate)
	totalEffectiveKE := translationalKE + rotationalKE*0.3

	basePenetration := a.calculateThompsonPenetration(
		impactVelocity, arrowMass, arrowCfg.TipArea,
		armorCfg.Density, armorCfg.YieldStrength, armorCfg.Hardness, arrowCfg.Hardness,
	)

	areaRatio := arrowCfg.TipArea / effectiveArea
	if areaRatio > 1.0 {
		areaRatio = 1.0
	}

	penetrationDepth := basePenetration * areaRatio * stabFactor

	if rotationalKE > 0 {
		rotaryBoost := 1.0 + (rotationalKE / translationalKE) * a.gyro.RotaryEnergyBoostFactor
		penetrationDepth *= rotaryBoost
	}

	residualVelocity := 0.0
	if penetrationDepth > armorThickness_ {
		remainingEnergy := totalEffectiveKE * (1.0 - armorThickness_/penetrationDepth)
		if remainingEnergy > 0 {
			residualVelocity = math.Sqrt(2.0 * remainingEnergy / arrowMass)
		}
		penetrationDepth = armorThickness_
	}

	energyAbsorbed := translationalKE - 0.5*arrowMass*residualVelocity*residualVelocity
	success := penetrationDepth >= armorThickness_

	return &models.PenetrationResult{
		ArmorType:        armorType,
		ArmorThickness:   armorThickness_,
		ImpactVelocity:   impactVelocity,
		ArrowMass:        arrowMass,
		ArrowHeadType:    arrowHeadType,
		PenetrationDepth: penetrationDepth,
		ResidualVelocity: residualVelocity,
		EnergyAbsorbed:   energyAbsorbed,
		Success:          success,
		ImpactSpinRate:   spinRate,
		GyroStability:    gyroStability,
		YawAngle:         yawAngle,
		EffectiveArea:    effectiveArea,
		StabilityFactor:  stabFactor,
	}
}

func (a *Analyzer) calculateThompsonPenetration(
	velocity, mass, area, armorDensity, yieldStrength, armorHardness, arrowHardness float64,
) float64 {
	hardnessRatio := arrowHardness / (armorHardness + arrowHardness)
	if hardnessRatio < 0.3 {
		hardnessRatio = 0.3
	}

	modifiedStrength := yieldStrength * (1.0 + 0.5*(1.0-hardnessRatio))
	term1 := (mass / area) / armorDensity
	term2 := 0.5 * armorDensity * velocity * velocity / modifiedStrength
	term3 := math.Log(1.0 + term2)

	penetration := term1 * term3 * hardnessRatio
	return penetration
}

func (a *Analyzer) CalculateBallisticLimit(
	arrowMass, tipArea, armorThickness, armorDensity, yieldStrength, armorHardness, arrowHardness float64,
) float64 {
	hardnessRatio := arrowHardness / (armorHardness + arrowHardness)
	if hardnessRatio < 0.3 {
		hardnessRatio = 0.3
	}

	modifiedStrength := yieldStrength * (1.0 + 0.5*(1.0-hardnessRatio))
	term1 := armorThickness * armorDensity * hardnessRatio * tipArea / arrowMass
	term2 := math.Exp(term1) - 1.0
	term3 := 2.0 * modifiedStrength / (armorDensity * term2)
	if term3 < 0 {
		term3 = 0
	}
	return math.Sqrt(term3)
}

func (a *Analyzer) CompareArmorsWithSpin(impactVelocity, arrowMass, arrowDiameter, arrowLength, spinRate float64, arrowHeadType string) map[string]*models.PenetrationResult {
	results := make(map[string]*models.PenetrationResult)
	for key, cfg := range a.armors {
		result := a.AnalyzeWithSpin(impactVelocity, arrowMass, arrowDiameter, arrowLength, spinRate, key, arrowHeadType, 0)
		results[cfg.Name] = result
	}
	return results
}

func (a *Analyzer) ToArmorPerformance(r *models.PenetrationResult) *models.ArmorPerformance {
	return &models.ArmorPerformance{
		Timestamp:        time.Now(),
		ArmorType:        r.ArmorType,
		ArmorThickness:   r.ArmorThickness,
		ImpactVelocity:   r.ImpactVelocity,
		ArrowMass:        r.ArrowMass,
		ArrowHeadType:    r.ArrowHeadType,
		PenetrationDepth: r.PenetrationDepth,
		ResidualVelocity: r.ResidualVelocity,
		EnergyAbsorbed:   r.EnergyAbsorbed,
		GyroStability:    r.GyroStability,
		YawAngle:         r.YawAngle,
		EffectiveArea:    r.EffectiveArea,
	}
}

func (a *Analyzer) RunPenetrationWorker(jobCh <-chan *models.PenJob, resultCh chan<- *models.PenetrationResult) {
	for job := range jobCh {
		a.Metrics.SetPendingPen(len(jobCh))
		a.Metrics.IncActivePen()
		start := time.Now()
		result := a.AnalyzeWithSpin(
			job.ImpactVelocity, job.ArrowMass, job.ArrowDiameter,
			job.ArrowLength, job.SpinRate, job.ArmorType,
			job.ArrowHeadType, job.ArmorThickness,
		)
		a.Metrics.ObservePenDuration(time.Since(start).Seconds())
		a.Metrics.ObservePenetrationDepth(result.PenetrationDepth * 1000)
		a.Metrics.IncPenetration()
		a.Metrics.DecActivePen()
		resultCh <- result
	}
}

func (a *Analyzer) ListArmorTypes() []map[string]interface{} {
	var list []map[string]interface{}
	for _, cfg := range a.armors {
		list = append(list, map[string]interface{}{
			"type":        cfg.Type,
			"name":        cfg.Name,
			"thickness_mm": cfg.Thickness * 1000,
			"density":     cfg.Density,
		})
	}
	return list
}

func (a *Analyzer) ListArrowHeadTypes() []map[string]interface{} {
	var list []map[string]interface{}
	for _, cfg := range a.arrowHeads {
		list = append(list, map[string]interface{}{
			"type":             cfg.Type,
			"name":             cfg.Name,
			"tip_diameter_mm":  cfg.TipDiameter * 1000,
			"hardness":         cfg.Hardness,
		})
	}
	return list
}

func (a *Analyzer) ArmorTypeKeys() []string {
	keys := make([]string, 0, len(a.armors))
	for k := range a.armors {
		keys = append(keys, k)
	}
	return keys
}

func (a *Analyzer) AnalyzeModernBullet(impactVelocity float64, weaponCfg *config.ModernWeaponConfig, armorType string, armorThickness float64) *models.PenetrationResult {
	armorCfg, _ := a.GetArmorConfig(armorType)
	armorThickness_ := armorCfg.Thickness
	if armorThickness > 0 {
		armorThickness_ = armorThickness
	}

	gyroStability := a.calculateGyroscopicStability(
		weaponCfg.SpinRate, impactVelocity, weaponCfg.BulletMass,
		weaponCfg.BulletDiameter, weaponCfg.BulletLength,
	)

	if gyroStability > 1.0 {
		gyroStability = math.Min(gyroStability*1.5, 50.0)
	}

	yawAngle := a.calculateYawAngle(gyroStability)
	effectiveArea := a.calculateEffectiveArea(weaponCfg.TipArea, yawAngle, weaponCfg.BulletDiameter, weaponCfg.BulletLength)
	stabFactor := a.calculateStabilityPenalty(gyroStability)

	translationalKE := 0.5 * weaponCfg.BulletMass * impactVelocity * impactVelocity
	rotationalKE := a.calculateRotationalEnergy(weaponCfg.BulletMass, weaponCfg.BulletDiameter, weaponCfg.SpinRate)
	totalEffectiveKE := translationalKE + rotationalKE*0.3

	basePenetration := a.calculateThompsonPenetration(
		impactVelocity, weaponCfg.BulletMass, weaponCfg.TipArea,
		armorCfg.Density, armorCfg.YieldStrength, armorCfg.Hardness, weaponCfg.Hardness,
	)

	areaRatio := weaponCfg.TipArea / effectiveArea
	if areaRatio > 1.0 {
		areaRatio = 1.0
	}

	penetrationDepth := basePenetration * areaRatio * stabFactor
	if rotationalKE > 0 {
		rotaryBoost := 1.0 + (rotationalKE / translationalKE) * a.gyro.RotaryEnergyBoostFactor * 1.5
		penetrationDepth *= rotaryBoost
	}

	residualVelocity := 0.0
	if penetrationDepth > armorThickness_ {
		remainingEnergy := totalEffectiveKE * (1.0 - armorThickness_/penetrationDepth)
		if remainingEnergy > 0 {
			residualVelocity = math.Sqrt(2.0 * remainingEnergy / weaponCfg.BulletMass)
		}
		penetrationDepth = armorThickness_
	}

	energyAbsorbed := translationalKE - 0.5*weaponCfg.BulletMass*residualVelocity*residualVelocity
	success := penetrationDepth >= armorThickness_

	return &models.PenetrationResult{
		ArmorType:        armorType,
		ArmorThickness:   armorThickness_,
		ImpactVelocity:   impactVelocity,
		ArrowMass:        weaponCfg.BulletMass,
		ArrowHeadType:    "modern_" + weaponCfg.Type,
		PenetrationDepth: penetrationDepth,
		ResidualVelocity: residualVelocity,
		EnergyAbsorbed:   energyAbsorbed,
		Success:          success,
		ImpactSpinRate:   weaponCfg.SpinRate,
		GyroStability:    gyroStability,
		YawAngle:         yawAngle,
		EffectiveArea:    effectiveArea,
		StabilityFactor:  stabFactor,
	}
}
