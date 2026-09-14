package handler

import (
	"fgo-calc-backend/internal/model"
	"testing"
)

func bondInfo(id int, percent float64) model.TeamResultServantBond {
	return model.TeamResultServantBond{Id: id, BonusPercent: percent, PreferenceWeight: 1}
}

func TestBondWithPositionUsesIndependentPositionMultiplier(t *testing.T) {
	info := bondInfo(1, 95)
	if got := bondWithPosition(1000, info, 20); got != 2340 {
		t.Fatalf("front +20%%: got %d, want 2340", got)
	}
	if got := bondWithPosition(1000, info, 24); got != 2418 {
		t.Fatalf("front + support 24%%: got %d, want 2418", got)
	}
	if got := bondWithPosition(1000, info, 4); got != 2028 {
		t.Fatalf("back + support 4%%: got %d, want 2028", got)
	}
}

func TestPositionOptimizerPrefersSupportBackWhenEqual(t *testing.T) {
	team := model.TeamResponse{
		Servants: []int{1, 2, 3, 4, 5},
		ServantBondBonuses: []model.TeamResultServantBond{
			bondInfo(1, 0), bondInfo(2, 0), bondInfo(3, 0), bondInfo(4, 0), bondInfo(5, 0),
		},
		SupportCraftEssences: []model.TeamResultCE{{Id: 1}},
	}
	result := ApplyPositionOptimization([]model.TeamResponse{team}, 1000)[0]
	if result.TotalBond != 5600 {
		t.Fatalf("total bond: got %d, want 5600", result.TotalBond)
	}
	if result.SupportPosition != "back" {
		t.Fatalf("support position: got %q, want back", result.SupportPosition)
	}
	if len(result.FrontlineServants) != 3 {
		t.Fatalf("frontline count: got %d, want 3", len(result.FrontlineServants))
	}
}

func TestPositionOptimizerCanPreferSupportFront(t *testing.T) {
	team := model.TeamResponse{
		Servants: []int{1, 2, 3, 4, 5},
		ServantBondBonuses: []model.TeamResultServantBond{
			bondInfo(1, 200),
			bondInfo(2, 200),
			bondInfo(3, -50),
			bondInfo(4, -50),
			bondInfo(5, -50),
		},
		SupportCraftEssences: []model.TeamResultCE{{Id: 1}},
	}
	result := ApplyPositionOptimization([]model.TeamResponse{team}, 1000)[0]
	if result.SupportPosition != "front" {
		t.Fatalf("support position: got %q, want front", result.SupportPosition)
	}
	if len(result.FrontlineServants) != 2 {
		t.Fatalf("frontline self-servant count: got %d, want 2", len(result.FrontlineServants))
	}
	if result.PositionBondGain <= 0 {
		t.Fatalf("expected positive position gain, got %d", result.PositionBondGain)
	}
}
