package model

type SvtBonus struct {
	Svt     *Servant
	DiffKey string
	Bonus   int
	Cost    int
}

type Team struct {
	Servants             []*Servant
	DiffChoice           []string
	CraftEssences        []CraftEssence
	SupportCraftEssences []CraftEssence
	TotalCost            int
	TotalBond            int
}

type TeamHeap []Team

func (h TeamHeap) Len() int { return len(h) }
func (h TeamHeap) Less(i, j int) bool {
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

type TeamResponse struct {
	Servants             []int          `json:"Servants"`
	DiffChoice           []string       `json:"DiffChoice"`
	CraftEssences        []TeamResultCE `json:"CraftEssences"`
	SupportCraftEssences []TeamResultCE `json:"SupportCraftEssences"`
	TotalCost            int            `json:"TotalCost"`
	TotalBond            int            `json:"TotalBond"`
}

type CeEffect struct {
	Percent float64
	Direct  int
}
