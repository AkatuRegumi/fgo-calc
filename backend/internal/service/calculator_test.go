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
