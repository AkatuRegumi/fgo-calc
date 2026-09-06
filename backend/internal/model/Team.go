package model

type SvtBonus struct {
	Svt        *Servant
	DiffKey    string
	Bonus      int
	Cost       int
	Percent    float64
	Direct     int
	Bond15     bool
	BondCapped bool
}

type ServantOptimizationProfile struct {
	Id          int    `json:"id"`
	BondRank    int    `json:"bondRank"`
	BondRankMax int    `json:"bondRankMax"`
	BondTotal   int    `json:"bondTotal"`
	BondNext    int    `json:"bondNext"`
	TargetRank  int    `json:"targetRank"`
	TargetTotal int    `json:"targetTotal"`
	Role        string `json:"role"`
	Priority    string `json:"priority"`
}

type Team struct {
	Servants             []*Servant
	DiffChoice           []string
	CraftEssences        []CraftEssence
	GrandCraftEssence    *CraftEssence
	SupportCraftEssences []CraftEssence
	TotalCost            int
	TotalBond            int
	OptimizationScore    int64
}

type TeamHeap []Team

func (h TeamHeap) Len() int { return len(h) }
func (h TeamHeap) Less(i, j int) bool {
	if h[i].OptimizationScore != h[j].OptimizationScore {
		return h[i].OptimizationScore < h[j].OptimizationScore
	}
	if h[i].TotalBond != h[j].TotalBond {
		return h[i].TotalBond < h[j].TotalBond
	}
	return h[i].TotalCost < h[j].TotalCost
}
func (h TeamHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *TeamHeap) Push(x interface{}) {
	*h = append(*h, x.(Team))
}
func (h *TeamHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type TeamResultCE struct {
	Id           int `json:"id"`
	Contribution int `json:"contribution"`
}

type TeamResultServantBond struct {
	Id                      int     `json:"id"`
	BonusPercent            float64 `json:"bonusPercent"`
	DirectBonus             int     `json:"directBonus"`
	TotalBond               int     `json:"totalBond"`
	Bond15GuidanceSource    bool    `json:"bond15GuidanceSource,omitempty"`
	BondCapped              bool    `json:"bondCapped,omitempty"`
	GuidanceReceivedPercent float64 `json:"guidanceReceivedPercent,omitempty"`
	PreferenceWeight        float64 `json:"preferenceWeight,omitempty"`
	OptimizationScore       float64 `json:"optimizationScore,omitempty"`
}

type TeamResponse struct {
	Servants              []int                   `json:"Servants"`
	DiffChoice            []string                `json:"DiffChoice"`
	ServantBondBonuses    []TeamResultServantBond `json:"ServantBondBonuses,omitempty"`
	CraftEssences         []TeamResultCE          `json:"CraftEssences"`
	GrandCraftEssence     *TeamResultCE           `json:"GrandCraftEssence,omitempty"`
	SupportCraftEssences  []TeamResultCE          `json:"SupportCraftEssences"`
	Bond15GuidanceCount   int                     `json:"Bond15GuidanceCount,omitempty"`
	Bond15GuidancePercent float64                 `json:"Bond15GuidancePercent,omitempty"`
	OptimizationMode      string                  `json:"OptimizationMode,omitempty"`
	OptimizationScore     float64                 `json:"OptimizationScore,omitempty"`
	TotalCost             int                     `json:"TotalCost"`
	TotalBond             int                     `json:"TotalBond"`
}

type CeEffect struct {
	Percent float64
	Direct  int
}
