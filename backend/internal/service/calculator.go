package service

import (
	"container/heap"
	"fgo-calc-backend/internal/model"
	"fgo-calc-backend/internal/repository"
	"log"
	"math"
	"runtime"
	"sort"
	"sync"
	"time"
)

const OPTIMIZE_LIMIT = 5
const TEATIME_ID = 9403520
const LUNCHTIME_ID = 9401970

type CalculatorService struct {
	repo *repository.Repository
}

func NewCalculatorService(repo *repository.Repository) *CalculatorService {
	return &CalculatorService{repo: repo}
}

func (s *CalculatorService) FilterServants(traits []int, includeSvt []int, excludeSvt []int) []model.Servant {
	includeSet := map[int]bool{}
	excludeSet := map[int]bool{}
	for _, id := range includeSvt {
		includeSet[id] = true
	}
	for _, id := range excludeSvt {
		excludeSet[id] = true
	}

	result := []model.Servant{}
	traitSet := map[int]bool{}
	for _, t := range traits {
		traitSet[t] = true
	}

	servants := s.repo.GetServants()
	for _, svt := range servants {
		if excludeSet[svt.Id] {
			continue
		}
		if includeSet[svt.Id] {
			result = append(result, svt)
			continue
		}
		if len(traits) == 0 {
			result = append(result, svt)
			continue
		}

		matched := false
		for _, detail := range svt.Diff {
			for _, st := range detail.Traits {
				if traitSet[st] {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if matched {
			result = append(result, svt)
		}
	}
	return result
}

func isSupportCandidate(ce model.CraftEssence) bool {
	if ce.Id == TEATIME_ID || ce.Id == LUNCHTIME_ID {
		return true
	}
	for _, filter := range ce.Filters {
		if filter.Effect >= 20 {
			return true
		}
	}
	return false
}

func (s *CalculatorService) GetSupportCombinations(supportLimit int, serverType string, includeSupportCe []int, excludeSupportCe []int) [][]model.CraftEssence {
	if supportLimit <= 0 {
		if len(includeSupportCe) > 0 {
			return [][]model.CraftEssence{}
		}
		return [][]model.CraftEssence{{}}
	}

	excludeSet := map[int]bool{}
	for _, id := range excludeSupportCe {
		excludeSet[id] = true
	}

	supportPool := []model.CraftEssence{}
	supportByID := map[int]model.CraftEssence{}
	craftEssences := s.repo.GetCraftEssences()
	for _, ce := range craftEssences {
		if ce.Server == "JP" && serverType != "JP" {
			continue
		}
		if excludeSet[ce.Id] {
			continue
		}
		if isSupportCandidate(ce) {
			supportPool = append(supportPool, ce)
			supportByID[ce.Id] = ce
		}
	}

	lockedSet := map[int]bool{}
	locked := make([]model.CraftEssence, 0, len(includeSupportCe))
	for _, id := range includeSupportCe {
		if lockedSet[id] {
			continue
		}
		ce, ok := supportByID[id]
		if !ok {
			return [][]model.CraftEssence{}
		}
		locked = append(locked, ce)
		lockedSet[id] = true
	}

	if len(locked) > supportLimit {
		return [][]model.CraftEssence{}
	}

	need := supportLimit - len(locked)
	if need == 0 {
		comb := make([]model.CraftEssence, len(locked))
		copy(comb, locked)
		return [][]model.CraftEssence{comb}
	}

	remaining := make([]model.CraftEssence, 0, len(supportPool))
	for _, ce := range supportPool {
		if !lockedSet[ce.Id] {
			remaining = append(remaining, ce)
		}
	}
	if len(remaining) < need {
		return [][]model.CraftEssence{}
	}

	results := [][]model.CraftEssence{}
	picked := make([]model.CraftEssence, 0, need)
	var dfs func(start int)
	dfs = func(start int) {
		if len(picked) == need {
			comb := make([]model.CraftEssence, 0, supportLimit)
			comb = append(comb, locked...)
			comb = append(comb, picked...)
			results = append(results, comb)
			return
		}
		remainSlots := need - len(picked)
		for i := start; i <= len(remaining)-remainSlots; i++ {
			picked = append(picked, remaining[i])
			dfs(i + 1)
			picked = picked[:len(picked)-1]
		}
	}
	dfs(0)

	return results
}

func (s *CalculatorService) computeTotalEffects(userCEs []model.CraftEssence, supportCEs []model.CraftEssence, svtId int, diffKey string) (float64, int) {
	totalPercent := 0.0
	totalDirect := 0
	ceEffects := s.repo.GetCeEffects()

	for _, ce := range userCEs {
		if m1, ok := ceEffects[ce.Id]; ok {
			if m2, ok2 := m1[svtId]; ok2 {
				if eff, ok3 := m2[diffKey]; ok3 {
					totalPercent += eff.Percent
					totalDirect += eff.Direct
				}
			}
		}
	}

	for _, ce := range supportCEs {
		if ce.Id == TEATIME_ID {
			totalPercent += 15.0
			continue
		}
		if m1, ok := ceEffects[ce.Id]; ok {
			if m2, ok2 := m1[svtId]; ok2 {
				if eff, ok3 := m2[diffKey]; ok3 {
					totalPercent += eff.Percent
					totalDirect += eff.Direct
				}
			}
		}
	}
	return totalPercent, totalDirect
}

func (s *CalculatorService) FindInPool(id int, cePool []model.CraftEssence, included []model.CraftEssence) bool {
	for _, ce := range cePool {
		if ce.Id == id {
			return true
		}
	}
	for _, ce := range included {
		if ce.Id == id {
			return true
		}
	}
	return false
}

func (s *CalculatorService) FixDominateMap(cePool []model.CraftEssence, included []model.CraftEssence) map[int]int {
	fixedMap := make(map[int]int)
	dominateMap := s.repo.GetDominateMap()
	for B, A := range dominateMap {
		if !s.FindInPool(B, cePool, included) {
			continue
		}
		currentA := A
		for {
			if !s.FindInPool(currentA, cePool, included) {
				if nextA, ok := dominateMap[currentA]; ok {
					currentA = nextA
				} else {
					currentA = -1
					break
				}
			} else {
				break
			}
		}
		if currentA != -1 {
			fixedMap[B] = currentA
		}
	}
	return fixedMap
}

func (s *CalculatorService) GetCombination(num int, includeCe []int, excludeCe []int, serverType string) [][]model.CraftEssence {
	if num < 0 {
		return [][]model.CraftEssence{}
	}
	if num == 0 {
		if len(includeCe) > 0 {
			return [][]model.CraftEssence{}
		}
		return [][]model.CraftEssence{{}}
	}
	includeSet := map[int]bool{}
	excludeSet := map[int]bool{}
	for _, id := range includeCe {
		includeSet[id] = true
	}
	for _, id := range excludeCe {
		excludeSet[id] = true
	}

	craftEssences := s.repo.GetCraftEssences()
	included := []model.CraftEssence{}
	pool := []model.CraftEssence{}
	for _, ce := range craftEssences {
		if ce.Server == "JP" && serverType != "JP" {
			continue
		}
		if excludeSet[ce.Id] {
			continue
		}
		if includeSet[ce.Id] {
			included = append(included, ce)
		} else {
			pool = append(pool, ce)
		}
	}

	if len(pool) < num-len(included) {
		return [][]model.CraftEssence{}
	}

	if len(included) > num {
		return [][]model.CraftEssence{}
	}
	need := num - len(included)
	if need == 0 {
		comb := make([]model.CraftEssence, len(included))
		copy(comb, included)
		return [][]model.CraftEssence{comb}
	}

	sort.Slice(pool, func(i, j int) bool {
		eff1 := 0.0
		if len(pool[i].Filters) > 0 {
			eff1 = pool[i].Filters[0].Effect
		}
		eff2 := 0.0
		if len(pool[j].Filters) > 0 {
			eff2 = pool[j].Filters[0].Effect
		}
		if eff1 != eff2 {
			return eff1 > eff2
		}
		return pool[i].Id < pool[j].Id
	})

	results := [][]model.CraftEssence{}
	fixedDominateMap := s.FixDominateMap(pool, included)
	initialPickedSet := make(map[int]bool)
	for _, ce := range included {
		initialPickedSet[ce.Id] = true
	}

	var dfs func(start int, picked []model.CraftEssence, pickedSet map[int]bool)
	dfs = func(start int, picked []model.CraftEssence, pickedSet map[int]bool) {
		if len(picked) == need {
			comb := make([]model.CraftEssence, 0, num)
			comb = append(comb, included...)
			comb = append(comb, picked...)
			results = append(results, comb)
			return
		}
		remainSlots := need - len(picked)
		for i := start; i <= len(pool)-remainSlots; i++ {
			ce := pool[i]
			if domA, ok := fixedDominateMap[ce.Id]; ok {
				if !pickedSet[domA] {
					continue
				}
			}
			pickedSet[ce.Id] = true
			dfs(i+1, append(picked, pool[i]), pickedSet)
			delete(pickedSet, ce.Id)
		}
	}
	dfs(0, []model.CraftEssence{}, initialPickedSet)
	return results
}

func (s *CalculatorService) getEventBonus(svt *model.Servant, serverType string, selectedEvents map[int]bool) int {
	bonus := 0
	if list, ok := svt.EventBonuses[serverType]; ok {
		for _, b := range list {
			if selectedEvents[b.Id] {
				bonus += b.Bonus
			}
		}
	}
	return bonus
}

func (s *CalculatorService) getEventMultiplier(svt *model.Servant, serverType string, selectedEvents map[int]bool) float64 {
	// 累乘逻辑，可能要fallback
	// multiplier := 1.0
	multiplier := 0.0
	if list, ok := svt.EventExtraBonuses[serverType]; ok {
		for _, b := range list {
			if selectedEvents[b.Id] {
				// multiplier *= float64(b.Bonus) / 100.0
				multiplier += float64(b.Bonus) / 100.0
			}
		}
	}
	return multiplier
}

func (s *CalculatorService) Optimize(costLimit int, svtLimit int, ceLimit int, supportLimit int, includeSupportCe []int, excludeSupportCe []int, allowTraits []int, includeSvt []int, includeSvtDiff []string, excludeSvt []int, includeCe []int, excludeCe []int, baseBond int, serverType string, enableEventBonus bool, selectedEventIds []int) ([]model.TeamResponse, time.Duration) {
	startTime := time.Now()
	log.Println("Optimize called with costLimit:", costLimit, "svtLimit:", svtLimit, "ceLimit:", ceLimit)

	selectedEvents := make(map[int]bool)
	for _, id := range selectedEventIds {
		selectedEvents[id] = true
	}

	if len(includeSvt) > svtLimit {
		return []model.TeamResponse{}, 0
	}
	if len(includeCe) > ceLimit {
		return []model.TeamResponse{}, 0
	}
	if len(includeSupportCe) > supportLimit {
		return []model.TeamResponse{}, 0
	}

	mince := len(includeCe)
	if mince < 0 {
		mince = 0
	}
	mince += (costLimit - svtLimit*16) / 12

	if mince > ceLimit {
		mince = ceLimit
	}

	// Prepare Support CE Pool
	supportPool := s.GetSupportCombinations(supportLimit, serverType, includeSupportCe, excludeSupportCe)
	if len(supportPool) == 0 {
		return []model.TeamResponse{}, 0
	}

	// Prepare User CE Pool
	userCePool := [][]model.CraftEssence{}
	for i := mince; i <= ceLimit; i++ {
		combs := s.GetCombination(i, includeCe, excludeCe, serverType)
		userCePool = append(userCePool, combs...)
	}

	log.Println("User CE Pool: ", len(userCePool))
	log.Println("Support CE Pool: ", len(supportPool))

	svtPool := s.FilterServants(allowTraits, includeSvt, excludeSvt)
	log.Println("Servant Pool: ", len(svtPool))

	includeSvtSet := map[int]bool{}
	for _, id := range includeSvt {
		includeSvtSet[id] = true
	}
	includeSvtDiffMap := make(map[int]string)
	for i, id := range includeSvt {
		if i < len(includeSvtDiff) {
			includeSvtDiffMap[id] = includeSvtDiff[i]
		}
	}

	// Collect all involved CEs (User + Support)
	// We need to scan all POTENTIAL user CEs.
	// Since we stream them now, we don't have them all in a list.
	// But we know the Universe of CEs from repo.

	allRepoCEs := s.repo.GetCraftEssences()
	ceIdToDense := make(map[int]int)
	denseToCeId := []int{}

	for _, ce := range allRepoCEs {
		if _, exists := ceIdToDense[ce.Id]; !exists {
			ceIdToDense[ce.Id] = len(denseToCeId)
			denseToCeId = append(denseToCeId, ce.Id)
		}
	}

	type SimpleEffect struct {
		Percent float64
		Direct  int
	}

	svtDiffEffects := make([]map[string][]SimpleEffect, len(svtPool))
	repoCeEffects := s.repo.GetCeEffects()

	for i, svt := range svtPool {
		svtDiffEffects[i] = make(map[string][]SimpleEffect)
		for key := range svt.Diff {
			effects := make([]SimpleEffect, len(denseToCeId))
			for ceDense, ceId := range denseToCeId {
				eff := SimpleEffect{}
				if m1, ok := repoCeEffects[ceId]; ok {
					if m2, ok2 := m1[svt.Id]; ok2 {
						if e, ok3 := m2[key]; ok3 {
							eff.Percent = e.Percent
							eff.Direct = e.Direct
						}
					}
				}
				effects[ceDense] = eff
			}
			svtDiffEffects[i][key] = effects
		}
	}

	type Job struct {
		UserCEs    []model.CraftEssence
		SupportCEs []model.CraftEssence
	}

	numWorkers := runtime.GOMAXPROCS(0)
	// Batch size
	const BatchSize = 100
	ceJobs := make(chan []Job, numWorkers*2)
	resultsChan := make(chan []model.Team, numWorkers*2)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		// Pre-allocate DP tables for reuse
		maxSvt := svtLimit + 1
		maxCost := costLimit + 1
		dp := make([][]int, maxSvt)
		for i := range dp {
			dp[i] = make([]int, maxCost)
		}
		paths := make([][]*model.PathNode, maxSvt)
		for i := range paths {
			paths[i] = make([]*model.PathNode, maxCost)
		}

		// Reusable slices to avoid allocation
		optionalBonusesBuf := make([]model.SvtBonus, len(svtPool)*4) // *4 for multiple diffs estimate

		for batch := range ceJobs {
			localTeams := []model.Team{}

			for _, job := range batch {
				ceCombo := job.UserCEs
				supportCombo := job.SupportCEs

				ceCost := 0
				// Pre-calculate dense IDs for this combo
				userCeDense := make([]int, len(ceCombo))
				for k, ce := range ceCombo {
					ceCost += ce.Cost
					userCeDense[k] = ceIdToDense[ce.Id]
				}

				// Support CEs
				supportCeDense := make([]int, len(supportCombo))
				supportIsTeatime := make([]bool, len(supportCombo))
				for k, ce := range supportCombo {
					supportCeDense[k] = ceIdToDense[ce.Id]
					if ce.Id == TEATIME_ID {
						supportIsTeatime[k] = true
					}
				}

				if ceCost > costLimit {
					continue
				}

				mandatoryBonuses := []model.SvtBonus{}
				optionalBonuses := optionalBonusesBuf[:0]

				currentSvtLimit := svtLimit
				currentCostLimit := costLimit - ceCost
				validJob := true

				for svtIdx := 0; svtIdx < len(svtPool); svtIdx++ {
					svt := &svtPool[svtIdx]

					getTotalEffect := func(effSlice []SimpleEffect) (float64, int) {
						tPercent := 0.0
						tDirect := 0

						// User CEs
						for _, idx := range userCeDense {
							e := effSlice[idx]
							tPercent += e.Percent
							tDirect += e.Direct
						}
						// Support CEs
						for k, idx := range supportCeDense {
							if supportIsTeatime[k] {
								tPercent += 15.0
								continue
							}
							e := effSlice[idx]
							tPercent += e.Percent
							tDirect += e.Direct
						}
						return tPercent, tDirect
					}

					if includeSvtSet[svt.Id] {
						// Mandatory
						diffKey := "default"
						if k, ok := includeSvtDiffMap[svt.Id]; ok {
							diffKey = k
						}

						if detail, ok := svt.Diff[diffKey]; ok {
							// Lookup effect slice
							effSlice := svtDiffEffects[svtIdx][diffKey]
							totalPercent, totalDirect := getTotalEffect(effSlice)

							if enableEventBonus {
								totalPercent += float64(s.getEventBonus(svt, serverType, selectedEvents))

								// convert independent multiplier to additive percentage
								multiplier := s.getEventMultiplier(svt, serverType, selectedEvents)
								if multiplier > 0 {
									totalPercent += math.Round((multiplier - 1.0) * 100.0)
								}
							}
							bonus := int(float64(baseBond)*totalPercent/100.0) + totalDirect + baseBond
							// if enableEventBonus {
							// 	bonus = int(float64(bonus) * s.getEventMultiplier(svt, serverType, selectedEvents))
							// }
							mandatoryBonuses = append(mandatoryBonuses, model.SvtBonus{
								Svt:     svt,
								DiffKey: diffKey,
								Bonus:   bonus,
								Cost:    detail.Cost,
							})
						} else {
							// Fallback logic
							bestBonus := -1
							bestDiffKey := "default"
							bestCost := svt.Diff["default"].Cost

							for key, detail := range svt.Diff {
								effSlice := svtDiffEffects[svtIdx][key]
								totalPercent, totalDirect := getTotalEffect(effSlice)

								if enableEventBonus {
									totalPercent += float64(s.getEventBonus(svt, serverType, selectedEvents))

									// convert independent multiplier to additive percentage
									multiplier := s.getEventMultiplier(svt, serverType, selectedEvents)
									if multiplier > 0 {
										totalPercent += math.Round((multiplier - 1.0) * 100.0)
									}
								}
								b := int(float64(baseBond)*totalPercent/100.0) + totalDirect + baseBond
								// if enableEventBonus {
								// 	b = int(float64(b) * s.getEventMultiplier(svt, serverType, selectedEvents))
								// }
								if b > bestBonus || (b == bestBonus && detail.Cost < bestCost) {
									bestBonus = b
									bestDiffKey = key
									bestCost = detail.Cost
								}
							}
							mandatoryBonuses = append(mandatoryBonuses, model.SvtBonus{
								Svt:     svt,
								DiffKey: bestDiffKey,
								Bonus:   bestBonus,
								Cost:    bestCost,
							})
						}
					} else {
						// Optional
						bestBonus := -1
						bestDiffKey := "default"
						bestCost := svt.Diff["default"].Cost

						for key, detail := range svt.Diff {
							effSlice := svtDiffEffects[svtIdx][key]
							totalPercent, totalDirect := getTotalEffect(effSlice)

							if enableEventBonus {
								totalPercent += float64(s.getEventBonus(svt, serverType, selectedEvents))

								// convert independent multiplier to additive percentage
								multiplier := s.getEventMultiplier(svt, serverType, selectedEvents)
								if multiplier > 0 {
									totalPercent += math.Round((multiplier - 1.0) * 100.0)
								}
							}
							b := int(float64(baseBond)*totalPercent/100.0) + totalDirect + baseBond
							// if enableEventBonus {
							// 	b = int(float64(b) * s.getEventMultiplier(svt, serverType, selectedEvents))
							// }
							if b > bestBonus || (b == bestBonus && detail.Cost < bestCost) {
								bestBonus = b
								bestDiffKey = key
								bestCost = detail.Cost
							}
						}
						optionalBonuses = append(optionalBonuses, model.SvtBonus{
							Svt:     svt,
							DiffKey: bestDiffKey,
							Bonus:   bestBonus,
							Cost:    bestCost,
						})
					}
				}

				// Sum Mandatory Costs
				mandatoryCost := 0
				mandatoryBond := 0
				for _, mb := range mandatoryBonuses {
					mandatoryCost += mb.Cost
					mandatoryBond += mb.Bonus
				}

				currentCostLimit = costLimit - ceCost - mandatoryCost
				currentSvtLimit = svtLimit - len(mandatoryBonuses)

				if currentCostLimit < 0 || currentSvtLimit < 0 {
					validJob = false
				}

				if !validJob {
					continue
				}

				if currentSvtLimit == 0 {
					team := model.Team{
						CraftEssences:        ceCombo,
						SupportCraftEssences: supportCombo,
						TotalBond:            mandatoryBond,
						TotalCost:            ceCost + mandatoryCost,
					}
					for _, sb := range mandatoryBonuses {
						team.Servants = append(team.Servants, sb.Svt)
						team.DiffChoice = append(team.DiffChoice, sb.DiffKey)
					}
					localTeams = append(localTeams, team)
					continue
				}

				if len(optionalBonuses) == 0 {
					team := model.Team{
						CraftEssences:        ceCombo,
						SupportCraftEssences: supportCombo,
						TotalBond:            mandatoryBond,
						TotalCost:            ceCost + mandatoryCost,
					}
					for _, sb := range mandatoryBonuses {
						team.Servants = append(team.Servants, sb.Svt)
						team.DiffChoice = append(team.DiffChoice, sb.DiffKey)
					}
					localTeams = append(localTeams, team)
					continue
				}

				// DP
				const NEG = -1 << 60
				// Reset DP tables
				for i := 0; i <= currentSvtLimit; i++ {
					for j := 0; j <= currentCostLimit; j++ {
						dp[i][j] = NEG
						paths[i][j] = nil
					}
				}
				dp[0][0] = 0

				for itemIdx, item := range optionalBonuses {
					cost := item.Cost
					bonus := item.Bonus
					if cost > currentCostLimit {
						continue
					}
					for k := currentSvtLimit; k >= 1; k-- {
						for j := currentCostLimit; j >= cost; j-- {
							if dp[k-1][j-cost] == NEG {
								continue
							}
							newBond := dp[k-1][j-cost] + bonus
							if newBond > dp[k][j] {
								dp[k][j] = newBond
								paths[k][j] = &model.PathNode{
									ItemIdx: itemIdx,
									Prev:    paths[k-1][j-cost],
								}
							}
						}
					}
				}

				for k := 1; k <= currentSvtLimit; k++ {
					for j := 0; j <= currentCostLimit; j++ {
						if dp[k][j] == NEG {
							continue
						}
						used := make([]bool, len(optionalBonuses))
						node := paths[k][j]
						for node != nil {
							used[node.ItemIdx] = true
							node = node.Prev
						}
						chosen := []model.SvtBonus{}
						totalCost := ceCost + mandatoryCost
						for idx, flag := range used {
							if flag {
								sb := optionalBonuses[idx]
								chosen = append(chosen, sb)
								totalCost += sb.Cost
							}
						}

						team := model.Team{
							CraftEssences:        ceCombo,
							SupportCraftEssences: supportCombo,
							TotalBond:            mandatoryBond,
						}
						for _, sb := range mandatoryBonuses {
							team.Servants = append(team.Servants, sb.Svt)
							team.DiffChoice = append(team.DiffChoice, sb.DiffKey)
						}
						for _, sb := range chosen {
							team.Servants = append(team.Servants, sb.Svt)
							team.DiffChoice = append(team.DiffChoice, sb.DiffKey)
							team.TotalBond += sb.Bonus
						}
						team.TotalCost = totalCost
						localTeams = append(localTeams, team)
					}
				}
			} // end batch loop

			if len(localTeams) > 0 {
				resultsChan <- localTeams
			}
		}
	}

	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go worker()
	}

	go func() {
		batch := make([]Job, 0, BatchSize)
		for _, userCEs := range userCePool {
			for _, supportCEs := range supportPool {
				batch = append(batch, Job{UserCEs: userCEs, SupportCEs: supportCEs})
				if len(batch) >= BatchSize {
					ceJobs <- batch
					batch = make([]Job, 0, BatchSize)
				}
			}
		}
		if len(batch) > 0 {
			ceJobs <- batch
		}
		close(ceJobs)
	}()

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	h := &model.TeamHeap{}
	heap.Init(h)

	for teams := range resultsChan {
		for _, team := range teams {
			if h.Len() < OPTIMIZE_LIMIT {
				heap.Push(h, team)
			} else {
				top := (*h)[0]
				if team.TotalBond > top.TotalBond || (team.TotalBond == top.TotalBond && team.TotalCost > top.TotalCost) {
					(*h)[0] = team
					heap.Fix(h, 0)
				}
			}
		}
	}

	limit := h.Len()
	sortedTeams := make([]model.Team, limit)
	for i := limit - 1; i >= 0; i-- {
		sortedTeams[i] = heap.Pop(h).(model.Team)
	}

	finalResults := make([]model.TeamResponse, 0, limit)
	ceEffects := s.repo.GetCeEffects()

	for i := 0; i < limit; i++ {
		team := sortedTeams[i]
		svtIds := make([]int, len(team.Servants))
		for k, s := range team.Servants {
			svtIds[k] = s.Id
		}

		response := model.TeamResponse{
			Servants:             svtIds,
			DiffChoice:           team.DiffChoice,
			TotalCost:            team.TotalCost,
			TotalBond:            team.TotalBond,
			CraftEssences:        make([]model.TeamResultCE, len(team.CraftEssences)),
			SupportCraftEssences: make([]model.TeamResultCE, len(team.SupportCraftEssences)),
		}

		for j, ce := range team.CraftEssences {
			totalContribution := 0
			for k, svt := range team.Servants {
				diffKey := team.DiffChoice[k]
				if m1, ok := ceEffects[ce.Id]; ok {
					if m2, ok2 := m1[svt.Id]; ok2 {
						if eff, ok3 := m2[diffKey]; ok3 {
							totalContribution += int(float64(baseBond)*eff.Percent/100.0) + eff.Direct
						}
					}
				}
			}
			response.CraftEssences[j] = model.TeamResultCE{
				Id:           ce.Id,
				Contribution: totalContribution,
			}
		}

		// Fill Support CE details in response
		for j, ce := range team.SupportCraftEssences {
			totalContribution := 0
			for k, svt := range team.Servants {
				diffKey := team.DiffChoice[k]
				if ce.Id == TEATIME_ID {
					totalContribution += int(float64(baseBond) * 15.0 / 100.0)
				} else {
					if m1, ok := ceEffects[ce.Id]; ok {
						if m2, ok2 := m1[svt.Id]; ok2 {
							if eff, ok3 := m2[diffKey]; ok3 {
								totalContribution += int(float64(baseBond)*eff.Percent/100.0) + eff.Direct
							}
						}
					}
				}
			}
			response.SupportCraftEssences[j] = model.TeamResultCE{
				Id:           ce.Id,
				Contribution: totalContribution,
			}
		}

		finalResults = append(finalResults, response)
	}

	return finalResults, time.Since(startTime)
}
