package salvo_optimizer

import (
	"math"
	"sync"

	"ballistics-system/backend/config"
	"ballistics-system/backend/models"
)

type SimulatorEngine interface {
	Simulate(params *models.SimulationParams) *models.SimulationResult
	SolveElevationForDistance(distance, velocity, arrowMass, arrowDiameter, arrowLength, spinRate float64) (float64, *models.SimulationResult)
}

type DynamicsConfig interface {
	GetGravity() float64
	GetAirDensitySea() float64
	GetMaxSimTime() float64
	GetDragCoefficient() float64
}

type configAdapter struct {
	cfg *config.DynamicsConfig
}

func (ca *configAdapter) GetGravity() float64         { return ca.cfg.Simulation.Gravity }
func (ca *configAdapter) GetAirDensitySea() float64   { return ca.cfg.Simulation.AirDensitySea }
func (ca *configAdapter) GetMaxSimTime() float64      { return ca.cfg.Simulation.MaxSimTime }
func (ca *configAdapter) GetDragCoefficient() float64 { return ca.cfg.Defaults.DragCoefficient }

type trajectoryJob struct {
	params      *models.SimulationParams
	originX     float64
	originY     float64
	azimuthDeg  float64
	cbIndex     int
	shotIdx     int
	shot        *models.BarrageShot
}

type trajectoryResult struct {
	cbIndex    int
	shotIdx    int
	simResult  *models.SimulationResult
	trajectory []models.TrajectorySample
	impactX    float64
	impactY    float64
}

type SalvoOptimizer struct {
	dynamicsCfg *config.DynamicsConfig
	cfgAdapter  DynamicsConfig
	simEngine   SimulatorEngine
	workerCount int
}

func NewSalvoOptimizer(dynamicsCfg *config.DynamicsConfig, simEngine SimulatorEngine) *SalvoOptimizer {
	return &SalvoOptimizer{
		dynamicsCfg: dynamicsCfg,
		cfgAdapter:  &configAdapter{cfg: dynamicsCfg},
		simEngine:   simEngine,
		workerCount: 4,
	}
}

func NewSalvoOptimizerWithWorkers(dynamicsCfg *config.DynamicsConfig, simEngine SimulatorEngine, workerCount int) *SalvoOptimizer {
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > 16 {
		workerCount = 16
	}
	return &SalvoOptimizer{
		dynamicsCfg: dynamicsCfg,
		cfgAdapter:  &configAdapter{cfg: dynamicsCfg},
		simEngine:   simEngine,
		workerCount: workerCount,
	}
}

