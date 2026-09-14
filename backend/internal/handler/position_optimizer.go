package handler

import (
	"fgo-calc-backend/internal/model"
	"math"
	"sort"
)

type positionPlan struct {
	supportPosition string
	front            []bool
	positionPercent  []float64
	preBond          []int
	finalBond        []int
	finalScore       []float64
	totalBond        int
	preTotalBond     int
	totalScore       float64
}

func bondWithPosition(baseBond int, info model.TeamResultServantBond, positionPercent float64) int {
	if info.BondCapped {
		return 0
	}
	bondMultiplier := 1.0 + info.BonusPercent/100.0
	positionMultiplier := 1.0 + positionPercent/100.0
	value := math.Floor(float64(baseBond)*bondMultiplier*positionMultiplier + 1e-9)
	return int(value) + info.DirectBonus
}

func weightedBondScore(bond int, weight float64) float64 {
	if weight <= 0 {
		weight = 1.0
	}
	return math.Round(float64(bond)*weight*1000.0) / 1000.0
}

func buildPositionPlan(team model.TeamResponse, baseBond int, supportFront bool) positionPlan {
	count := len(team.ServantBondBonuses)
	plan := positionPlan{
		supportPosition: "back",
		front:            make([]bool, count),
		positionPercent:  make([]float64, count),
		preBond:          make([]int, count),
		finalBond:        make([]int, count),
		finalScore:       make([]float64, count),
	}

	globalPositionPercent := 0.0
	frontSlots := 3
	if supportFront {
		plan.supportPosition = "front"
		globalPositionPercent = 4.0
		frontSlots = 2
	}
	if frontSlots > count {
		frontSlots = count
	}

	type frontCandidate struct {
		index     int
		scoreGain float64
		bondGain  int
		servantID int
	}
	candidates := make([]frontCandidate, 0, count)

	for i, info := range team.ServantBondBonuses {
		pre := bondWithPosition(baseBond, info, 0)
		back := bondWithPosition(baseBond, info, globalPositionPercent)
		front := bondWithPosition(baseBond, info, globalPositionPercent+20.0)
		weight := info.PreferenceWeight
		backScore := weightedBondScore(back, weight)
		frontScore := weightedBondScore(front, weight)

		plan.preBond[i] = pre
		plan.preTotalBond += pre
		candidates = append(candidates, frontCandidate{
			index:     i,
			scoreGain: frontScore - backScore,
			bondGain:  front - back,
			servantID: info.Id,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].scoreGain != candidates[j].scoreGain {
			return candidates[i].scoreGain > candidates[j].scoreGain
		}
		if candidates[i].bondGain != candidates[j].bondGain {
			return candidates[i].bondGain > candidates[j].bondGain
		}
		if candidates[i].servantID != candidates[j].servantID {
			return candidates[i].servantID < candidates[j].servantID
		}
		return candidates[i].index < candidates[j].index
	})
	for i := 0; i < frontSlots; i++ {
		plan.front[candidates[i].index] = true
	}

	for i, info := range team.ServantBondBonuses {
		positionPercent := globalPositionPercent
		if plan.front[i] {
			positionPercent += 20.0
		}
		bond := bondWithPosition(baseBond, info, positionPercent)
		score := weightedBondScore(bond, info.PreferenceWeight)
		plan.positionPercent[i] = positionPercent
		plan.finalBond[i] = bond
		plan.finalScore[i] = score
		plan.totalBond += bond
		plan.totalScore += score
	}

	return plan
}

func betterPositionPlan(candidate, current positionPlan) bool {
	if candidate.totalScore != current.totalScore {
		return candidate.totalScore > current.totalScore
	}
	if candidate.totalBond != current.totalBond {
		return candidate.totalBond > current.totalBond
	}
	// Stable tie-break: keep support in the back when both layouts are identical.
	return candidate.supportPosition == "back" && current.supportPosition == "front"
}

func applyPositionPlan(team *model.TeamResponse, plan positionPlan) {
	team.PositionOptimized = true
	team.SupportPosition = plan.supportPosition
	team.PrePositionTotalBond = plan.preTotalBond
	team.PositionBondGain = plan.totalBond - plan.preTotalBond
	team.TotalBond = plan.totalBond
	team.OptimizationScore = plan.totalScore
	team.FrontlineServants = team.FrontlineServants[:0]
	team.BacklineServants = team.BacklineServants[:0]

	for i := range team.ServantBondBonuses {
		info := &team.ServantBondBonuses[i]
		info.PrePositionBond = plan.preBond[i]
		info.PositionBonusPercent = plan.positionPercent[i]
		info.TotalBond = plan.finalBond[i]
		info.OptimizationScore = plan.finalScore[i]
		if plan.front[i] {
			info.Position = "front"
			team.FrontlineServants = append(team.FrontlineServants, info.Id)
		} else {
			info.Position = "back"
			team.BacklineServants = append(team.BacklineServants, info.Id)
		}
	}
}

func ApplyPositionOptimization(results []model.TeamResponse, baseBond int) []model.TeamResponse {
	for i := range results {
		team := &results[i]
		if len(team.ServantBondBonuses) == 0 {
			continue
		}

		best := buildPositionPlan(*team, baseBond, false)
		if len(team.SupportCraftEssences) > 0 {
			frontSupport := buildPositionPlan(*team, baseBond, true)
			if betterPositionPlan(frontSupport, best) {
				best = frontSupport
			}
		} else {
			best.supportPosition = "none"
		}
		applyPositionPlan(team, best)
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].OptimizationScore != results[j].OptimizationScore {
			return results[i].OptimizationScore > results[j].OptimizationScore
		}
		if results[i].TotalBond != results[j].TotalBond {
			return results[i].TotalBond > results[j].TotalBond
		}
		return results[i].TotalCost > results[j].TotalCost
	})
	return results
}
