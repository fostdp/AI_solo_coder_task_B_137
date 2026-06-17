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

	for angleDeg := 5.0; angleDeg <= 85.0; angleDeg += 0.5 {
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
		err := math.Abs(result.Range - distance)
		if err < minError {
			minError = err
			bestAngle = angleDeg
			bestResult = result
		}
	}
	return bestAngle, bestResult
}

func (s *Simulator) SolveElevationWithWind(distance, height, velocity, arrowMass, arrowDiameter, arrowLength, spinRate, windSpeed, windDirDeg float64) (float64, float64, *models.SimulationResult) {
	bestAngle := 45.0
	bestAzimuth := 0.0
	bestResult := &models.SimulationResult{}
	minError := 1e9

	windDirRad := windDirDeg * math.Pi / 180.0
	windX := windSpeed * math.Cos(windDirRad)
	windZ := windSpeed * math.Sin(windDirRad)

	for angleDeg := 5.0; angleDeg <= 85.0; angleDeg += 0.5 {
		for aziDeg := -15.0; aziDeg <= 15.0; aziDeg += 0.5 {
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

			angleRad := angleDeg * math.Pi / 180.0
			azimuthRad := aziDeg * math.Pi / 180.0
			vx := velocity * math.Cos(angleRad) * math.Cos(azimuthRad)
			vy := velocity * math.Sin(angleRad)
			vz := velocity * math.Cos(angleRad) * math.Sin(azimuthRad)
			x, y, z := 0.0, 0.0, 0.0
			maxHeight := 0.0
			flightTime := 0.0
			spinR := spinRate

			crossArea := math.Pi * math.Pow(arrowDiameter/2.0, 2)
			dragFactor := 0.5 * s.def.DragCoefficient * s.sim.AirDensitySea * crossArea / arrowMass

			for t := 0.0; t < s.sim.MaxSimTime; t += s.sim.TimeStep {
				v := math.Sqrt(vx*vx + vy*vy + vz*vz)
				if y < -height && t > 0.05 {
					flightTime = t
					break
				}
				if y > maxHeight {
					maxHeight = y
				}
				relVx := vx - windX
				relVz := vz - windZ
				relV := math.Sqrt(relVx*relVx + vy*vy + relVz*relVz)
				ax := -dragFactor * relV * relVx
				ay := -s.sim.Gravity - dragFactor*relV*vy
				az := -dragFactor * relV * relVz
				spinR *= (1.0 - s.aero.SpinDecayRate*v*s.sim.TimeStep)
				vx += ax * s.sim.TimeStep
				vy += ay * s.sim.TimeStep
				vz += az * s.sim.TimeStep
				x += vx * s.sim.TimeStep
				y += vy * s.sim.TimeStep
				z += vz * s.sim.TimeStep
			}

			range_ := math.Sqrt(x*x + z*z)
			heightErr := math.Abs(y + height)
			distErr := math.Abs(range_ - distance)
			lateralErr := math.Abs(z)
			totalErr := distErr*1.0 + heightErr*2.0 + lateralErr*1.5

			if totalErr < minError {
				minError = totalErr
				bestAngle = angleDeg
				bestAzimuth = aziDeg
				impactVel := math.Sqrt(vx*vx + vy*vy + vz*vz)
				bestResult = &models.SimulationResult{
					Timestamp:       time.Now(),
					InitialVelocity: velocity,
					LaunchAngle:     angleDeg,
					FlightTime:      flightTime,
					MaxHeight:       maxHeight,
					Range:           range_,
					ImpactVelocity:  impactVel,
					KineticEnergy:   0.5 * arrowMass * impactVel * impactVel,
					ImpactSpinRate:  spinR,
				}
			}
		}
	}
	return bestAngle, bestAzimuth, bestResult
}

func (s *Simulator) OptimizeBarrage(req *models.BarrageOptimizationRequest, crossbowConfigs map[string]config.CrossbowTypeConfig) *models.BarrageOptimizationResponse {
	shots := make([]models.BarrageShot, 0)
	allImpactX := make([]float64, 0)
	allImpactY := make([]float64, 0)
	var minArrival, maxArrival float64 = 1e9, 0
	totalKE := 0.0

	for _, cb := range req.Crossbows {
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
			arrivalTime := sim.FlightTime

			if arrivalTime < minArrival {
				minArrival = arrivalTime
			}
			if arrivalTime > maxArrival {
				maxArrival = arrivalTime
			}
			totalKE += sim.KineticEnergy

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
			})
			allImpactX = append(allImpactX, impactX)
			allImpactY = append(allImpactY, impactY)
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
		TargetHitRate:  hitRate,
		AreaCoveredM2:  areaCovered,
		ShotsInTarget:  shotsInTarget,
		TotalShots:     len(shots),
		TimeWindow:     timeWindow,
		KEConcentrated: totalKE,
	}
}
