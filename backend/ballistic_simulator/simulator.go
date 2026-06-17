package ballistic_simulator

import (
	"math"
	"time"

	"ballistics-system/config"
	"ballistics-system/models"
)

type Simulator struct {
	bow    config.BowConfig
	sim    config.SimulationConfig
	def    config.DefaultsConfig
	aero   config.AerodynamicsConfig
	Metrics SimMetricsHooks
}

type SimMetricsHooks interface {
	IncSimulation()
	ObserveSimDuration(d float64)
	ObserveImpactVelocity(v float64)
	IncActiveSim()
	DecActiveSim()
	SetPendingSim(n int)
}

type noopSimMetrics struct{}

func (noopSimMetrics) IncSimulation()           {}
func (noopSimMetrics) ObserveSimDuration(float64)  {}
func (noopSimMetrics) ObserveImpactVelocity(float64) {}
func (noopSimMetrics) IncActiveSim()            {}
func (noopSimMetrics) DecActiveSim()            {}
func (noopSimMetrics) SetPendingSim(int)        {}

func NewSimulator(dynCfg *config.DynamicsConfig) *Simulator {
	return &Simulator{
		bow:     dynCfg.Bow,
		sim:     dynCfg.Simulation,
		def:     dynCfg.Defaults,
		aero:    dynCfg.Aerodynamics,
		Metrics: noopSimMetrics{},
	}
}

func sign(x float64) float64 {
	if x > 0 {
		return 1.0
	} else if x < 0 {
		return -1.0
	}
	return 0.0
}

func (s *Simulator) fillDefaults(params *models.SimulationParams) {
	if params.AirDensity == 0 {
		params.AirDensity = s.sim.AirDensitySea
	}
	if params.ArrowMass == 0 {
		params.ArrowMass = s.def.ArrowMass
	}
	if params.ArrowDiameter == 0 {
		params.ArrowDiameter = s.def.ArrowDiameter
	}
	if params.ArrowLength == 0 {
		params.ArrowLength = s.def.ArrowLength
	}
	if params.DragCoefficient == 0 {
		params.DragCoefficient = s.def.DragCoefficient
	}
	if params.SpinRate == 0 {
		params.SpinRate = s.def.SpinRate
	}
	if params.LaunchAngle == 0 {
		params.LaunchAngle = s.def.LaunchAngle
	}
}

func (s *Simulator) Simulate(params *models.SimulationParams) *models.SimulationResult {
	s.fillDefaults(params)

	angleRad := params.LaunchAngle * math.Pi / 180.0
	azimuthRad := params.AzimuthAngle * math.Pi / 180.0

	vx := params.InitialVelocity * math.Cos(angleRad) * math.Cos(azimuthRad)
	vy := params.InitialVelocity * math.Sin(angleRad)
	vz := params.InitialVelocity * math.Cos(angleRad) * math.Sin(azimuthRad)

	spinRate := params.SpinRate

	x, y, z := 0.0, 0.0, 0.0
	maxHeight := 0.0

	crossArea := math.Pi * math.Pow(params.ArrowDiameter/2.0, 2)
	dragFactor := 0.5 * params.DragCoefficient * params.AirDensity * crossArea / params.ArrowMass
	liftFactor := 0.5 * s.aero.LiftCoefficient * params.AirDensity * crossArea / params.ArrowMass
	magnusFactor := 0.5 * s.aero.MagnusCoefficient * params.AirDensity * math.Pow(params.ArrowDiameter, 3) * spinRate / params.ArrowMass

	gyroStability := s.CalculateGyroscopicStability(params.SpinRate, params.InitialVelocity, params.ArrowMass, params.ArrowDiameter, params.ArrowLength)

	trajectory := make([]models.TrajectoryPoint, 0, int(s.sim.MaxSimTime/s.sim.TimeStep))

	var t float64
	for t = 0.0; t < s.sim.MaxSimTime; t += s.sim.TimeStep {
		v := math.Sqrt(vx*vx + vy*vy + vz*vz)
		if y < 0 && t > 0.05 {
			break
		}

		pitchDamping := 0.0
		if v > 0 && gyroStability > 1.0 {
			pitchDamping = s.aero.PitchDampingBase / gyroStability
		}

		point := models.TrajectoryPoint{
			Time:           t,
			X:              x,
			Y:              y,
			Z:              z,
			Vx:             vx,
			Vy:             vy,
			Vz:             vz,
			Velocity:       v,
			SpinRate:       spinRate,
			GyroStability:  gyroStability,
			AttitudeStable: gyroStability >= 1.0,
		}
		trajectory = append(trajectory, point)

		if y > maxHeight {
			maxHeight = y
		}

		ax := -dragFactor*v*vx + magnusFactor*vz
		ay := -s.sim.Gravity - dragFactor*v*vy
		az := -dragFactor*v*vz - magnusFactor*vx

		if gyroStability >= 1.0 {
			ax += liftFactor * v * vy * pitchDamping
		}

		spinRate *= (1.0 - s.aero.SpinDecayRate*v*s.sim.TimeStep)

		vx += ax * s.sim.TimeStep
		vy += ay * s.sim.TimeStep
		vz += az * s.sim.TimeStep

		x += vx * s.sim.TimeStep
		y += vy * s.sim.TimeStep
		z += vz * s.sim.TimeStep
	}

	flightTime := t
	range_ := math.Sqrt(x*x + z*z)
	impactVelocity := math.Sqrt(vx*vx + vy*vy + vz*vz)
	kineticEnergy := 0.5 * params.ArrowMass * impactVelocity * impactVelocity
	impactSpin := spinRate
	impactGyro := s.CalculateGyroscopicStability(spinRate, impactVelocity, params.ArrowMass, params.ArrowDiameter, params.ArrowLength)

	return &models.SimulationResult{
		Timestamp:       time.Now(),
		InitialVelocity: params.InitialVelocity,
		LaunchAngle:     params.LaunchAngle,
		FlightTime:      flightTime,
		MaxHeight:       maxHeight,
		Range:           range_,
		ImpactVelocity:  impactVelocity,
		KineticEnergy:   kineticEnergy,
		ImpactSpinRate:  impactSpin,
		ImpactGyroStab:  impactGyro,
		Trajectory:      trajectory,
	}
}

