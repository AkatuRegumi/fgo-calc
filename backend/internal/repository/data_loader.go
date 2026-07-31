package repository

import (
	"database/sql"
	"encoding/json"
	"fgo-calc-backend/internal/model"
	"os"
	"path/filepath"
	"sort"
)

type Repository struct {
	servants      []model.Servant
	craftEssences []model.CraftEssence
	traits        map[int]string
	ceEffects     map[int]map[int]map[string]model.CeEffect
	dominateMap   map[int]int

	db      *sql.DB
	dataDir string
}

func NewRepository(dataDir string) (*Repository, error) {
	repo := &Repository{
		dataDir: dataDir,
	}
	if err := repo.loadData(dataDir); err != nil {
		return nil, err
	}
	if err := repo.initUserStore(); err != nil {
		return nil, err
	}
	repo.precompute()
	repo.clearInternalData()
	return repo, nil
}

func (r *Repository) Close() error {
	if r.db == nil {
		return nil
	}
	return r.db.Close()
}

func (r *Repository) clearInternalData() {
	for i := range r.servants {
		for key, detail := range r.servants[i].Diff {
			detail.TraitSet = nil
			r.servants[i].Diff[key] = detail
		}
	}
}

func (r *Repository) loadData(dataDir string) error {
	svtFile, err := os.Open(filepath.Join(dataDir, "servants.json"))
	if err != nil {
		return err
	}
	defer svtFile.Close()
	if err := json.NewDecoder(svtFile).Decode(&r.servants); err != nil {
		return err
	}

	for i := range r.servants {
		for key, detail := range r.servants[i].Diff {
			traitSet := make(map[int]struct{}, len(detail.Traits))
			for _, traitId := range detail.Traits {
				traitSet[traitId] = struct{}{}
			}
			detail.TraitSet = traitSet
			r.servants[i].Diff[key] = detail
		}
	}

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
	r.ceEffects = make(map[int]map[int]map[string]model.CeEffect)
	for _, ce := range r.craftEssences {
		ceMap := make(map[int]map[string]model.CeEffect)
		for _, svt := range r.servants {
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
			r.ceEffects[ce.Id] = ceMap
		}
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

func (r *Repository) GetServants() []model.Servant {
	return r.servants
}

func (r *Repository) GetCraftEssences() []model.CraftEssence {
	return r.craftEssences
}

func (r *Repository) GetTraits() map[int]string {
	return r.traits
}

func (r *Repository) GetCeEffects() map[int]map[int]map[string]model.CeEffect {
	return r.ceEffects
}

func (r *Repository) GetDominateMap() map[int]int {
	return r.dominateMap
}
