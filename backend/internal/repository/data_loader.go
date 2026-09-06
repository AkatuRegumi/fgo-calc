package repository

import (
	"encoding/json"
	"fgo-calc-backend/internal/model"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Repository struct {
	dataMu        sync.RWMutex
	servants      []model.Servant
	cnServants    []model.Servant
	cnOverrides   []model.Servant
	cnUnavailable []int
	craftEssences []model.CraftEssence
	traits        map[int]string
	ceEffects     map[string]map[int]map[int]map[string]model.CeEffect
	dominateMap   map[int]int
	dataUpdatedAt int64

	dataDir string
}

func NewRepository(dataDir string) (*Repository, error) {
	repo := &Repository{
		dataDir: dataDir,
	}
	if err := repo.loadData(dataDir); err != nil {
		return nil, err
	}
	repo.precompute()
	repo.clearInternalData()
	return repo, nil
}

func (r *Repository) ReloadData() error {
	fresh := &Repository{dataDir: r.dataDir}
	if err := fresh.loadData(r.dataDir); err != nil {
		return err
	}
	fresh.precompute()
	fresh.clearInternalData()

	r.dataMu.Lock()
	defer r.dataMu.Unlock()
	r.servants = fresh.servants
	r.cnServants = fresh.cnServants
	r.cnOverrides = fresh.cnOverrides
	r.cnUnavailable = fresh.cnUnavailable
	r.craftEssences = fresh.craftEssences
	r.traits = fresh.traits
	r.ceEffects = fresh.ceEffects
	r.dominateMap = fresh.dominateMap
	r.dataUpdatedAt = fresh.dataUpdatedAt
	return nil
}

func (r *Repository) clearInternalData() {
	clear := func(servants []model.Servant) {
		for i := range servants {
			for key, detail := range servants[i].Diff {
				detail.TraitSet = nil
				servants[i].Diff[key] = detail
			}
		}
	}
	clear(r.servants)
	clear(r.cnServants)
	clear(r.cnOverrides)
}

func (r *Repository) loadData(dataDir string) error {
	if updatedAt, err := os.ReadFile(filepath.Join(dataDir, "update.txt")); err == nil {
		r.dataUpdatedAt, _ = strconv.ParseInt(strings.TrimSpace(string(updatedAt)), 10, 64)
	}

	svtFile, err := os.Open(filepath.Join(dataDir, "servants.json"))
	if err != nil {
		return err
	}
	defer svtFile.Close()
	if err := json.NewDecoder(svtFile).Decode(&r.servants); err != nil {
		return err
	}

	initializeTraits := func(servants []model.Servant) {
		for i := range servants {
			for key, detail := range servants[i].Diff {
				traitSet := make(map[int]struct{}, len(detail.Traits))
				for _, traitId := range detail.Traits {
					traitSet[traitId] = struct{}{}
				}
				detail.TraitSet = traitSet
				servants[i].Diff[key] = detail
			}
		}
	}
	initializeTraits(r.servants)

	if cnFile, err := os.Open(filepath.Join(dataDir, "cn.json")); err == nil {
		defer cnFile.Close()
		if err := json.NewDecoder(cnFile).Decode(&r.cnOverrides); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if unavailable, err := os.ReadFile(filepath.Join(dataDir, "cn_unavailable.txt")); err == nil {
		for _, line := range strings.Fields(string(unavailable)) {
			id, err := strconv.Atoi(line)
			if err != nil {
				return err
			}
			r.cnUnavailable = append(r.cnUnavailable, id)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	initializeTraits(r.cnOverrides)
	r.buildCNServants()

	ceFile, err := os.Open(filepath.Join(dataDir, "ces.json"))
	if err != nil {
		return err
	}
	defer ceFile.Close()
	if err := json.NewDecoder(ceFile).Decode(&r.craftEssences); err != nil {
		return err
	}

	traitMapFile, err := os.Open(filepath.Join(dataDir, "names", "traits.json"))
	if err != nil {
		return err
	}
	defer traitMapFile.Close()
	if err := json.NewDecoder(traitMapFile).Decode(&r.traits); err != nil {
		return err
	}

	return nil
}

func (r *Repository) precompute() {
	r.precomputeCeEffects()
	r.buildDominateMap()
}

func (r *Repository) precomputeCeEffects() {
	r.ceEffects = make(map[string]map[int]map[int]map[string]model.CeEffect)
	r.ceEffects["JP"] = buildCeEffects(r.servants, r.craftEssences)
	r.ceEffects["CN"] = buildCeEffects(r.cnServants, r.craftEssences)
}

func buildCeEffects(servants []model.Servant, craftEssences []model.CraftEssence) map[int]map[int]map[string]model.CeEffect {
	result := make(map[int]map[int]map[string]model.CeEffect)
	for _, ce := range craftEssences {
		ceMap := make(map[int]map[string]model.CeEffect)
		for _, svt := range servants {
			svtDiffMap := make(map[string]model.CeEffect)
			for diffKey, detail := range svt.Diff {
				percent := 0.0
				direct := 0
				for _, filter := range ce.Filters {
					match := true
					if len(filter.Traits) > 0 {
						for _, tr := range filter.Traits {
							if _, ok := detail.TraitSet[tr]; !ok {
								match = false
								break
							}
						}
					}
					if match {
						if filter.Effect > 0 {
							percent += filter.Effect
						} else {
							direct += int(-filter.Effect)
						}
						break
					}
				}
				if percent != 0 || direct != 0 {
					svtDiffMap[diffKey] = model.CeEffect{Percent: percent, Direct: direct}
				}
			}
			if len(svtDiffMap) > 0 {
				ceMap[svt.Id] = svtDiffMap
			}
		}
		if len(ceMap) > 0 {
			result[ce.Id] = ceMap
		}
	}
	return result
}

func (r *Repository) buildCNServants() {
	overrides := make(map[int]model.Servant, len(r.cnOverrides))
	for _, servant := range r.cnOverrides {
		overrides[servant.Id] = servant
	}
	unavailable := make(map[int]struct{}, len(r.cnUnavailable))
	for _, id := range r.cnUnavailable {
		unavailable[id] = struct{}{}
	}
	r.cnServants = make([]model.Servant, 0, len(r.servants)-len(unavailable))
	for _, servant := range r.servants {
		if _, excluded := unavailable[servant.Id]; excluded {
			continue
		}
		if override, ok := overrides[servant.Id]; ok {
			servant = override
		}
		r.cnServants = append(r.cnServants, servant)
	}
}

func (r *Repository) buildDominateMap() {
	nonFilterCe := []model.CraftEssence{}
	for _, ce := range r.craftEssences {
		if len(ce.Filters) == 1 && ce.Cost == 12 {
			if len(ce.Filters[0].Traits) == 0 {
				nonFilterCe = append(nonFilterCe, ce)
			}
		}
	}
	sort.Slice(nonFilterCe, func(i, j int) bool {
		if nonFilterCe[i].Filters[0].Effect != nonFilterCe[j].Filters[0].Effect {
			return nonFilterCe[i].Filters[0].Effect > nonFilterCe[j].Filters[0].Effect
		}
		return nonFilterCe[i].Id < nonFilterCe[j].Id
	})
	r.dominateMap = make(map[int]int)
	for i := 0; i < len(nonFilterCe)-1; i++ {
		r.dominateMap[nonFilterCe[i+1].Id] = nonFilterCe[i].Id
	}
}

func (r *Repository) GetServants(server string) []model.Servant {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()
	if server == "CN" {
		return r.cnServants
	}
	return r.servants
}

func (r *Repository) GetCNOverrides() []model.Servant {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()
	return r.cnOverrides
}

func (r *Repository) GetCNUnavailable() []int {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()
	return r.cnUnavailable
}

func (r *Repository) GetCraftEssences() []model.CraftEssence {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()
	return r.craftEssences
}

func (r *Repository) GetTraits() map[int]string {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()
	return r.traits
}

func (r *Repository) GetDataUpdatedAt() int64 {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()
	return r.dataUpdatedAt
}

func (r *Repository) GetCeEffects(server string) map[int]map[int]map[string]model.CeEffect {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()
	if server == "CN" {
		return r.ceEffects["CN"]
	}
	return r.ceEffects["JP"]
}

func (r *Repository) GetDominateMap() map[int]int {
	r.dataMu.RLock()
	defer r.dataMu.RUnlock()
	return r.dominateMap
}