func (s *Simulator) CalculateGyroscopicStability(spinRate, velocity, mass, diameter, length float64) float64 {
	if length == 0 {
		length = s.def.ArrowLength
	}
	if velocity < 1.0 {
		return 10.0
	}
	axialMOI := 0.5 * mass * math.Pow(diameter/2.0, 2)
	transverseMOI := (1.0 / 12.0) * mass * (3.0*math.Pow(diameter/2.0, 2) + length*length)
	angularMomentum := axialMOI * spinRate * 2.0 * math.Pi
	aerodynamicMoment := 0.5 * s.sim.AirDensitySea * velocity * velocity * math.Pow(diameter, 2) * length * s.aero.AeroMomentCoefficient
	if aerodynamicMoment < 1e-9 {
		return 10.0
	}
	stability := (angularMomentum * angularMomentum) / (2.0 * axialMOI * transverseMOI * aerodynamicMoment)
	return math.Min(math.Max(stability, 0.1), 50.0)
}

type ReleaseState struct {
	ArrowX           float64
	ArrowV           float64
	ArmAngle         float64
	ArmAngularVel    float64
	StringTension    float64
	StringElong      float64
	PotentialEnergy  float64
	KineticEnergy    float64
	DissipatedEnergy float64
	Time             float64
}

func (s *Simulator) SimulateFullRelease(arrowMass float64) (float64, map[string]float64) {
	bow := s.bow
	state := &ReleaseState{
		ArrowX:          -bow.DrawLength,
		ArmAngle:        math.Asin(bow.DrawLength / (2.0 * bow.ArmLength)),
		StringElong:     bow.DrawLength * 0.3,
		PotentialEnergy: 0.5 * bow.PeakTension * bow.DrawLength,
	}

	armInertia := (1.0 / 3.0) * bow.ArmMass * bow.ArmLength * bow.ArmLength
	stringCrossArea := bow.StringCrossArea
	if stringCrossArea == 0 {
		stringCrossArea = 5e-5
	}
	stringStiffness := bow.StringYoungMod * stringCrossArea / bow.StringLength

	totalInitialEnergy := state.PotentialEnergy

	var t float64
	for t = 0.0; t < s.sim.ReleaseDuration; t += s.sim.ReleaseTimeStep {
		armRestoringTorque := -bow.PeakTension * bow.ArmLength * state.ArmAngle / (0.5 * math.Pi)

		nonlinearDamTq := -bow.NonlinearDamping * armInertia * state.ArmAngularVel * math.Abs(state.ArmAngularVel)

		hysteresisDamTq := -bow.HysteresisFactor * bow.PeakTension * bow.ArmLength * sign(state.ArmAngularVel)

		viscousDamTq := -bow.ViscousDamping * armInertia * state.ArmAngularVel

		internalDamTq := -bow.InternalDamping * bow.PeakTension * bow.ArmLength * state.ArmAngle / (0.5 * math.Pi)

		totalTorque := armRestoringTorque + nonlinearDamTq + hysteresisDamTq + viscousDamTq + internalDamTq
		armAngularAccel := totalTorque / armInertia

		armTipVel := state.ArmAngularVel * bow.ArmLength
		_ = armTipVel * math.Cos(state.ArmAngle)

		currentDraw := -state.ArrowX
		stringTensionForce := stringStiffness * state.StringElong

		accelOnArrow := (stringTensionForce * math.Cos(state.ArmAngle) * 2.0) / arrowMass
		viscousArrowDam := -bow.ViscousDamping * 0.1 * state.ArrowV

		totalArrowAccel := accelOnArrow + viscousArrowDam

		state.ArmAngularVel += armAngularAccel * s.sim.ReleaseTimeStep
		state.ArmAngle += state.ArmAngularVel * s.sim.ReleaseTimeStep
		state.ArrowV += totalArrowAccel * s.sim.ReleaseTimeStep
		state.ArrowX += state.ArrowV * s.sim.ReleaseTimeStep

		state.StringElong = math.Max(0, currentDraw*0.3+(armTipVel-armTipVel*math.Cos(state.ArmAngle))*s.sim.ReleaseTimeStep)

		armKE := 0.5 * armInertia * state.ArmAngularVel * state.ArmAngularVel * 3.0
		arrowKE := 0.5 * arrowMass * state.ArrowV * state.ArrowV
		stringKE := 0.5 * bow.StringMass * state.ArrowV * state.ArrowV * 0.33
		state.KineticEnergy = armKE + arrowKE + stringKE

		angleRatio := state.ArmAngle / math.Asin(bow.DrawLength/(2.0*bow.ArmLength))
		state.PotentialEnergy = 0.5 * bow.PeakTension * bow.DrawLength * angleRatio * angleRatio

		state.DissipatedEnergy = totalInitialEnergy - state.PotentialEnergy - state.KineticEnergy

		if state.ArrowX >= 0 && state.ArrowV > 0 {
			break
		}
	}

	exitVelocity := state.ArrowV
	armFinalKE := 0.5 * armInertia * state.ArmAngularVel * state.ArmAngularVel * 3.0
	arrowFinalKE := 0.5 * arrowMass * exitVelocity * exitVelocity

	energyBudget := map[string]float64{
		"initial_potential": totalInitialEnergy,
		"arrow_ke":          arrowFinalKE,
		"arm_ke":            armFinalKE,
		"dissipated":        state.DissipatedEnergy,
		"hysteresis_loss":   state.DissipatedEnergy * 0.35,
		"viscous_loss":      state.DissipatedEnergy * 0.30,
		"internal_loss":     state.DissipatedEnergy * 0.20,
		"nonlinear_loss":    state.DissipatedEnergy * 0.15,
		"efficiency":        arrowFinalKE / totalInitialEnergy,
		"release_time":      t,
	}

	return exitVelocity, energyBudget
}

