package service

import (
	"os"
	"path/filepath"
	"testing"

	"fgo-calc-backend/internal/repository"
)

func TestGetCombinationAllowsEmptyCombination(t *testing.T) {
	service := &CalculatorService{}

	combinations := service.GetCombination(0, nil, nil, "CN")
	if len(combinations) != 1 || len(combinations[0]) != 0 {
		t.Fatalf("expected one empty combination, got %#v", combinations)
	}
}

func TestGetCombinationRejectsIncludedCEAtZeroLimit(t *testing.T) {
	service := &CalculatorService{}

	combinations := service.GetCombination(0, []int{1}, nil, "CN")
	if len(combinations) != 0 {
		t.Fatalf("expected no combinations, got %#v", combinations)
	}
}

func TestOptimizePrefersLowerCostFormWhenBondIsEqual(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "names"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"servants.json": `[{
			"id": 800100,
			"name": "Mash",
			"diff": {
				"default": {"name":"Default", "traits":[], "cost":0, "img":""},
				"expensive": {"name":"Expensive", "traits":[], "cost":16, "img":""}
			},
			"event_bonuses": {},
			"event_extra_bonuses": {}
		}]`,
		"ces.json":          `[]`,
		"names/traits.json": `{}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dataDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	repo, err := repository.NewRepository(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := NewCalculatorService(repo)

	teams, _ := service.Optimize(16, 1, 0, 0, nil, nil, nil, nil, nil, nil, nil, nil, 100, "CN", false, nil)
	if len(teams) == 0 {
		t.Fatal("expected a team")
	}
	if teams[0].TotalCost != 0 || teams[0].DiffChoice[0] != "default" {
		t.Fatalf("expected the zero-cost form, got cost=%d diff=%q", teams[0].TotalCost, teams[0].DiffChoice[0])
	}
}

func TestFilterServantsUsesRegionalOverridesAndAvailability(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "names"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"servants.json": `[
			{"id":1,"name":"regional","diff":{"default":{"name":"Default","traits":[2780],"cost":1,"img":"jp"}},"event_bonuses":{},"event_extra_bonuses":{}},
			{"id":2,"name":"jp-only","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":"jp"}},"event_bonuses":{},"event_extra_bonuses":{}}
		]`,
		"cn.json": `[
			{"id":1,"name":"regional-cn","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":"cn"}},"event_bonuses":{},"event_extra_bonuses":{}}
		]`,
		"cn_unavailable.txt": "2\n",
		"ces.json":           `[]`,
		"names/traits.json":  `{}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dataDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	repo, err := repository.NewRepository(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := NewCalculatorService(repo)

	jp := service.FilterServants(nil, nil, nil, "JP")
	cn := service.FilterServants(nil, nil, nil, "CN")
	if len(jp) != 2 || len(cn) != 1 || cn[0].Name != "regional-cn" {
		t.Fatalf("unexpected regional pools: JP=%#v CN=%#v", jp, cn)
	}
	if got := service.FilterServants([]int{2780}, nil, nil, "JP"); len(got) != 1 {
		t.Fatalf("expected JP trait match, got %#v", got)
	}
	if got := service.FilterServants([]int{2780}, nil, nil, "CN"); len(got) != 0 {
		t.Fatalf("expected no CN trait match, got %#v", got)
	}
	teams, _ := service.Optimize(1, 1, 0, 0, nil, nil, nil, []int{2}, []string{"default"}, nil, nil, nil, 100, "CN", false, nil)
	if len(teams) != 0 {
		t.Fatalf("expected unavailable mandatory servant to produce no teams, got %#v", teams)
	}
}

func TestOptimizeRequiredServantOverridesExclusionAndCanStandAlone(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "names"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"servants.json": `[
			{"id":1,"name":"required","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_extra_bonuses":{}},
			{"id":2,"name":"unaffordable","diff":{"default":{"name":"Default","traits":[],"cost":16,"img":""}},"event_bonuses":{},"event_extra_bonuses":{}}
		]`,
		"ces.json":          `[]`,
		"names/traits.json": `{}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dataDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	repo, err := repository.NewRepository(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := NewCalculatorService(repo)

	teams, _ := service.Optimize(1, 2, 0, 0, nil, nil, nil, []int{1}, []string{"default"}, []int{1}, nil, nil, 100, "CN", false, nil)
	if len(teams) == 0 {
		t.Fatal("expected the required-only team")
	}
	if len(teams[0].Servants) != 1 || teams[0].Servants[0] != 1 {
		t.Fatalf("expected only required servant 1, got %#v", teams[0].Servants)
	}
}

func TestOptimizeSearchesCECountsBelowCostHeuristic(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "names"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"servants.json": `[{
			"id":1,"name":"servant","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},
			"event_bonuses":{},"event_extra_bonuses":{}
		}]`,
		"ces.json":          `[]`,
		"names/traits.json": `{}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dataDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	repo, err := repository.NewRepository(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := NewCalculatorService(repo)

	teams, _ := service.Optimize(128, 5, 6, 0, nil, nil, nil, nil, nil, nil, nil, nil, 100, "CN", false, nil)
	if len(teams) == 0 {
		t.Fatal("expected a valid team with fewer than the heuristic CE count")
	}
}

func TestOptimizeAppliesPartyEventBonusOnlyWhenProviderIsSelected(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "names"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"servants.json": `[
			{"id":1,"name":"provider","diff":{"default":{"name":"Default","traits":[],"cost":2,"img":""},"cheap":{"name":"Cheap","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{"CN":[{"id":100,"name":"event","bonus":5}]},"event_extra_bonuses":{}},
			{"id":2,"name":"ordinary-a","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{},"event_extra_bonuses":{}},
			{"id":3,"name":"ordinary-b","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{},"event_extra_bonuses":{}}
		]`,
		"ces.json":          `[]`,
		"names/traits.json": `{}`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dataDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	repo, err := repository.NewRepository(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	service := NewCalculatorService(repo)

	withProvider, _ := service.Optimize(2, 2, 0, 0, nil, nil, nil, nil, nil, nil, nil, nil, 100, "CN", true, []int{100})
	if len(withProvider) == 0 || withProvider[0].TotalBond != 210 {
		t.Fatalf("expected selected provider to grant 5%% to both servants, got %#v", withProvider)
	}
	providerSelected := false
	for _, id := range withProvider[0].Servants {
		if id == 1 {
			providerSelected = true
		}
	}
	if !providerSelected {
		t.Fatalf("expected the provider in the optimal team, got %#v", withProvider[0].Servants)
	}
	if withProvider[0].TotalCost != 2 || withProvider[0].DiffChoice[0] != "cheap" {
		t.Fatalf("expected optional provider to use its lower-cost form, got cost=%d diffs=%#v", withProvider[0].TotalCost, withProvider[0].DiffChoice)
	}

	withoutProvider, _ := service.Optimize(2, 2, 0, 0, nil, nil, nil, nil, nil, []int{1}, nil, nil, 100, "CN", true, []int{100})
	if len(withoutProvider) == 0 || withoutProvider[0].TotalBond != 200 {
		t.Fatalf("expected no party bonus without provider, got %#v", withoutProvider)
	}

	disabled, _ := service.Optimize(2, 2, 0, 0, nil, nil, nil, nil, nil, nil, nil, nil, 100, "CN", false, []int{100})
	if len(disabled) == 0 || disabled[0].TotalBond != 200 {
		t.Fatalf("expected disabled event bonuses to be ignored, got %#v", disabled)
	}
}