func (so *SalvoOptimizer) sampleTrajectory(params *models.SimulationParams, originX, originY, azimuthDeg float64) []models.TrajectorySample {
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
	for t := 0.0; t < so.cfgAdapter.GetMaxSimTime(); t += dt {
		if y < -50.0 && t > 0.1 {
			break
		}
		speed := math.Sqrt(vx*vx + vy*vy + vz*vz)
		if speed < 1e-6 {
			break
		}
		ax := -dragFactor * speed * vx
		ay := -so.cfgAdapter.GetGravity() - dragFactor*speed*vy
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

func (so *SalvoOptimizer) minTrajectoryDistance(a, b []models.TrajectorySample, delayA, delayB float64) (float64, float64) {
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

func (so *SalvoOptimizer) worker(id int, jobs <-chan trajectoryJob, results chan<- trajectoryResult, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		simResult := so.simEngine.Simulate(job.params)
		aziRad := job.azimuthDeg * math.Pi / 180.0
		impactRange := simResult.Range
		impactX := job.originX + impactRange*math.Cos(aziRad)
		impactY := job.originY + impactRange*math.Sin(aziRad)
		traj := so.sampleTrajectory(job.params, job.originX, job.originY, job.azimuthDeg)

		job.shot.Azimuth = job.azimuthDeg
		job.shot.Range = impactRange
		job.shot.FlightTime = simResult.FlightTime
		job.shot.ImpactX = impactX
		job.shot.ImpactY = impactY
		job.shot.InitialVelocity = job.params.InitialVelocity

		results <- trajectoryResult{
			cbIndex:    job.cbIndex,
			shotIdx:    job.shotIdx,
			simResult:  simResult,
			trajectory: traj,
			impactX:    impactX,
			impactY:    impactY,
		}
	}
}

func (so *SalvoOptimizer) Optimize(req *models.BarrageOptimizationRequest, crossbowConfigs map[string]config.CrossbowTypeConfig) *models.BarrageOptimizationResponse {
	shots := make([]models.BarrageShot, 0)
	allImpactX := make([]float64, 0)
	allImpactY := make([]float64, 0)
	trajectories := make([][]models.TrajectorySample, 0)
	shotMeta := make([]struct {
		cbIndex int
		shotIdx int
	}, 0)
	simResults := make([]*models.SimulationResult, 0)

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

	totalJobs := 0
	for _, cb := range req.Crossbows {
		if _, ok := crossbowConfigs[cb.Type]; ok {
			totalJobs += req.MaxShotsPerCrossbow
		}
	}

	jobsCh := make(chan trajectoryJob, totalJobs)
	resultsCh := make(chan trajectoryResult, totalJobs)
	var wg sync.WaitGroup

	for w := 0; w < so.workerCount; w++ {
		wg.Add(1)
		go so.worker(w, jobsCh, resultsCh, &wg)
	}

	var minArrival, maxArrival float64 = 1e9, 0
	totalKE := 0.0
	totalDelayMs := 0.0
	shotPtrs := make([]*models.BarrageShot, 0)

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

		elev, _ := so.simEngine.SolveElevationForDistance(baseDistance, cfg.TypicalVelocity, cfg.ArrowMass, cfg.ArrowDiameter, cfg.ArrowLength, cfg.SpinRate)

		for i := 0; i < numShots; i++ {
			angleStep := req.SpreadAngle / float64(numShots)
			aziOffset := -spreadHalf + angleStep*float64(i) + angleStep/2.0
			azi := baseAzimuth + aziOffset
			_ = baseDistance * (0.95 + 0.1*float64(i)/float64(numShots))
			elevJitter := elev - 1.0 + 2.0*float64(i)/float64(numShots)

			params := &models.SimulationParams{
				InitialVelocity: cfg.TypicalVelocity,
				LaunchAngle:     elevJitter,
				AzimuthAngle:    0,
				ArrowMass:       cfg.ArrowMass,
				ArrowDiameter:   cfg.ArrowDiameter,
				ArrowLength:     cfg.ArrowLength,
				SpinRate:        cfg.SpinRate,
				AirDensity:      so.cfgAdapter.GetAirDensitySea(),
				DragCoefficient: so.cfgAdapter.GetDragCoefficient(),
			}

			perShotDelay := delayBase * (float64(cbIndex)*0.7 + float64(i)*0.3)
			shot := &models.BarrageShot{
				CrossbowID:     cb.ID,
				CrossbowName:   cb.Name,
				Elevation:      elevJitter,
				ArrivalTime:    0,
				FireDelayMs:    perShotDelay,
				MinSeparationM: 9999.0,
				CollisionRisk:  "low",
			}

			jobsCh <- trajectoryJob{
				params:     params,
				originX:    cb.X,
				originY:    cb.Y,
				azimuthDeg: azi,
				cbIndex:    cbIndex,
				shotIdx:    i,
				shot:       shot,
			}

			shotPtrs = append(shotPtrs, shot)
			shotMeta = append(shotMeta, struct{ cbIndex, shotIdx int }{cbIndex, i})
		}
	}
	close(jobsCh)
	wg.Wait()
	close(resultsCh)

	resultsList := make([]trajectoryResult, 0, totalJobs)
	for res := range resultsCh {
		resultsList = append(resultsList, res)
	}

	for _, res := range resultsList {
		shot := shotPtrs[res.cbIndex*req.MaxShotsPerCrossbow+res.shotIdx]
		arrivalTime := res.simResult.FlightTime + shot.FireDelayMs/1000.0
		shot.ArrivalTime = arrivalTime
		totalKE += res.simResult.KineticEnergy
		totalDelayMs += shot.FireDelayMs

		if arrivalTime < minArrival {
			minArrival = arrivalTime
		}
		if arrivalTime > maxArrival {
			maxArrival = arrivalTime
		}

		shots = append(shots, *shot)
		trajectories = append(trajectories, res.trajectory)
		simResults = append(simResults, res.simResult)
		allImpactX = append(allImpactX, res.impactX)
		allImpactY = append(allImpactY, res.impactY)
	}

	collisions := 0
	warnings := 0
	delaySec := make([]float64, len(shots))
	for si := range shots {
		delaySec[si] = shots[si].FireDelayMs / 1000.0
	}

	for i := 0; i < len(shots); i++ {
		for j := i + 1; j < len(shots); j++ {
			minDist, tAt := so.minTrajectoryDistance(trajectories[i], trajectories[j], delaySec[i], delaySec[j])
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
				minDist, _ := so.minTrajectoryDistance(trajectories[i], trajectories[j], delaySec[i], delaySec[j])
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

	_ = simResults
	_ = shotMeta

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