func (s *Simulator) CalculateDeformationStress(deformation, armLength, armThickness, modulus float64) float64 {
	strain := deformation * armThickness / (2.0 * armLength * armLength)
	stress := modulus * strain
	return stress
}

func (s *Simulator) CalculateOptimalAngle(targetDistance, velocity float64) float64 {
	g := s.sim.Gravity
	v2 := velocity * velocity
	discriminant := v2*v2 - g*(g*targetDistance*targetDistance)
	if discriminant < 0 {
		return 45.0
	}
	sqrtDisc := math.Sqrt(discriminant)
	angle1 := math.Asin((v2 - sqrtDisc) / (g * targetDistance))
	angle2 := math.Asin((v2 + sqrtDisc) / (g * targetDistance))
	angle := math.Min(angle1, angle2)
	return angle * 180.0 / math.Pi
}

func (s *Simulator) RunSimulationWorker(jobCh <-chan *models.SimJob, resultCh chan<- *models.SimulationResult) {
	for job := range jobCh {
		s.Metrics.SetPendingSim(len(jobCh))
		s.Metrics.IncActiveSim()
		start := time.Now()
		result := s.Simulate(job.Params)
		result.DeviceID = job.DeviceID
		s.Metrics.ObserveSimDuration(time.Since(start).Seconds())
		s.Metrics.ObserveImpactVelocity(result.ImpactVelocity)
		s.Metrics.IncSimulation()
		s.Metrics.DecActiveSim()
		resultCh <- result
	}
}

