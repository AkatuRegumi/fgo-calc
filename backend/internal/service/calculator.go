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
	// Explicit include wins over exclusion, matching the upstream contract.
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
	// Explicit include wins over exclusion, matching the upstream contract.
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

func (s *CalculatorService) Optimize(costLimit int, svtLimit int, ceLimit int, supportLimit int, includeSupportCe []int, excludeSupportCe []int, allowTraits []int, includeSvt []int, includeSvtDiff []string, excludeSvt []int, includeCe []int, excludeCe []int, baseBond int, serverType string, enableEventBonus bool, selectedEventIds []int) ([]model.TeamResponse, time.Duration) {
	return s.OptimizeAdvanced(costLimit, svtLimit, ceLimit, supportLimit, includeSupportCe, excludeSupportCe, allowTraits, includeSvt, includeSvtDiff, excludeSvt, includeCe, excludeCe, baseBond, serverType, enableEventBonus, selectedEventIds, false, nil)
}

func (s *CalculatorService) OptimizeAdvanced(costLimit int, svtLimit int, ceLimit int, supportLimit int, includeSupportCe []int, excludeSupportCe []int, allowTraits []int, includeSvt []int, includeSvtDiff []string, excludeSvt []int, includeCe []int, excludeCe []int, baseBond int, serverType string, enableEventBonus bool, selectedEventIds []int, grandMode bool, bond15Svt []int) ([]model.TeamResponse, time.Duration) {
	return s.OptimizeAdvancedWithProfile(costLimit, svtLimit, ceLimit, supportLimit, includeSupportCe, excludeSupportCe, allowTraits, includeSvt, includeSvtDiff, excludeSvt, includeCe, excludeCe, baseBond, serverType, enableEventBonus, selectedEventIds, grandMode, bond15Svt, nil, "max", nil)
}

func normalizeOptimizationMode(mode string) string {
	switch mode {
	case "balanced", "finish":
		return mode
	default:
		return "max"
	}
}

