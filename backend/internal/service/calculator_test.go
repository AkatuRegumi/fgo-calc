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