func (s *Simulator) SimulateWithCrossbowType(crossbowType string, crossbowCfg *config.CrossbowTypeConfig, launchAngle, azimuthAngle float64) *models.SimulationResult {
	params := &models.SimulationParams{
		InitialVelocity: crossbowCfg.TypicalVelocity,
		LaunchAngle:     launchAngle,
		AzimuthAngle:    azimuthAngle,
		ArrowMass:       crossbowCfg.ArrowMass,
		ArrowDiameter:   crossbowCfg.ArrowDiameter,
		ArrowLength:     crossbowCfg.ArrowLength,
		SpinRate:        crossbowCfg.SpinRate,
		AirDensity:      s.sim.AirDensitySea,
		DragCoefficient: s.def.DragCoefficient,
	}
	return s.Simulate(params)
}

func (s *Simulator) SolveElevationForDistance(distance, velocity, arrowMass, arrowDiameter, arrowLength, spinRate float64) (float64, *models.SimulationResult) {
	bestAngle := 45.0
	bestResult := &models.SimulationResult{}
	minError := 1e9

	runSim := func(angleDeg float64) (*models.SimulationResult, float64) {
		params := &models.SimulationParams{
			InitialVelocity: velocity,
			LaunchAngle:     angleDeg,
			AzimuthAngle:    0.0,
			ArrowMass:       arrowMass,
			ArrowDiameter:   arrowDiameter,
			ArrowLength:     arrowLength,
			SpinRate:        spinRate,
			AirDensity:      s.sim.AirDensitySea,
			DragCoefficient: s.def.DragCoefficient,
		}
		result := s.Simulate(params)
		return result, math.Abs(result.Range - distance)
	}

	for angleDeg := 2.0; angleDeg <= 88.0; angleDeg += 0.25 {
		result, err := runSim(angleDeg)
		if err < minError {
			minError = err
			bestAngle = angleDeg
			bestResult = result
		}
	}

	if minError > 0.5 {
		refineStart := math.Max(2.0, bestAngle-1.0)
		refineEnd := math.Min(88.0, bestAngle+1.0)
		for angleDeg := refineStart; angleDeg <= refineEnd; angleDeg += 0.05 {
			result, err := runSim(angleDeg)
			if err < minError {
				minError = err
				bestAngle = angleDeg
				bestResult = result
			}
		}
	}

	return bestAngle, bestResult
}

func (s *Simulator) simulateFlightWithWind(params *models.SimulationParams, targetDist, targetHeight, windX, windZ float64) (float64, float64, float64, float64, float64, float64) {
	angleRad := params.LaunchAngle * math.Pi / 180.0
	azimuthRad := params.AzimuthAngle * math.Pi / 180.0
	v := params.InitialVelocity
	vx := v * math.Cos(angleRad) * math.Cos(azimuthRad)
	vy := v * math.Sin(angleRad)
	vz := v * math.Cos(angleRad) * math.Sin(azimuthRad)
	x, y, z := 0.0, 0.0, 0.0
	maxHeight := 0.0
	flightTime := 0.0
	spinR := params.SpinRate
	impactVx, impactVy, impactVz := 0.0, 0.0, 0.0

	crossArea := math.Pi * math.Pow(params.ArrowDiameter/2.0, 2)
	dragFactor := 0.5 * params.DragCoefficient * params.AirDensity * crossArea / params.ArrowMass

	for t := 0.0; t < s.sim.MaxSimTime; t += s.sim.TimeStep {
		speed := math.Sqrt(vx*vx + vy*vy + vz*vz)
		if y < -targetHeight && t > 0.05 {
			frac := (y + targetHeight) / (y - (y + s.sim.TimeStep*vy))
			if math.IsNaN(frac) {
				frac = 0.5
			}
			flightTime = t - frac*s.sim.TimeStep
			x -= frac * s.sim.TimeStep * vx
			y = -targetHeight
			z -= frac * s.sim.TimeStep * vz
			impactVx = vx
			impactVy = vy
			impactVz = vz
			break
		}
		if t >= s.sim.MaxSimTime-s.sim.TimeStep {
			flightTime = t
			impactVx = vx
			impactVy = vy
			impactVz = vz
		}
		if y > maxHeight {
			maxHeight = y
		}
		relVx := vx - windX
		relVz := vz - windZ
		relV := math.Sqrt(relVx*relVx + vy*vy + relVz*relVz)
		if relV < 1e-6 {
			relV = 1e-6
		}
		ax := -dragFactor * relV * relVx
		ay := -s.sim.Gravity - dragFactor*relV*vy
		az := -dragFactor * relV * relVz
		if spinR > 0.1 && speed > 1 {
			magnusF := s.aero.MagnusCoefficient * crossArea * params.AirDensity * spinR * speed / params.ArrowMass
			perpendicularX := -vz / speed
			perpendicularZ := vx / speed
			ax += magnusF * perpendicularX
			az += magnusF * perpendicularZ
		}
		spinR *= (1.0 - s.aero.SpinDecayRate*speed*s.sim.TimeStep)
		vx += ax * s.sim.TimeStep
		vy += ay * s.sim.TimeStep
		vz += az * s.sim.TimeStep
		x += vx * s.sim.TimeStep
		y += vy * s.sim.TimeStep
		z += vz * s.sim.TimeStep
	}

	horizontalRange := math.Sqrt(x*x + z*z)
	impactSpeed := math.Sqrt(impactVx*impactVx + impactVy*impactVy + impactVz*impactVz)
	return horizontalRange, y, z, flightTime, maxHeight, impactSpeed
}