func preferenceWeight(profile model.ServantOptimizationProfile, mode string) float64 {
	if mode == "max" {
		return 1.0
	}
	roleWeight := 1.0
	switch profile.Role {
	case "driver":
		roleWeight = 0.55
	case "passenger":
		roleWeight = 1.25
	}
	priorityWeight := 1.0
	switch profile.Priority {
	case "high":
		priorityWeight = 1.35
	case "low":
		priorityWeight = 0.70
	}

	progressKnown := false
	progress := 0.0
	if profile.TargetTotal > 0 && profile.BondTotal >= 0 {
		progressKnown = true
		progress = float64(profile.BondTotal) / float64(profile.TargetTotal)
	} else if profile.TargetRank > 0 && profile.BondRank > 0 {
		progressKnown = true
		progress = float64(profile.BondRank) / float64(profile.TargetRank)
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	progressWeight := 1.0
	if progressKnown {
		if progress >= 1.0 {
			// 已达到当前培养目标时不再把它当成“老板”优先投喂；固定出场仍由硬约束保证。
			progressWeight = 0.15
		} else if mode == "balanced" {
			// 越缺羁绊越值得占用稀缺后排位：低进度约1.35，接近目标约0.90。
			progressWeight = 1.35 - 0.45*progress
		} else {
			// 收尾模式反过来偏向接近目标但尚未完成的从者。
			progressWeight = 0.85 + 0.85*progress
		}
	}
	return roleWeight * priorityWeight * progressWeight
}

func (s *CalculatorService) OptimizeAdvancedWithProfile(costLimit int, svtLimit int, ceLimit int, supportLimit int, includeSupportCe []int, excludeSupportCe []int, allowTraits []int, includeSvt []int, includeSvtDiff []string, excludeSvt []int, includeCe []int, excludeCe []int, baseBond int, serverType string, enableEventBonus bool, selectedEventIds []int, grandMode bool, bond15Svt []int, bond10Svt []int, optimizationMode string, optimizationProfiles []model.ServantOptimizationProfile) ([]model.TeamResponse, time.Duration) {
	startTime := time.Now()
	log.Println("Optimize called with costLimit:", costLimit, "svtLimit:", svtLimit, "ceLimit:", ceLimit)

	selectedEvents := make(map[int]bool)
	for _, id := range selectedEventIds {
		selectedEvents[id] = true
	}
	optimizationMode = normalizeOptimizationMode(optimizationMode)
	profileBySvt := make(map[int]model.ServantOptimizationProfile, len(optimizationProfiles))
	for _, profile := range optimizationProfiles {
		profileBySvt[profile.Id] = profile
	}

	// Bond 15 has two distinct states:
	//   capped: bond is currently at its maximum, earns 0 but provides party +25%;
	//   active: JP 15->16 (or later equivalent), still earns bond and also provides +25%.
	// The existing Box bond15 list represents capped/manual Bond 15 entries. Imported
	// profile data lets us infer active 15->16 providers without another UI list.
	bond15CappedSet := make(map[int]bool, len(bond15Svt))
	for _, id := range bond15Svt {
		bond15CappedSet[id] = true
	}
	bond15ActiveSet := make(map[int]bool)
	bond15ProviderSet := make(map[int]bool)
	for id := range bond15CappedSet {
		bond15ProviderSet[id] = true
	}
	for _, profile := range optimizationProfiles {
		if profile.BondRank >= 15 && profile.BondRankMax > 15 && !bond15CappedSet[profile.Id] {
			bond15ActiveSet[profile.Id] = true
			bond15ProviderSet[profile.Id] = true
		}
	}
	bond10Set := make(map[int]bool, len(bond10Svt))
	for _, id := range bond10Svt {
		if !bond15ProviderSet[id] {
			bond10Set[id] = true
		}
	}
	weightFor := func(id int) float64 {
		profile, ok := profileBySvt[id]
		if !ok {
			return 1.0
		}
		return preferenceWeight(profile, optimizationMode)
	}
	scoreFor := func(id int, actualBond int) int64 {
		return int64(math.Round(float64(actualBond) * weightFor(id) * 1000.0))
	}

	if len(includeSvt) > svtLimit {
		return []model.TeamResponse{}, 0
	}
	maxSelfCeCount := ceLimit
	if grandMode {
		// 冠位枠では通常の自備礼装に加えて「報酬アップ礼装」を1枚追加装備できる。
		// この追加枠はCost 0として扱う。
		maxSelfCeCount++
	}
	if len(includeCe) > maxSelfCeCount {
		return []model.TeamResponse{}, 0
	}
	if len(includeSupportCe) > supportLimit {
		return []model.TeamResponse{}, 0
	}

	// Upstream removed the old Cost-based minimum-CE heuristic because it could prune
	// legal/global-optimal combinations. Only explicit required CEs constrain the minimum.
	mince := len(includeCe)
	if mince < 0 {
		mince = 0
	}
	if mince > ceLimit {
		mince = ceLimit
	}

	// Prepare Support CE Pool
	supportPool := s.GetSupportCombinations(supportLimit, serverType, includeSupportCe, excludeSupportCe)
	if len(supportPool) == 0 {
		return []model.TeamResponse{}, 0
	}

	// Prepare User CE Pool. ceLimit is the number of ordinary self-owned CE slots.
	// In Grand mode one additional reward-up CE is always available on the Grand servant at zero Cost.
	userCePool := [][]model.CraftEssence{}
	for i := mince; i <= ceLimit; i++ {
		totalCeCount := i
		if grandMode {
			totalCeCount++
		}
		combs := s.GetCombination(totalCeCount, includeCe, excludeCe, serverType)
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

	// Some events have servants whose presence grants a party-wide bond bonus.
	// Enumerate only those provider-presence states so the ordinary servant DP never
	// silently selects a provider without also applying its party effect.
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
			copySelected := make(map[int]bool, len(selected))
			for id := range selected {
				copySelected[id] = true
			}
			partyBonusStates = append(partyBonusStates, partyBonusState{selected: copySelected, bonus: bonus})
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
			normalCount     int
			activeProviders int
			cappedProviders int
			dpCost          int
			cost            int
			bond            int
			score           int64
		}
		isBetterCandidate := func(a, b teamCandidate) bool {
			if a.score != b.score {
				return a.score > b.score
			}
			if a.bond != b.bond {
				return a.bond > b.bond
			}
			return a.cost > b.cost
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
		isBetterTeam := func(a, b model.Team) bool {
			if a.OptimizationScore != b.OptimizationScore {
				return a.OptimizationScore > b.OptimizationScore
			}
			if a.TotalBond != b.TotalBond {
				return a.TotalBond > b.TotalBond
			}
			return a.TotalCost > b.TotalCost
		}
		addLocalTeam := func(teams []model.Team, team model.Team) []model.Team {
			if len(teams) < OPTIMIZE_LIMIT {
				return append(teams, team)
			}
			worst := 0
			for i := 1; i < len(teams); i++ {
				if isBetterTeam(teams[worst], teams[i]) {
					worst = i
				}
			}
			if isBetterTeam(team, teams[worst]) {
				teams[worst] = team
			}
			return teams
		}

		// DP dimensions: selected servant count, active Bond-15 provider count, Cost.
		// We enumerate the FINAL total provider count (0..6), so the +25% bucket is
		// fixed during each DP run. This preserves exact additive-percent rounding while
		// still supporting JP 15->16 providers that earn bond themselves.
		maxSvt := svtLimit + 1
		maxCost := costLimit + 1
		maxQ := svtLimit + 1
		const NEG_SCORE int64 = -1 << 62
		dpScore := make([][][]int64, maxSvt)
		nextDPScore := make([][][]int64, maxSvt)
		dpBond := make([][][]int, maxSvt)
		nextDPBond := make([][][]int, maxSvt)
		paths := make([][][]pathSelection, maxSvt)
		nextPaths := make([][][]pathSelection, maxSvt)
		for k := 0; k < maxSvt; k++ {
			dpScore[k] = make([][]int64, maxQ)
			nextDPScore[k] = make([][]int64, maxQ)
			dpBond[k] = make([][]int, maxQ)
			nextDPBond[k] = make([][]int, maxQ)
			paths[k] = make([][]pathSelection, maxQ)
			nextPaths[k] = make([][]pathSelection, maxQ)
			for q := 0; q < maxQ; q++ {
				dpScore[k][q] = make([]int64, maxCost)
				nextDPScore[k][q] = make([]int64, maxCost)
				dpBond[k][q] = make([]int, maxCost)
				nextDPBond[k][q] = make([]int, maxCost)
				paths[k][q] = make([]pathSelection, maxCost)
				nextPaths[k][q] = make([]pathSelection, maxCost)
			}
		}

		optionalBonusesBuf := make([]model.SvtBonus, len(svtPool)*4)
		userEffectTotals := make([]map[string]SimpleEffect, len(svtPool))
		for svtIdx, svt := range svtPool {
			userEffectTotals[svtIdx] = make(map[string]SimpleEffect, len(svt.Diff))
		}

		for batch := range ceJobs {
			localTeams := make([]model.Team, 0, OPTIMIZE_LIMIT)

			for _, job := range batch {
				allSelfCEs := job.UserCEs
				normalCEs := allSelfCEs
				var grandExtraCE *model.CraftEssence

				grandExtraIndex := -1
				if grandMode && len(allSelfCEs) > 0 {
					grandExtraIndex = 0
					for i := 1; i < len(allSelfCEs); i++ {
						if allSelfCEs[i].Cost > allSelfCEs[grandExtraIndex].Cost ||
							(allSelfCEs[i].Cost == allSelfCEs[grandExtraIndex].Cost && allSelfCEs[i].Id < allSelfCEs[grandExtraIndex].Id) {
							grandExtraIndex = i
						}
					}
					extra := allSelfCEs[grandExtraIndex]
					grandExtraCE = &extra
					normalCEs = make([]model.CraftEssence, 0, len(allSelfCEs)-1)
					for i, ce := range allSelfCEs {
						if i != grandExtraIndex {
							normalCEs = append(normalCEs, ce)
						}
					}
				}

				ceCost := 0
				for _, ce := range normalCEs {
					ceCost += ce.Cost
				}
				if ceCost > costLimit {
					continue
				}

				userCeDense := make([]int, len(allSelfCEs))
				for k, ce := range allSelfCEs {
					userCeDense[k] = ceIdToDense[ce.Id]
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
						validJob := true

						for svtIdx := 0; svtIdx < len(svtPool); svtIdx++ {
							svt := &svtPool[svtIdx]
							isPartyProvider := enableEventBonus && s.getEventPartyBonus(svt, serverType, selectedEvents) > 0
							if isPartyProvider && !partyState.selected[svt.Id] {
								continue
							}
							isBond15Capped := bond15CappedSet[svt.Id]
							isBond15Active := bond15ActiveSet[svt.Id]
							isBond10 := bond10Set[svt.Id]

							getTotalEffect := func(diffKey string, effSlice []SimpleEffect) (float64, int) {
								total := userEffectTotals[svtIdx][diffKey]
								tPercent := total.Percent
								tDirect := total.Direct
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

							applyEventEffects := func(percent float64) float64 {
								if !enableEventBonus {
									return percent
								}
								percent += float64(s.getEventBonus(svt, serverType, selectedEvents))
								percent += float64(partyState.bonus)
								multiplier := s.getEventMultiplier(svt, serverType, selectedEvents)
								if multiplier > 0 {
									percent += math.Round((multiplier - 1.0) * 100.0)
								}
								return percent
							}

							chooseCheapestDetail := func(preferred string) (string, int, bool) {
								if preferred != "" {
									if detail, ok := svt.Diff[preferred]; ok {
										return preferred, detail.Cost, true
									}
								}
								bestKey := ""
								bestCost := int(^uint(0) >> 1)
								for key, detail := range svt.Diff {
									if detail.Cost < bestCost || (detail.Cost == bestCost && (bestKey == "" || key < bestKey)) {
										bestKey = key
										bestCost = detail.Cost
									}
								}
								return bestKey, bestCost, bestKey != ""
							}

							mandatory := includeSvtSet[svt.Id] || partyState.selected[svt.Id]
							if mandatory {
								preferred := ""
								if includeSvtSet[svt.Id] {
									preferred = "default"
									if k, ok := includeSvtDiffMap[svt.Id]; ok {
										preferred = k
									}
								}
								if isBond15Capped || isBond10 {
									diffKey, detailCost, ok := chooseCheapestDetail(preferred)
									if ok {
										mandatoryBonuses = append(mandatoryBonuses, model.SvtBonus{
											Svt: svt, DiffKey: diffKey, Bonus: 0, Cost: detailCost,
											Bond15: isBond15Capped, BondCapped: true,
										})
									}
									continue
								}

								selectBest := func() model.SvtBonus {
									best := model.SvtBonus{Svt: svt, DiffKey: "default", Bonus: -1, Cost: svt.Diff["default"].Cost, Bond15: isBond15Active}
									for key, detail := range svt.Diff {
										if preferred != "" && key != preferred {
											continue
										}
										effSlice := svtDiffEffects[svtIdx][key]
										percent, direct := getTotalEffect(key, effSlice)
										percent = applyEventEffects(percent)
										b := baseBond + int(float64(baseBond)*percent/100.0) + direct
										if b > best.Bonus || (b == best.Bonus && detail.Cost < best.Cost) {
											best = model.SvtBonus{Svt: svt, DiffKey: key, Bonus: b, Cost: detail.Cost, Percent: percent, Direct: direct, Bond15: isBond15Active}
										}
									}
									return best
								}
								mandatoryBonuses = append(mandatoryBonuses, selectBest())
							} else {
								if isBond15Capped {
									diffKey, detailCost, ok := chooseCheapestDetail("")
									if ok {
										optionalBonuses = append(optionalBonuses, model.SvtBonus{Svt: svt, DiffKey: diffKey, Bonus: 0, Cost: detailCost, Bond15: true, BondCapped: true})
									}
									continue
								}
								if isBond10 {
									continue
								}
								best := model.SvtBonus{Svt: svt, DiffKey: "default", Bonus: -1, Cost: svt.Diff["default"].Cost, Bond15: isBond15Active}
								for key, detail := range svt.Diff {
									effSlice := svtDiffEffects[svtIdx][key]
									percent, direct := getTotalEffect(key, effSlice)
									percent = applyEventEffects(percent)
									b := baseBond + int(float64(baseBond)*percent/100.0) + direct
									if b > best.Bonus || (b == best.Bonus && detail.Cost < best.Cost) {
										best = model.SvtBonus{Svt: svt, DiffKey: key, Bonus: b, Cost: detail.Cost, Percent: percent, Direct: direct, Bond15: isBond15Active}
									}
								}
								optionalBonuses = append(optionalBonuses, best)
							}
						}

						mandatoryCost := 0
						mandatoryCappedProviders := 0
						mandatoryActiveProviders := 0
						for _, mb := range mandatoryBonuses {
							mandatoryCost += mb.Cost
							if mb.Bond15 {
								if mb.BondCapped {
									mandatoryCappedProviders++
								} else {
									mandatoryActiveProviders++
								}
							}
						}
						currentCostLimit := costLimit - ceCost - mandatoryCost
						currentSvtLimit := svtLimit - len(mandatoryBonuses)
						if currentCostLimit < 0 || currentSvtLimit < 0 {
							validJob = false
						}
						if !validJob {
							continue
						}

						optionalNormal := make([]model.SvtBonus, 0, len(optionalBonuses))
						optionalCapped := make([]model.SvtBonus, 0, len(optionalBonuses))
						activeOptionalCount := 0
						for _, sb := range optionalBonuses {
							if sb.Bond15 && sb.BondCapped {
								optionalCapped = append(optionalCapped, sb)
							} else {
								optionalNormal = append(optionalNormal, sb)
								if sb.Bond15 {
									activeOptionalCount++
								}
							}
						}
						sort.Slice(optionalCapped, func(i, j int) bool {
							if optionalCapped[i].Cost != optionalCapped[j].Cost {
								return optionalCapped[i].Cost < optionalCapped[j].Cost
							}
							return optionalCapped[i].Svt.Id < optionalCapped[j].Svt.Id
						})
						maxCapped := min(len(optionalCapped), currentSvtLimit)
						cappedPrefixCost := make([]int, maxCapped+1)
						for i := 1; i <= maxCapped; i++ {
							cappedPrefixCost[i] = cappedPrefixCost[i-1] + optionalCapped[i-1].Cost
						}

						mandatoryProviders := mandatoryCappedProviders + mandatoryActiveProviders
						maxTotalProviders := min(svtLimit, mandatoryProviders+maxCapped+activeOptionalCount)
						for totalProviders := mandatoryProviders; totalProviders <= maxTotalProviders; totalProviders++ {
							guidancePercent := float64(totalProviders * 25)
							mandatoryAdjustedBond := 0
							mandatoryAdjustedScore := int64(0)
							for _, mb := range mandatoryBonuses {
								if mb.BondCapped {
									continue
								}
								actual := baseBond + int(float64(baseBond)*(mb.Percent+guidancePercent)/100.0) + mb.Direct
								mandatoryAdjustedBond += actual
								mandatoryAdjustedScore += scoreFor(mb.Svt.Id, actual)
							}

							qMax := min(currentSvtLimit, activeOptionalCount)
							for k := 0; k <= currentSvtLimit; k++ {
								for q := 0; q <= qMax; q++ {
									for j := 0; j <= currentCostLimit; j++ {
										dpScore[k][q][j] = NEG_SCORE
										dpBond[k][q][j] = 0
										paths[k][q][j] = pathSelection{}
									}
								}
							}
							dpScore[0][0][0] = 0

							adjustedBonuses := make([]int, len(optionalNormal))
							adjustedScores := make([]int64, len(optionalNormal))
							providerGroups := make([][]int, currentCostLimit+1)
							normalGroups := make([][]int, currentCostLimit+1)
							for itemIdx, item := range optionalNormal {
								if item.Cost > currentCostLimit {
									continue
								}
								actual := baseBond + int(float64(baseBond)*(item.Percent+guidancePercent)/100.0) + item.Direct
								adjustedBonuses[itemIdx] = actual
								adjustedScores[itemIdx] = scoreFor(item.Svt.Id, actual)
								if item.Bond15 {
									providerGroups[item.Cost] = append(providerGroups[item.Cost], itemIdx)
								} else {
									normalGroups[item.Cost] = append(normalGroups[item.Cost], itemIdx)
								}
							}
							betterIdx := func(a, b int) bool {
								if adjustedScores[a] != adjustedScores[b] {
									return adjustedScores[a] > adjustedScores[b]
								}
								if adjustedBonuses[a] != adjustedBonuses[b] {
									return adjustedBonuses[a] > adjustedBonuses[b]
								}
								return optionalNormal[a].Svt.Id < optionalNormal[b].Svt.Id
							}
							for cost := 0; cost <= currentCostLimit; cost++ {
								sort.Slice(providerGroups[cost], func(i, j int) bool { return betterIdx(providerGroups[cost][i], providerGroups[cost][j]) })
								sort.Slice(normalGroups[cost], func(i, j int) bool { return betterIdx(normalGroups[cost][i], normalGroups[cost][j]) })
								if len(providerGroups[cost]) > currentSvtLimit {
									providerGroups[cost] = providerGroups[cost][:currentSvtLimit]
								}
								if len(normalGroups[cost]) > currentSvtLimit {
									normalGroups[cost] = normalGroups[cost][:currentSvtLimit]
								}
							}

							for cost := 0; cost <= currentCostLimit; cost++ {
								pg := providerGroups[cost]
								ng := normalGroups[cost]
								if len(pg) == 0 && len(ng) == 0 {
									continue
								}
								pScore := make([]int64, len(pg)+1)
								pBond := make([]int, len(pg)+1)
								nScore := make([]int64, len(ng)+1)
								nBond := make([]int, len(ng)+1)
								for i, idx := range pg {
									pScore[i+1] = pScore[i] + adjustedScores[idx]
									pBond[i+1] = pBond[i] + adjustedBonuses[idx]
								}
								for i, idx := range ng {
									nScore[i+1] = nScore[i] + adjustedScores[idx]
									nBond[i+1] = nBond[i] + adjustedBonuses[idx]
								}

								for k := 0; k <= currentSvtLimit; k++ {
									for q := 0; q <= qMax; q++ {
										for j := 0; j <= currentCostLimit; j++ {
											nextDPScore[k][q][j] = NEG_SCORE
											nextDPBond[k][q][j] = 0
											nextPaths[k][q][j] = pathSelection{}
										}
									}
								}
								for k := 0; k <= currentSvtLimit; k++ {
									for q := 0; q <= qMax; q++ {
										for j := 0; j <= currentCostLimit; j++ {
											if dpScore[k][q][j] == NEG_SCORE {
												continue
											}
											baseScore := dpScore[k][q][j]
											baseBondValue := dpBond[k][q][j]
											baseSelection := paths[k][q][j]
											maxTakeP := min(len(pg), currentSvtLimit-k)
											for takeP := 0; takeP <= maxTakeP; takeP++ {
												if q+takeP > qMax {
													break
												}
												maxTakeN := min(len(ng), currentSvtLimit-k-takeP)
												for takeN := 0; takeN <= maxTakeN; takeN++ {
													take := takeP + takeN
													newCost := j + take*cost
													if newCost > currentCostLimit {
														break
													}
													newK := k + take
													newQ := q + takeP
													newScore := baseScore + pScore[takeP] + nScore[takeN]
													newBond := baseBondValue + pBond[takeP] + nBond[takeN]
													oldScore := nextDPScore[newK][newQ][newCost]
													oldBond := nextDPBond[newK][newQ][newCost]
													if newScore > oldScore || (newScore == oldScore && newBond > oldBond) {
														sel := baseSelection
														pos := k
														for x := 0; x < takeP; x++ {
															sel[pos] = uint16(pg[x] + 1)
															pos++
														}
														for x := 0; x < takeN; x++ {
															sel[pos] = uint16(ng[x] + 1)
															pos++
														}
														nextDPScore[newK][newQ][newCost] = newScore
														nextDPBond[newK][newQ][newCost] = newBond
														nextPaths[newK][newQ][newCost] = sel
													}
												}
											}
										}
									}
								}
								dpScore, nextDPScore = nextDPScore, dpScore
								dpBond, nextDPBond = nextDPBond, dpBond
								paths, nextPaths = nextPaths, paths
							}

							candidates := make([]teamCandidate, 0, OPTIMIZE_LIMIT)
							for k := 0; k <= currentSvtLimit; k++ {
								for q := 0; q <= qMax; q++ {
									p := totalProviders - mandatoryProviders - q
									if p < 0 || p > maxCapped || p+k > currentSvtLimit {
										continue
									}
									providerCost := cappedPrefixCost[p]
									for j := 0; j <= currentCostLimit-providerCost; j++ {
										if dpScore[k][q][j] == NEG_SCORE {
											continue
										}
										totalCount := len(mandatoryBonuses) + p + k
										if totalCount == 0 {
											continue
										}
										candidates = addCandidate(candidates, teamCandidate{
											normalCount: k, activeProviders: q, cappedProviders: p, dpCost: j,
											cost:  ceCost + mandatoryCost + providerCost + j,
											bond:  mandatoryAdjustedBond + dpBond[k][q][j],
											score: mandatoryAdjustedScore + dpScore[k][q][j],
										})
									}
								}
							}

							for _, candidate := range candidates {
								team := model.Team{
									CraftEssences: normalCEs, GrandCraftEssence: grandExtraCE,
									SupportCraftEssences: supportCombo,
									TotalBond:            candidate.bond, OptimizationScore: candidate.score, TotalCost: candidate.cost,
								}
								for _, sb := range mandatoryBonuses {
									team.Servants = append(team.Servants, sb.Svt)
									team.DiffChoice = append(team.DiffChoice, sb.DiffKey)
								}
								for i := 0; i < candidate.cappedProviders; i++ {
									sb := optionalCapped[i]
									team.Servants = append(team.Servants, sb.Svt)
									team.DiffChoice = append(team.DiffChoice, sb.DiffKey)
								}
								selection := paths[candidate.normalCount][candidate.activeProviders][candidate.dpCost]
								for i := 0; i < candidate.normalCount; i++ {
									itemIdx := int(selection[i]) - 1
									if itemIdx < 0 {
										continue
									}
									sb := optionalNormal[itemIdx]
									team.Servants = append(team.Servants, sb.Svt)
									team.DiffChoice = append(team.DiffChoice, sb.DiffKey)
								}
								localTeams = addLocalTeam(localTeams, team)
							}
						}
					}
				}
			}

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
				better := team.OptimizationScore > top.OptimizationScore ||
					(team.OptimizationScore == top.OptimizationScore && team.TotalBond > top.TotalBond) ||
					(team.OptimizationScore == top.OptimizationScore && team.TotalBond == top.TotalBond && team.TotalCost > top.TotalCost)
				if better {
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
		guidanceCount := 0
		partyEventPercent := 0.0
		for k, svt := range team.Servants {
			svtIds[k] = svt.Id
			if bond15ProviderSet[svt.Id] {
				guidanceCount++
			}
			if enableEventBonus {
				partyEventPercent += float64(s.getEventPartyBonus(svt, serverType, selectedEvents))
			}
		}
		guidancePercent := float64(guidanceCount * 25)

		response := model.TeamResponse{
			Servants:              svtIds,
			DiffChoice:            team.DiffChoice,
			ServantBondBonuses:    make([]model.TeamResultServantBond, len(team.Servants)),
			Bond15GuidanceCount:   guidanceCount,
			Bond15GuidancePercent: guidancePercent,
			OptimizationMode:      optimizationMode,
			OptimizationScore:     float64(team.OptimizationScore) / 1000.0,
			TotalCost:             team.TotalCost,
			TotalBond:             team.TotalBond,
			CraftEssences:         make([]model.TeamResultCE, len(team.CraftEssences)),
			SupportCraftEssences:  make([]model.TeamResultCE, len(team.SupportCraftEssences)),
		}

		for k, svt := range team.Servants {
			diffKey := team.DiffChoice[k]
			if bond15CappedSet[svt.Id] {
				response.ServantBondBonuses[k] = model.TeamResultServantBond{
					Id:                   svt.Id,
					BonusPercent:         0,
					DirectBonus:          0,
					TotalBond:            0,
					Bond15GuidanceSource: true,
					BondCapped:           true,
				}
				continue
			}
			if bond10Set[svt.Id] {
				response.ServantBondBonuses[k] = model.TeamResultServantBond{
					Id:         svt.Id,
					TotalBond:  0,
					BondCapped: true,
				}
				continue
			}
			percent := 0.0
			direct := 0

			addCeEffect := func(ce model.CraftEssence, support bool) {
				if support && ce.Id == TEATIME_ID {
					percent += 15.0
					return
				}
				if m1, ok := ceEffects[ce.Id]; ok {
					if m2, ok2 := m1[svt.Id]; ok2 {
						if eff, ok3 := m2[diffKey]; ok3 {
							percent += eff.Percent
							direct += eff.Direct
						}
					}
				}
			}

			for _, ce := range team.CraftEssences {
				addCeEffect(ce, false)
			}
			if team.GrandCraftEssence != nil {
				addCeEffect(*team.GrandCraftEssence, false)
			}
			for _, ce := range team.SupportCraftEssences {
				addCeEffect(ce, true)
			}

			if enableEventBonus {
				percent += float64(s.getEventBonus(svt, serverType, selectedEvents))
				percent += partyEventPercent
				multiplier := s.getEventMultiplier(svt, serverType, selectedEvents)
				if multiplier > 0 {
					percent += math.Round((multiplier - 1.0) * 100.0)
				}
			}
			percent += guidancePercent

			totalBond := baseBond + int(float64(baseBond)*percent/100.0) + direct
			weight := weightFor(svt.Id)
			response.ServantBondBonuses[k] = model.TeamResultServantBond{
				Id:                      svt.Id,
				BonusPercent:            percent,
				DirectBonus:             direct,
				TotalBond:               totalBond,
				Bond15GuidanceSource:    bond15ProviderSet[svt.Id],
				GuidanceReceivedPercent: guidancePercent,
				PreferenceWeight:        weight,
				OptimizationScore:       float64(scoreFor(svt.Id, totalBond)) / 1000.0,
			}
		}

		for j, ce := range team.CraftEssences {
			totalContribution := 0
			for k, svt := range team.Servants {
				if bond15CappedSet[svt.Id] || bond10Set[svt.Id] {
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

		if team.GrandCraftEssence != nil {
			ce := team.GrandCraftEssence
			totalContribution := 0
			for k, svt := range team.Servants {
				if bond15CappedSet[svt.Id] || bond10Set[svt.Id] {
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
			response.GrandCraftEssence = &model.TeamResultCE{
				Id:           ce.Id,
				Contribution: totalContribution,
			}
		}

		// Fill Support CE details in response
		for j, ce := range team.SupportCraftEssences {
			totalContribution := 0
			for k, svt := range team.Servants {
				if bond15CappedSet[svt.Id] || bond10Set[svt.Id] {
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
