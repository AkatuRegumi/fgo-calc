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

func (s *CalculatorService) FilterServants(traits []int, includeSvt []int, excludeSvt []int, serverType string) []model.Servant {
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

	servants := s.repo.GetServants(serverType)
	for _, svt := range servants {
		if includeSet[svt.Id] {
			result = append(result, svt)
			continue
		}
		if excludeSet[svt.Id] {
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
	for _, id := range includeSupportCe {
		delete(excludeSet, id)
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
	for _, id := range includeCe {
		delete(excludeSet, id)
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

func (s *CalculatorService) getEventPartyBonus(svt *model.Servant, serverType string, selectedEvents map[int]bool) int {
	bonus := 0
	if list, ok := svt.EventPartyBonuses[serverType]; ok {
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

func (s *CalculatorService) Optimize(costLimit int, svtLimit int, ceLimit int, supportLimit int, includeSupportCe []int, excludeSupportCe []int, allowTraits []int, includeSvt []int, includeSvtDiff []string, excludeSvt []int, includeCe []int, excludeCe []int, baseBond int, serverType string, enableEventBonus bool, selectedEventIds []int, bond15Svt []int, bond15Full []bool) ([]model.TeamResponse, time.Duration) {
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

	svtPool := s.FilterServants(allowTraits, includeSvt, excludeSvt, serverType)
	log.Println("Servant Pool: ", len(svtPool))
	availableSvt := make(map[int]struct{}, len(svtPool))
	for _, servant := range svtPool {
		availableSvt[servant.Id] = struct{}{}
	}
	for _, id := range includeSvt {
		if _, ok := availableSvt[id]; !ok {
			return []model.TeamResponse{}, 0
		}
	}

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

	// 15绊配置：已满（full）的从者自身不再获得羁绊，仅提供全队+25%；
	// 未满（如日服已开放16绊）的从者正常获得羁绊，同时提供全队+25%。
	// 被排除或不在候选池中的从者天然不参与，无需特判。
	bond15FullSet := map[int]bool{}
	bond15NotFullSet := map[int]bool{}
	for i, id := range bond15Svt {
		full := true
		if i < len(bond15Full) {
			full = bond15Full[i]
		}
		if full {
			bond15FullSet[id] = true
			delete(bond15NotFullSet, id)
		} else {
			bond15NotFullSet[id] = true
			delete(bond15FullSet, id)
		}
	}

	// 已满15绊的“纯buff位”候选：彼此同质（收益0，仅cost不同），按cost升序预排序，
	// 出解阶段按个数p取cost最低的p个补入。必选项和活动全局buff提供者不列入：
	// 前者作为mandatory处理，后者由partyBonusStates枚举其入队状态。
	type bond15Provider struct {
		svt     *model.Servant
		diffKey string
		cost    int
	}
	bond15Providers := []bond15Provider{}
	for i := range svtPool {
		svt := &svtPool[i]
		if !bond15FullSet[svt.Id] || includeSvtSet[svt.Id] {
			continue
		}
		if enableEventBonus && s.getEventPartyBonus(svt, serverType, selectedEvents) > 0 {
			continue
		}
		bestKey := ""
		bestCost := math.MaxInt32
		for key, detail := range svt.Diff {
			if detail.Cost < bestCost {
				bestCost = detail.Cost
				bestKey = key
			}
		}
		bond15Providers = append(bond15Providers, bond15Provider{svt: svt, diffKey: bestKey, cost: bestCost})
	}
	sort.Slice(bond15Providers, func(i, j int) bool {
		if bond15Providers[i].cost != bond15Providers[j].cost {
			return bond15Providers[i].cost < bond15Providers[j].cost
		}
		return bond15Providers[i].svt.Id < bond15Providers[j].svt.Id
	})
	providerCostPrefix := make([]int, len(bond15Providers)+1)
	for i, provider := range bond15Providers {
		providerCostPrefix[i+1] = providerCostPrefix[i] + provider.cost
	}

	// 未满15绊从者保留在常规候选池中，仅额外标记provider身份，用DP的q维记录入队数量
	bond15NotFullCount := 0
	for i := range svtPool {
		if bond15NotFullSet[svtPool[i].Id] {
			bond15NotFullCount++
		}
	}
	qSize := min(svtLimit, bond15NotFullCount) + 1

	type partyBonusState struct {
		selected map[int]bool
		bonus    int
	}
	partyBonusProviders := []struct {
		id    int
		bonus int
	}{}
	if enableEventBonus {
		for i := range svtPool {
			if bonus := s.getEventPartyBonus(&svtPool[i], serverType, selectedEvents); bonus > 0 {
				partyBonusProviders = append(partyBonusProviders, struct {
					id    int
					bonus int
				}{svtPool[i].Id, bonus})
			}
		}
	}
	partyBonusStates := []partyBonusState{}
	var buildPartyBonusStates func(int, map[int]bool, int)
	buildPartyBonusStates = func(index int, selected map[int]bool, bonus int) {
		if len(selected) > svtLimit {
			return
		}
		if index == len(partyBonusProviders) {
			for _, provider := range partyBonusProviders {
				if includeSvtSet[provider.id] && !selected[provider.id] {
					return
				}
			}
			stateSelected := make(map[int]bool, len(selected))
			for id := range selected {
				stateSelected[id] = true
			}
			partyBonusStates = append(partyBonusStates, partyBonusState{selected: stateSelected, bonus: bonus})
			return
		}

		provider := partyBonusProviders[index]
		if !includeSvtSet[provider.id] {
			buildPartyBonusStates(index+1, selected, bonus)
		}
		selected[provider.id] = true
		buildPartyBonusStates(index+1, selected, bonus+provider.bonus)
		delete(selected, provider.id)
	}
	buildPartyBonusStates(0, map[int]bool{}, 0)

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
	repoCeEffects := s.repo.GetCeEffects(serverType)

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
	supportDense := make([][]int, len(supportPool))
	supportTeatime := make([][]bool, len(supportPool))
	for i, supportCombo := range supportPool {
		supportDense[i] = make([]int, len(supportCombo))
		supportTeatime[i] = make([]bool, len(supportCombo))
		for k, ce := range supportCombo {
			supportDense[i][k] = ceIdToDense[ce.Id]
			supportTeatime[i][k] = ce.Id == TEATIME_ID
		}
	}

	type Job struct {
		UserCEs []model.CraftEssence
	}

	numWorkers := runtime.GOMAXPROCS(0)
	// Batch size
	const BatchSize = 100
	ceJobs := make(chan []Job, numWorkers*2)
	resultsChan := make(chan []model.Team, numWorkers*2)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		type pathSelection [6]uint16
		type teamCandidate struct {
			count   int
			q       int
			p       int
			cost    int
			bond    int
			bonus15 int
		}
		isBetterCandidate := func(a, b teamCandidate) bool {
			return a.bond > b.bond || (a.bond == b.bond && a.cost > b.cost)
		}
		addCandidate := func(candidates []teamCandidate, candidate teamCandidate) []teamCandidate {
			if len(candidates) < OPTIMIZE_LIMIT {
				return append(candidates, candidate)
			}
			worst := 0
			for i := 1; i < len(candidates); i++ {
				if isBetterCandidate(candidates[worst], candidates[i]) {
					worst = i
				}
			}
			if isBetterCandidate(candidate, candidates[worst]) {
				candidates[worst] = candidate
			}
			return candidates
		}
		addLocalTeam := func(teams []model.Team, team model.Team) []model.Team {
			if len(teams) < OPTIMIZE_LIMIT {
				return append(teams, team)
			}
			worst := 0
			for i := 1; i < len(teams); i++ {
				if teams[worst].TotalBond > teams[i].TotalBond ||
					(teams[worst].TotalBond == teams[i].TotalBond && teams[worst].TotalCost > teams[i].TotalCost) {
					worst = i
				}
			}
			if team.TotalBond > teams[worst].TotalBond ||
				(team.TotalBond == teams[worst].TotalBond && team.TotalCost > teams[worst].TotalCost) {
				teams[worst] = team
			}
			return teams
		}

		// Pre-allocate DP tables for reuse
		maxSvt := svtLimit + 1
		maxCost := costLimit + 1
		dp := make([][][]int, maxSvt)
		nextDP := make([][][]int, maxSvt)
		for i := range dp {
			dp[i] = make([][]int, qSize)
			nextDP[i] = make([][]int, qSize)
			for q := 0; q < qSize; q++ {
				dp[i][q] = make([]int, maxCost)
				nextDP[i][q] = make([]int, maxCost)
			}
		}
		paths := make([][][]pathSelection, maxSvt)
		nextPaths := make([][][]pathSelection, maxSvt)
		for i := range paths {
			paths[i] = make([][]pathSelection, qSize)
			nextPaths[i] = make([][]pathSelection, qSize)
			for q := 0; q < qSize; q++ {
				paths[i][q] = make([]pathSelection, maxCost)
				nextPaths[i][q] = make([]pathSelection, maxCost)
			}
		}
		// 出解阶段的cost维前缀最优表（bond相同取cost较高者，与候选比较规则一致）
		prefBond := make([][][]int, maxSvt)
		prefJ := make([][][]int, maxSvt)
		for i := range prefBond {
			prefBond[i] = make([][]int, qSize)
			prefJ[i] = make([][]int, qSize)
			for q := 0; q < qSize; q++ {
				prefBond[i][q] = make([]int, maxCost)
				prefJ[i][q] = make([]int, maxCost)
			}
		}

		// Reusable slices to avoid allocation
		optionalBonusesBuf := make([]model.SvtBonus, len(svtPool)*4) // *4 for multiple diffs estimate
		userEffectTotals := make([]map[string]SimpleEffect, len(svtPool))
		for svtIdx, svt := range svtPool {
			userEffectTotals[svtIdx] = make(map[string]SimpleEffect, len(svt.Diff))
		}

		for batch := range ceJobs {
			localTeams := make([]model.Team, 0, OPTIMIZE_LIMIT)

			for _, job := range batch {
				ceCombo := job.UserCEs

				ceCost := 0
				// Pre-calculate dense IDs for this combo
				userCeDense := make([]int, len(ceCombo))
				for k, ce := range ceCombo {
					ceCost += ce.Cost
					userCeDense[k] = ceIdToDense[ce.Id]
				}

				if ceCost > costLimit {
					continue
				}
				for svtIdx := range svtPool {
					for key, effSlice := range svtDiffEffects[svtIdx] {
						total := SimpleEffect{}
						for _, idx := range userCeDense {
							effect := effSlice[idx]
							total.Percent += effect.Percent
							total.Direct += effect.Direct
						}
						userEffectTotals[svtIdx][key] = total
					}
				}

				for supportIdx, supportCombo := range supportPool {
					supportCeDense := supportDense[supportIdx]
					supportIsTeatime := supportTeatime[supportIdx]
					for _, partyState := range partyBonusStates {

						mandatoryBonuses := []model.SvtBonus{}
						optionalBonuses := optionalBonusesBuf[:0]

						currentSvtLimit := svtLimit
						currentCostLimit := costLimit - ceCost
						validJob := true
						mandatoryProviders := 0
						mandatoryFull := 0

						for svtIdx := 0; svtIdx < len(svtPool); svtIdx++ {
							svt := &svtPool[svtIdx]
							isPartyBonusProvider := enableEventBonus && s.getEventPartyBonus(svt, serverType, selectedEvents) > 0
							if isPartyBonusProvider && !partyState.selected[svt.Id] {
								continue
							}
							isBond15Full := bond15FullSet[svt.Id]
							isBond15NotFull := bond15NotFullSet[svt.Id]

							getTotalEffect := func(diffKey string, effSlice []SimpleEffect) (float64, int) {
								total := userEffectTotals[svtIdx][diffKey]
								tPercent := total.Percent
								tDirect := total.Direct
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

							if includeSvtSet[svt.Id] || partyState.selected[svt.Id] {
								// Mandatory
								if isBond15Full {
									// 已满15绊：自身不再获得羁绊，仅占位并提供全队+25%
									diffKey := ""
									if k, ok := includeSvtDiffMap[svt.Id]; ok {
										diffKey = k
									}
									detail, ok := svt.Diff[diffKey]
									if !ok {
										bestFullCost := math.MaxInt32
										for key, d := range svt.Diff {
											if d.Cost < bestFullCost {
												bestFullCost = d.Cost
												diffKey = key
											}
										}
										detail = svt.Diff[diffKey]
									}
									mandatoryBonuses = append(mandatoryBonuses, model.SvtBonus{
										Svt:     svt,
										DiffKey: diffKey,
										Bonus:   0,
										Cost:    detail.Cost,
									})
									mandatoryProviders++
									mandatoryFull++
									continue
								}
								if isBond15NotFull {
									mandatoryProviders++
								}
								diffKey := ""
								if includeSvtSet[svt.Id] {
									diffKey = "default"
									if k, ok := includeSvtDiffMap[svt.Id]; ok {
										diffKey = k
									}
								}

								if detail, ok := svt.Diff[diffKey]; ok {
									// Lookup effect slice
									effSlice := svtDiffEffects[svtIdx][diffKey]
									totalPercent, totalDirect := getTotalEffect(diffKey, effSlice)

									if enableEventBonus {
										totalPercent += float64(s.getEventBonus(svt, serverType, selectedEvents))
										totalPercent += float64(partyState.bonus)

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
										Svt:        svt,
										DiffKey:    diffKey,
										Bonus:      bonus,
										Cost:       detail.Cost,
										IsProvider: isBond15NotFull,
									})
								} else {
									// Fallback logic
									bestBonus := -1
									bestDiffKey := "default"
									bestCost := svt.Diff["default"].Cost

									for key, detail := range svt.Diff {
										effSlice := svtDiffEffects[svtIdx][key]
										totalPercent, totalDirect := getTotalEffect(key, effSlice)

										if enableEventBonus {
											totalPercent += float64(s.getEventBonus(svt, serverType, selectedEvents))
											totalPercent += float64(partyState.bonus)

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
										Svt:        svt,
										DiffKey:    bestDiffKey,
										Bonus:      bestBonus,
										Cost:       bestCost,
										IsProvider: isBond15NotFull,
									})
								}
							} else {
								// Optional
								if isBond15Full {
									// 已满15绊自身无收益，不作为常规候选，统一在出解阶段按个数补入
									continue
								}
								bestBonus := -1
								bestDiffKey := "default"
								bestCost := svt.Diff["default"].Cost

								for key, detail := range svt.Diff {
									effSlice := svtDiffEffects[svtIdx][key]
									totalPercent, totalDirect := getTotalEffect(key, effSlice)

									if enableEventBonus {
										totalPercent += float64(s.getEventBonus(svt, serverType, selectedEvents))
										totalPercent += float64(partyState.bonus)

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
									Svt:        svt,
									DiffKey:    bestDiffKey,
									Bonus:      bestBonus,
									Cost:       bestCost,
									IsProvider: isBond15NotFull,
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
						mandatoryEarners := len(mandatoryBonuses) - mandatoryFull

						currentCostLimit = costLimit - ceCost - mandatoryCost
						currentSvtLimit = svtLimit - len(mandatoryBonuses)

						if currentCostLimit < 0 || currentSvtLimit < 0 {
							validJob = false
						}

						if !validJob {
							continue
						}

						// DP
						const NEG = -1 << 60
						providerCount := 0
						for _, item := range optionalBonuses {
							if item.IsProvider {
								providerCount++
							}
						}
						qMax := min(currentSvtLimit, providerCount)
						// Reset DP tables
						for i := 0; i <= currentSvtLimit; i++ {
							for q := 0; q <= qMax; q++ {
								for j := 0; j <= currentCostLimit; j++ {
									dp[i][q][j] = NEG
									paths[i][q][j] = pathSelection{}
								}
							}
						}
						dp[0][0][0] = 0

						costGroups := make([][]int, currentCostLimit+1)
						for itemIdx, item := range optionalBonuses {
							if item.Cost > currentCostLimit {
								continue
							}
							group := costGroups[item.Cost]
							insertAt := len(group)
							for i, existingIdx := range group {
								if item.Bonus > optionalBonuses[existingIdx].Bonus {
									insertAt = i
									break
								}
							}
							if insertAt >= currentSvtLimit {
								continue
							}
							group = append(group, 0)
							copy(group[insertAt+1:], group[insertAt:])
							group[insertAt] = itemIdx
							if len(group) > currentSvtLimit {
								group = group[:currentSvtLimit]
							}
							costGroups[item.Cost] = group
						}
						// 每个cost组内按收益降序排列，记录前t个物品中未满15绊provider的数量
						costGroupProviders := make([][]int, currentCostLimit+1)
						for cost, group := range costGroups {
							if len(group) == 0 {
								continue
							}
							prov := make([]int, len(group)+1)
							for take, itemIdx := range group {
								prov[take+1] = prov[take]
								if optionalBonuses[itemIdx].IsProvider {
									prov[take+1]++
								}
							}
							costGroupProviders[cost] = prov
						}

						for cost, group := range costGroups {
							if len(group) == 0 {
								continue
							}
							prov := costGroupProviders[cost]
							for k := 0; k <= currentSvtLimit; k++ {
								for q := 0; q <= qMax; q++ {
									for j := 0; j <= currentCostLimit; j++ {
										nextDP[k][q][j] = NEG
										nextPaths[k][q][j] = pathSelection{}
									}
								}
							}
							for k := 0; k <= currentSvtLimit; k++ {
								for q := 0; q <= qMax; q++ {
									for j := 0; j <= currentCostLimit; j++ {
										if dp[k][q][j] == NEG {
											continue
										}
										bond := dp[k][q][j]
										selection := paths[k][q][j]
										maxTake := min(len(group), currentSvtLimit-k)
										for take := 0; take <= maxTake; take++ {
											newQ := q + prov[take]
											if newQ > qMax {
												break
											}
											newCost := j + take*cost
											if newCost > currentCostLimit {
												break
											}
											if take > 0 {
												itemIdx := group[take-1]
												bond += optionalBonuses[itemIdx].Bonus
												selection[k+take-1] = uint16(itemIdx + 1)
											}
											if bond > nextDP[k+take][newQ][newCost] {
												nextDP[k+take][newQ][newCost] = bond
												nextPaths[k+take][newQ][newCost] = selection
											}
										}
									}
								}
							}
							dp, nextDP = nextDP, dp
							paths, nextPaths = nextPaths, paths
						}

						// 出解：先对每个(人数k, 未满15绊数q)在cost维求前缀最优，
						// 再枚举补入p个已满15绊位（彼此同质，取cost最低的p个）。
						// 15绊全队收益 = 吃羁绊人数E × int(base×25%×P)，
						// 其中 E = 必选收益人数+k，P = 必选15绊数+q+p，只依赖计数，
						// 与cost无关，因此对每个(k,q,p)取cost维前缀最优是精确的。
						for k := 0; k <= currentSvtLimit; k++ {
							for q := 0; q <= qMax; q++ {
								best := NEG
								bestJ := 0
								for j := 0; j <= currentCostLimit; j++ {
									if dp[k][q][j] != NEG && dp[k][q][j] >= best {
										best = dp[k][q][j]
										bestJ = j
									}
									prefBond[k][q][j] = best
									prefJ[k][q][j] = bestJ
								}
							}
						}

						candidates := make([]teamCandidate, 0, OPTIMIZE_LIMIT)
						for k := 0; k <= currentSvtLimit; k++ {
							earners := mandatoryEarners + k
							if earners == 0 {
								// 纯已满15绊的队伍没有任何羁绊收益
								continue
							}
							for q := 0; q <= qMax; q++ {
								for p := 0; p <= len(bond15Providers) && p+k <= currentSvtLimit; p++ {
									jLimit := currentCostLimit - providerCostPrefix[p]
									if jLimit < 0 {
										break
									}
									best := prefBond[k][q][jLimit]
									if best == NEG {
										continue
									}
									providers := mandatoryProviders + q + p
									bonus15 := 0
									if providers > 0 {
										bonus15 = earners * int(float64(baseBond)*25.0*float64(providers)/100.0)
									}
									candidates = addCandidate(candidates, teamCandidate{
										count:   k,
										q:       q,
										p:       p,
										cost:    ceCost + mandatoryCost + prefJ[k][q][jLimit] + providerCostPrefix[p],
										bond:    mandatoryBond + best + bonus15,
										bonus15: bonus15,
									})
								}
							}
						}

						for _, candidate := range candidates {
							team := model.Team{
								CraftEssences:        ceCombo,
								SupportCraftEssences: supportCombo,
								TotalBond:            candidate.bond,
								TotalCost:            candidate.cost,
								Bond15Bonus:          candidate.bonus15,
							}
							for _, sb := range mandatoryBonuses {
								team.Servants = append(team.Servants, sb.Svt)
								team.DiffChoice = append(team.DiffChoice, sb.DiffKey)
							}
							j := candidate.cost - ceCost - mandatoryCost - providerCostPrefix[candidate.p]
							selection := paths[candidate.count][candidate.q][j]
							for i := 0; i < candidate.count; i++ {
								itemIdx := int(selection[i]) - 1
								if itemIdx < 0 {
									continue
								}
								sb := optionalBonuses[itemIdx]
								team.Servants = append(team.Servants, sb.Svt)
								team.DiffChoice = append(team.DiffChoice, sb.DiffKey)
							}
							for i := 0; i < candidate.p; i++ {
								provider := bond15Providers[i]
								team.Servants = append(team.Servants, provider.svt)
								team.DiffChoice = append(team.DiffChoice, provider.diffKey)
							}
							localTeams = addLocalTeam(localTeams, team)
						}
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
		batchSize := BatchSize / len(supportPool)
		if batchSize < 1 {
			batchSize = 1
		}
		batch := make([]Job, 0, batchSize)
		for _, userCEs := range userCePool {
			batch = append(batch, Job{UserCEs: userCEs})
			if len(batch) >= batchSize {
				ceJobs <- batch
				batch = make([]Job, 0, batchSize)
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
	ceEffects := s.repo.GetCeEffects(serverType)

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
			Bond15Bonus:          team.Bond15Bonus,
			CraftEssences:        make([]model.TeamResultCE, len(team.CraftEssences)),
			SupportCraftEssences: make([]model.TeamResultCE, len(team.SupportCraftEssences)),
		}

		for j, ce := range team.CraftEssences {
			totalContribution := 0
			for k, svt := range team.Servants {
				if bond15FullSet[svt.Id] {
					// 已满15绊从者自身无收益，不计入礼装贡献
					continue
				}
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
				if bond15FullSet[svt.Id] {
					continue
				}
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