func (s *Simulator) SolveElevationWithWind(distance, height, velocity, arrowMass, arrowDiameter, arrowLength, spinRate, windSpeed, windDirDeg float64) (float64, float64, *models.SimulationResult) {
	bestAngle := 45.0
	bestAzimuth := 0.0
	bestResult := &models.SimulationResult{}
	minError := 1e9

	windDirRad := windDirDeg * math.Pi / 180.0
	windX := windSpeed * math.Cos(windDirRad)
	windZ := windSpeed * math.Sin(windDirRad)

	evaluate := func(angleDeg, aziDeg float64) float64 {
		params := &models.SimulationParams{
			InitialVelocity: velocity,
			LaunchAngle:     angleDeg,
			AzimuthAngle:    aziDeg,
			ArrowMass:       arrowMass,
			ArrowDiameter:   arrowDiameter,
			ArrowLength:     arrowLength,
			SpinRate:        spinRate,
			AirDensity:      s.sim.AirDensitySea,
			DragCoefficient: s.def.DragCoefficient,
		}
		range_, impactAlt, lateral, ft, mh, impVel := s.simulateFlightWithWind(params, distance, height, windX, windZ)
		_ = impVel
		distErr := math.Abs(range_ - distance)
		altErr := math.Abs(impactAlt + height)
		latErr := math.Abs(lateral)
		score := distErr*1.0 + altErr*3.0 + latErr*1.5
		if score < minError {
			minError = score
			bestAngle = angleDeg
			bestAzimuth = aziDeg
			ke := 0.5 * arrowMass * impVel * impVel
			bestResult = &models.SimulationResult{
				Timestamp:       time.Now(),
				InitialVelocity: velocity,
				LaunchAngle:     angleDeg,
				FlightTime:      ft,
				MaxHeight:       mh,
				Range:           range_,
				ImpactVelocity:  impVel,
				KineticEnergy:   ke,
				ImpactSpinRate:  spinRoughDecay(spinRate, ft),
				RangeError:      distErr,
				HeightError:     altErr,
				LateralError:    latErr,
				DriftLateral:    lateral,
			}
		}
		return score
	}

	aziHalfWind := 8.0 + windSpeed*1.5
	if aziHalfWind < 3.0 {
		aziHalfWind = 3.0
	}
	for angleDeg := 3.0; angleDeg <= 85.0; angleDeg += 0.25 {
		for aziDeg := -aziHalfWind; aziDeg <= aziHalfWind; aziDeg += 0.25 {
			evaluate(angleDeg, aziDeg)
		}
	}

	if minError > 0.3 {
		angleFrom := math.Max(3.0, bestAngle-0.6)
		angleTo := math.Min(85.0, bestAngle+0.6)
		aziFrom := bestAzimuth - 0.6
		aziTo := bestAzimuth + 0.6
		for angleDeg := angleFrom; angleDeg <= angleTo; angleDeg += 0.05 {
			for aziDeg := aziFrom; aziDeg <= aziTo; aziDeg += 0.05 {
				evaluate(angleDeg, aziDeg)
			}
		}
	}

	return bestAngle, bestAzimuth, bestResult
}

func (s *Simulator) RunSimWithWindDirect(params *models.SimulationParams, targetDist, targetHeight, windX, windZ float64) (float64, float64, float64, float64, float64, float64) {
	return s.simulateFlightWithWind(params, targetDist, targetHeight, windX, windZ)
}

func spinRoughDecay(spinInit, t float64) float64 {
	return spinInit * math.Exp(-0.02*t)
}

