package service

import (
	"os"
	"path/filepath"
	"testing"

	"fgo-calc-backend/internal/model"
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

func TestOptimizeReportsPerServantBondBonus(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "names"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"servants.json": `[{
			"id": 1,
			"name": "Test Servant",
			"diff": {"default": {"name":"Default", "traits":[], "cost":1, "img":""}},
			"event_bonuses": {},
			"event_extra_bonuses": {}
		}]`,
		"ces.json":          `[{"id":10,"name":"Bond 20","img":"","cost":0,"server":"","filters":[[[],20]]}]`,
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
	service := NewCalculatorService(repo)

	teams, _ := service.Optimize(1, 1, 1, 0, nil, nil, nil, nil, nil, nil, []int{10}, nil, 100, "CN", false, nil)
	if len(teams) == 0 {
		t.Fatal("expected a team")
	}
	if len(teams[0].ServantBondBonuses) != 1 {
		t.Fatalf("expected one servant bond breakdown, got %#v", teams[0].ServantBondBonuses)
	}
	got := teams[0].ServantBondBonuses[0]
	if got.BonusPercent != 20 || got.DirectBonus != 0 || got.TotalBond != 120 {
		t.Fatalf("unexpected servant bond breakdown: %#v", got)
	}
}

func TestOptimizeBond15GuidanceCanChooseCappedProvider(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "names"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"servants.json": `[
			{"id":1,"name":"Bond15 Provider","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_extra_bonuses":{}},
			{"id":2,"name":"Farmer A","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_extra_bonuses":{}},
			{"id":3,"name":"Farmer B","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_extra_bonuses":{}}
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
	service := NewCalculatorService(repo)

	teams, _ := service.OptimizeAdvanced(3, 3, 0, 0, nil, nil, nil, nil, nil, nil, nil, nil, 100, "CN", false, nil, false, []int{1})
	if len(teams) == 0 {
		t.Fatal("expected a team")
	}
	team := teams[0]
	if team.TotalBond != 250 {
		t.Fatalf("expected provider + two recipients = 250 total bond, got %d", team.TotalBond)
	}
	if team.Bond15GuidanceCount != 1 || team.Bond15GuidancePercent != 25 {
		t.Fatalf("unexpected guidance summary: count=%d percent=%v", team.Bond15GuidanceCount, team.Bond15GuidancePercent)
	}
	if len(team.Servants) != 3 || len(team.ServantBondBonuses) != 3 {
		t.Fatalf("unexpected team shape: %#v", team)
	}
	providerSeen := false
	recipients := 0
	for _, info := range team.ServantBondBonuses {
		if info.Id == 1 {
			providerSeen = true
			if !info.Bond15GuidanceSource || info.TotalBond != 0 {
				t.Fatalf("expected bond15 provider to be capped at zero own bond, got %#v", info)
			}
			continue
		}
		recipients++
		if info.BonusPercent != 25 || info.GuidanceReceivedPercent != 25 || info.TotalBond != 125 {
			t.Fatalf("unexpected guidance recipient breakdown: %#v", info)
		}
	}
	if !providerSeen || recipients != 2 {
		t.Fatalf("expected one provider and two recipients, got %#v", team.ServantBondBonuses)
	}
}

func TestOptimizeBond15GuidanceStacks(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "names"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"servants.json": `[
			{"id":1,"name":"Provider A","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_extra_bonuses":{}},
			{"id":2,"name":"Provider B","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_extra_bonuses":{}},
			{"id":3,"name":"Farmer A","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_extra_bonuses":{}},
			{"id":4,"name":"Farmer B","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_extra_bonuses":{}}
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
	service := NewCalculatorService(repo)

	teams, _ := service.OptimizeAdvanced(4, 4, 0, 0, nil, nil, nil, nil, nil, nil, nil, nil, 100, "CN", false, nil, false, []int{1, 2})
	if len(teams) == 0 {
		t.Fatal("expected a team")
	}
	team := teams[0]
	if team.Bond15GuidanceCount != 2 || team.Bond15GuidancePercent != 50 || team.TotalBond != 300 {
		t.Fatalf("expected two stacked providers and two 150-bond recipients, got %#v", team)
	}
}

func TestOptimizeBalancedPrefersPassengerWithLowerBondProgress(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "names"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"servants.json": `[
			{"id":1,"name":"Driver","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_extra_bonuses":{}},
			{"id":2,"name":"Passenger","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_extra_bonuses":{}}
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
	service := NewCalculatorService(repo)

	profiles := []model.ServantOptimizationProfile{
		{Id: 1, BondRank: 9, BondTotal: 900, TargetRank: 10, TargetTotal: 1000, Role: "driver", Priority: "auto"},
		{Id: 2, BondRank: 3, BondTotal: 200, TargetRank: 10, TargetTotal: 1000, Role: "passenger", Priority: "auto"},
	}
	teams, _ := service.OptimizeAdvancedWithProfile(1, 1, 0, 0, nil, nil, nil, nil, nil, nil, nil, nil, 100, "CN", false, nil, false, nil, nil, "balanced", profiles)
	if len(teams) == 0 {
		t.Fatal("expected a team")
	}
	if len(teams[0].Servants) != 1 || teams[0].Servants[0] != 2 {
		t.Fatalf("expected passenger to win balanced scoring, got %#v", teams[0])
	}
	if teams[0].TotalBond != 100 {
		t.Fatalf("weighted optimization must not change real bond, got %d", teams[0].TotalBond)
	}
	if teams[0].OptimizationScore <= float64(teams[0].TotalBond) {
		t.Fatalf("expected passenger weighted score above raw bond, got %v", teams[0].OptimizationScore)
	}
}

func TestOptimizeBond10MandatoryIsCappedAtZero(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "names"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"servants.json":     `[{"id":1,"name":"Capped Driver","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_extra_bonuses":{}}]`,
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
	service := NewCalculatorService(repo)

	teams, _ := service.OptimizeAdvancedWithProfile(1, 1, 0, 0, nil, nil, nil, []int{1}, []string{"default"}, nil, nil, nil, 100, "CN", false, nil, false, nil, []int{1}, "balanced", nil)
	if len(teams) == 0 {
		t.Fatal("expected mandatory capped driver team")
	}
	if teams[0].TotalBond != 0 || len(teams[0].ServantBondBonuses) != 1 || !teams[0].ServantBondBonuses[0].BondCapped {
		t.Fatalf("expected bond10 mandatory servant to consume slot but gain zero bond, got %#v", teams[0])
	}
}

func TestV9RequiredServantOverridesExclusion(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "names"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"servants.json": `[
			{"id":1,"name":"required","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{},"event_extra_bonuses":{}},
			{"id":2,"name":"other","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{},"event_extra_bonuses":{}}
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
	service := NewCalculatorService(repo)

	teams, _ := service.OptimizeAdvancedWithProfile(1, 1, 0, 0, nil, nil, nil, []int{1}, []string{"default"}, []int{1}, nil, nil, 100, "CN", false, nil, false, nil, nil, "max", nil)
	if len(teams) == 0 || len(teams[0].Servants) != 1 || teams[0].Servants[0] != 1 {
		t.Fatalf("required servant must win over exclusion, got %#v", teams)
	}
}

func TestV9PartyEventBonusOnlyWhenProviderSelected(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "names"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"servants.json": `[
			{"id":1,"name":"provider","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{"CN":[{"id":100,"name":"event","bonus":5}]},"event_extra_bonuses":{}},
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
	service := NewCalculatorService(repo)

	withProvider, _ := service.OptimizeAdvancedWithProfile(2, 2, 0, 0, nil, nil, nil, nil, nil, nil, nil, nil, 100, "CN", true, []int{100}, false, nil, nil, "max", nil)
	if len(withProvider) == 0 || withProvider[0].TotalBond != 210 {
		t.Fatalf("expected selected provider to grant 5%% to the party, got %#v", withProvider)
	}
	providerSeen := false
	for _, id := range withProvider[0].Servants {
		if id == 1 {
			providerSeen = true
		}
	}
	if !providerSeen {
		t.Fatalf("expected party-bonus provider in optimal team, got %#v", withProvider[0].Servants)
	}

	withoutProvider, _ := service.OptimizeAdvancedWithProfile(2, 2, 0, 0, nil, nil, nil, nil, nil, []int{1}, nil, nil, 100, "CN", true, []int{100}, false, nil, nil, "max", nil)
	if len(withoutProvider) == 0 || withoutProvider[0].TotalBond != 200 {
		t.Fatalf("expected no party bonus after provider exclusion, got %#v", withoutProvider)
	}
}

func TestV9Bond15To16ProviderEarnsAndBuffsParty(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "names"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"servants.json": `[
			{"id":1,"name":"a","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{},"event_extra_bonuses":{}},
			{"id":2,"name":"b","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{},"event_extra_bonuses":{}},
			{"id":3,"name":"c","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{},"event_extra_bonuses":{}},
			{"id":4,"name":"d","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{},"event_extra_bonuses":{}},
			{"id":5,"name":"bond15-active","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{},"event_extra_bonuses":{}}
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
	service := NewCalculatorService(repo)

	profiles := []model.ServantOptimizationProfile{{Id: 5, BondRank: 15, BondRankMax: 16, BondTotal: 0, TargetRank: 15, TargetTotal: 1}}
	teams, _ := service.OptimizeAdvancedWithProfile(10, 5, 0, 0, nil, nil, nil, nil, nil, nil, nil, nil, 100, "JP", false, nil, false, nil, nil, "max", profiles)
	if len(teams) == 0 {
		t.Fatal("expected a team")
	}
	team := teams[0]
	if team.TotalBond != 625 || team.Bond15GuidanceCount != 1 || team.Bond15GuidancePercent != 25 {
		t.Fatalf("expected five 125-bond earners with one active guidance provider, got %#v", team)
	}
	activeFound := false
	for _, info := range team.ServantBondBonuses {
		if info.Id == 5 {
			activeFound = true
			if info.BondCapped || !info.Bond15GuidanceSource || info.TotalBond != 125 || info.GuidanceReceivedPercent != 25 {
				t.Fatalf("active Bond15->16 servant must earn and provide guidance, got %#v", info)
			}
		}
	}
	if !activeFound {
		t.Fatalf("active Bond15->16 provider missing: %#v", team.ServantBondBonuses)
	}
}

func TestV9Bond15CappedPartyEventProviderStacksExactly(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "names"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"servants.json": `[
			{"id":1,"name":"a","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{},"event_extra_bonuses":{}},
			{"id":2,"name":"b","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{},"event_extra_bonuses":{}},
			{"id":3,"name":"c","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{},"event_extra_bonuses":{}},
			{"id":4,"name":"d","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{},"event_extra_bonuses":{}},
			{"id":5,"name":"mash","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},"event_bonuses":{},"event_party_bonuses":{"CN":[{"id":100,"name":"event","bonus":5}]},"event_extra_bonuses":{}}
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
	service := NewCalculatorService(repo)

	teams, _ := service.OptimizeAdvancedWithProfile(10, 5, 0, 0, nil, nil, nil, nil, nil, nil, nil, nil, 100, "CN", true, []int{100}, false, []int{5}, nil, "max", nil)
	if len(teams) == 0 {
		t.Fatal("expected a team")
	}
	// Provider earns 0. Four earners get additive 25% guidance + 5% party event = 130 each.
	if teams[0].TotalBond != 520 || teams[0].Bond15GuidanceCount != 1 {
		t.Fatalf("expected exact additive event+guidance total 520, got %#v", teams[0])
	}
}

func TestV9GrandExtraSelfCEIsZeroCostAndStillApplies(t *testing.T) {
	dataDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dataDir, "names"), 0755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"servants.json": `[{
			"id":1,"name":"grand","diff":{"default":{"name":"Default","traits":[],"cost":1,"img":""}},
			"event_bonuses":{},"event_party_bonuses":{},"event_extra_bonuses":{}
		}]`,
		"ces.json": `[
			{"id":10,"name":"A","img":"","cost":12,"server":"","filters":[[[],100]]},
			{"id":11,"name":"B","img":"","cost":12,"server":"","filters":[[[],50]]}
		]`,
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
	service := NewCalculatorService(repo)

	teams, _ := service.OptimizeAdvancedWithProfile(13, 1, 1, 0, nil, nil, nil, nil, nil, nil, nil, nil, 100, "JP", false, nil, true, nil, nil, "max", nil)
	if len(teams) == 0 {
		t.Fatal("expected a Grand team")
	}
	team := teams[0]
	if team.TotalCost != 13 || team.TotalBond != 250 || team.GrandCraftEssence == nil || len(team.CraftEssences) != 1 {
		t.Fatalf("expected 1 normal CE + 1 zero-cost Grand CE and 250 bond, got %#v", team)
	}
}