func (s *Simulator) sampleTrajectory(params *models.SimulationParams, originX, originY, azimuthDeg float64) []models.TrajectorySample {
	samples := make([]models.TrajectorySample, 0, 64)
	angleRad := params.LaunchAngle * math.Pi / 180.0
	aziRad := azimuthDeg * math.Pi / 180.0
	v := params.InitialVelocity
	vx := v * math.Cos(angleRad) * math.Cos(aziRad)
	vy := v * math.Sin(angleRad)
	vz := v * math.Cos(angleRad) * math.Sin(aziRad)
	x, y, z := 0.0, 0.0, 0.0

	crossArea := math.Pi * math.Pow(params.ArrowDiameter/2.0, 2)
	dragFactor := 0.5 * params.DragCoefficient * params.AirDensity * crossArea / params.ArrowMass

	dt := 0.05
	for t := 0.0; t < s.sim.MaxSimTime; t += dt {
		if y < -50.0 && t > 0.1 {
			break
		}
		speed := math.Sqrt(vx*vx + vy*vy + vz*vz)
		if speed < 1e-6 {
			break
		}
		ax := -dragFactor * speed * vx
		ay := -s.sim.Gravity - dragFactor*speed*vy
		az := -dragFactor * speed * vz
		vx += ax * dt
		vy += ay * dt
		vz += az * dt
		x += vx * dt
		y += vy * dt
		z += vz * dt
		worldX := originX + x*math.Cos(aziRad) - z*math.Sin(aziRad)
		worldY := originY + x*math.Sin(aziRad) + z*math.Cos(aziRad)
		samples = append(samples, models.TrajectorySample{
			TimeS: t,
			X:     worldX,
			Y:     worldY,
			Z:     y,
		})
	}
	return samples
}

func (s *Simulator) minTrajectoryDistance(a, b []models.TrajectorySample, delayA, delayB float64) (float64, float64) {
	minDist := 1e9
	timeAtMin := 0.0
	lenA, lenB := len(a), len(b)
	i, j := 0, 0
	for i < lenA && j < lenB {
		ta := a[i].TimeS + delayA
		tb := b[j].TimeS + delayB
		dt := ta - tb
		if math.Abs(dt) < 0.051 {
			dx := a[i].X - b[j].X
			dy := a[i].Y - b[j].Y
			dz := a[i].Z - b[j].Z
			dist := math.Sqrt(dx*dx + dy*dy + dz*dz)
			if dist < minDist {
				minDist = dist
				timeAtMin = math.Max(ta, tb)
			}
			i++
			j++
		} else if dt < 0 {
			i++
		} else {
			j++
		}
	}
	return minDist, timeAtMin
}

func (s *Simulator) OptimizeBarrage(req *models.BarrageOptimizationRequest, crossbowConfigs map[string]config.CrossbowTypeConfig) *models.BarrageOptimizationResponse {
	shots := make([]models.BarrageShot, 0)
	allImpactX := make([]float64, 0)
	allImpactY := make([]float64, 0)
	trajectories := make([][]models.TrajectorySample, 0)
	shotMeta := make([]struct {
		cbIndex int
		shotIdx int
	}, 0)

	if len(req.Crossbows) == 0 {
		defaultCB := models.BarrageCrossbow{
			ID: "default-1", Type: "bed_crossbow_triple", Name: "三弓床弩#1",
			X: -20, Y: -10, Heading: 0, Elevation: 35,
		}
		defaultCB2 := models.BarrageCrossbow{
			ID: "default-2", Type: "bed_crossbow_triple", Name: "三弓床弩#2",
			X: 0, Y: -10, Heading: 0, Elevation: 35,
		}
		defaultCB3 := models.BarrageCrossbow{
			ID: "default-3", Type: "bed_crossbow_triple", Name: "三弓床弩#3",
			X: 20, Y: -10, Heading: 0, Elevation: 35,
		}
		req.Crossbows = []models.BarrageCrossbow{defaultCB, defaultCB2, defaultCB3}
	}
	if req.MaxShotsPerCrossbow <= 0 {
		req.MaxShotsPerCrossbow = 2
	}
	if req.SpreadAngle <= 0 {
		req.SpreadAngle = 8.0
	}
	safetySeparation := req.SafetySeparationM
	if safetySeparation <= 0 {
		safetySeparation = 3.0
	}
	enableCA := req.EnableCollisionAvoidance
	if !enableCA {
		enableCA = true
	}
	delayBase := req.FireDelayBaseMs
	if delayBase <= 0 {
		delayBase = 120.0
	}

	var minArrival, maxArrival float64 = 1e9, 0
	totalKE := 0.0
	totalDelayMs := 0.0

	for cbIndex, cb := range req.Crossbows {
		cfg, ok := crossbowConfigs[cb.Type]
		if !ok {
			continue
		}
		numShots := req.MaxShotsPerCrossbow
		if numShots <= 0 {
			numShots = 1
		}
		spreadHalf := req.SpreadAngle / 2.0
		if spreadHalf <= 0 {
			spreadHalf = 5.0
		}

		dx := req.Target.X - cb.X
		dy := req.Target.Y - cb.Y
		baseAzimuth := math.Atan2(dy, dx) * 180.0 / math.Pi
		baseDistance := math.Sqrt(dx*dx + dy*dy)

		elev, _ := s.SolveElevationForDistance(baseDistance, cfg.TypicalVelocity, cfg.ArrowMass, cfg.ArrowDiameter, cfg.ArrowLength, cfg.SpinRate)

		for i := 0; i < numShots; i++ {
			angleStep := req.SpreadAngle / float64(numShots)
			aziOffset := -spreadHalf + angleStep*float64(i) + angleStep/2.0
			azi := baseAzimuth + aziOffset
			distJitter := baseDistance * (0.95 + 0.1*float64(i)/float64(numShots))
			elevJitter := elev - 1.0 + 2.0*float64(i)/float64(numShots)

			params := &models.SimulationParams{
				InitialVelocity: cfg.TypicalVelocity,
				LaunchAngle:     elevJitter,
				AzimuthAngle:    0,
				ArrowMass:       cfg.ArrowMass,
				ArrowDiameter:   cfg.ArrowDiameter,
				ArrowLength:     cfg.ArrowLength,
				SpinRate:        cfg.SpinRate,
				AirDensity:      s.sim.AirDensitySea,
				DragCoefficient: s.def.DragCoefficient,
			}
			sim := s.Simulate(params)

			aziRad := azi * math.Pi / 180.0
			impactRange := sim.Range
			impactX := cb.X + impactRange*math.Cos(aziRad)
			impactY := cb.Y + impactRange*math.Sin(aziRad)

			perShotDelay := delayBase * (float64(cbIndex)*0.7 + float64(i)*0.3)
			arrivalTime := sim.FlightTime + perShotDelay/1000.0

			if arrivalTime < minArrival {
				minArrival = arrivalTime
			}
			if arrivalTime > maxArrival {
				maxArrival = arrivalTime
			}
			totalKE += sim.KineticEnergy
			totalDelayMs += perShotDelay

			traj := s.sampleTrajectory(params, cb.X, cb.Y, azi)
			trajectories = append(trajectories, traj)
			shotMeta = append(shotMeta, struct{ cbIndex, shotIdx int }{cbIndex, i})

			shots = append(shots, models.BarrageShot{
				CrossbowID:      cb.ID,
				CrossbowName:    cb.Name,
				Azimuth:         azi,
				Elevation:       elevJitter,
				Range:           impactRange,
				FlightTime:      sim.FlightTime,
				ImpactX:         impactX,
				ImpactY:         impactY,
				ArrivalTime:     arrivalTime,
				InitialVelocity: cfg.TypicalVelocity,
				FireDelayMs:     perShotDelay,
				MinSeparationM:  9999.0,
				CollisionRisk:   "low",
			})
			allImpactX = append(allImpactX, impactX)
			allImpactY = append(allImpactY, impactY)
		}
	}

	collisions := 0
	warnings := 0
	delaySec := make([]float64, len(shots))
	for si := range shots {
		delaySec[si] = shots[si].FireDelayMs / 1000.0
	}

	for i := 0; i < len(shots); i++ {
		for j := i + 1; j < len(shots); j++ {
			minDist, tAt := s.minTrajectoryDistance(trajectories[i], trajectories[j], delaySec[i], delaySec[j])
			if minDist < shots[i].MinSeparationM {
				shots[i].MinSeparationM = minDist
			}
			if minDist < shots[j].MinSeparationM {
				shots[j].MinSeparationM = minDist
			}
			if minDist < 0.5 {
				collisions++
				shots[i].CollisionRisk = "critical"
				shots[j].CollisionRisk = "critical"
				shots[i].MinSeparationM = minDist
				shots[j].MinSeparationM = minDist
				_ = tAt
			} else if minDist < safetySeparation {
				warnings++
				if shots[i].CollisionRisk != "critical" {
					shots[i].CollisionRisk = "warning"
				}
				if shots[j].CollisionRisk != "critical" {
					shots[j].CollisionRisk = "warning"
				}
			}
		}
	}

	if collisions > 0 {
		for si := range shots {
			if shots[si].CollisionRisk == "critical" {
				shots[si].FireDelayMs += delayBase * 0.8
				delaySec[si] = shots[si].FireDelayMs / 1000.0
				arrivalOld := shots[si].ArrivalTime
				shots[si].ArrivalTime = shots[si].FlightTime + shots[si].FireDelayMs/1000.0
				if shots[si].ArrivalTime > maxArrival {
					maxArrival = shots[si].ArrivalTime
				}
				if shots[si].ArrivalTime < minArrival && arrivalOld == minArrival {
					minArrival = shots[si].ArrivalTime
				}
			}
		}
		collisions = 0
		warnings = 0
		for i := 0; i < len(shots); i++ {
			shots[i].MinSeparationM = 9999.0
			shots[i].CollisionRisk = "low"
		}
		for i := 0; i < len(shots); i++ {
			for j := i + 1; j < len(shots); j++ {
				minDist, _ := s.minTrajectoryDistance(trajectories[i], trajectories[j], delaySec[i], delaySec[j])
				if minDist < shots[i].MinSeparationM {
					shots[i].MinSeparationM = minDist
				}
				if minDist < shots[j].MinSeparationM {
					shots[j].MinSeparationM = minDist
				}
				if minDist < 0.5 {
					collisions++
					shots[i].CollisionRisk = "critical"
					shots[j].CollisionRisk = "critical"
				} else if minDist < safetySeparation {
					warnings++
					if shots[i].CollisionRisk != "critical" {
						shots[i].CollisionRisk = "warning"
					}
					if shots[j].CollisionRisk != "critical" {
						shots[j].CollisionRisk = "warning"
					}
				}
			}
		}
	}

	var minX, maxX, minY, maxY float64
	if len(allImpactX) > 0 {
		minX, maxX = allImpactX[0], allImpactX[0]
		minY, maxY = allImpactY[0], allImpactY[0]
		for i := range allImpactX {
			if allImpactX[i] < minX {
				minX = allImpactX[i]
			}
			if allImpactX[i] > maxX {
				maxX = allImpactX[i]
			}
			if allImpactY[i] < minY {
				minY = allImpactY[i]
			}
			if allImpactY[i] > maxY {
				maxY = allImpactY[i]
			}
		}
	}

	padding := 10.0
	cellSize := 5.0
	minX -= padding
	maxX += padding
	minY -= padding
	maxY += padding

	nx := int(math.Ceil((maxX - minX) / cellSize))
	ny := int(math.Ceil((maxY - minY) / cellSize))
	if nx < 1 {
		nx = 1
	}
	if ny < 1 {
		ny = 1
	}
	grid := make([][]int, nx)
	for i := range grid {
		grid[i] = make([]int, ny)
	}

	shotsInTarget := 0
	for _, shot := range shots {
		dx := shot.ImpactX - req.Target.X
		dy := shot.ImpactY - req.Target.Y
		if math.Sqrt(dx*dx+dy*dy) <= req.Target.Radius {
			shotsInTarget++
		}
		ix := int((shot.ImpactX - minX) / cellSize)
		iy := int((shot.ImpactY - minY) / cellSize)
		if ix >= 0 && ix < nx && iy >= 0 && iy < ny {
			grid[ix][iy]++
		}
	}

	cellsWithShots := 0
	for i := range grid {
		for j := range grid[i] {
			if grid[i][j] > 0 {
				cellsWithShots++
			}
		}
	}

	areaCovered := float64(cellsWithShots) * cellSize * cellSize
	hitRate := 0.0
	if len(shots) > 0 {
		hitRate = float64(shotsInTarget) / float64(len(shots))
	}
	timeWindow := maxArrival - minArrival
	if timeWindow < 0 {
		timeWindow = 0
	}
	avgDelay := 0.0
	if len(shots) > 0 {
		avgDelay = totalDelayMs / float64(len(shots))
	}

	return &models.BarrageOptimizationResponse{
		Shots: shots,
		Coverage: models.CoverageGrid{
			MinX:     minX,
			MaxX:     maxX,
			MinY:     minY,
			MaxY:     maxY,
			CellSize: cellSize,
			Grid:     grid,
		},
		TargetHitRate:      hitRate,
		AreaCoveredM2:      areaCovered,
		ShotsInTarget:      shotsInTarget,
		TotalShots:         len(shots),
		TimeWindow:         timeWindow,
		KEConcentrated:     totalKE,
		CollisionsDetected: collisions,
		SeparationWarnings: warnings,
		AvgFireDelayMs:     avgDelay,
	}
}
