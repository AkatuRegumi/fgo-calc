package model

type ServantDetail struct {
	Name     string           `json:"name"`
	Traits   []int            `json:"traits"`
	Cost     int              `json:"cost"`
	Img      string           `json:"img"`
	TraitSet map[int]struct{} `json:"-"`
}

type EventBonus struct {
	Id    int    `json:"id"`
	Name  string `json:"name"`
	Bonus int    `json:"bonus"`
}

type Servant struct {
	Id                int                      `json:"id"`
	Name              string                   `json:"name"`
	Diff              map[string]ServantDetail `json:"diff"`
	EventBonuses      map[string][]EventBonus  `json:"event_bonuses"`
	EventPartyBonuses map[string][]EventBonus  `json:"event_party_bonuses"`
	EventExtraBonuses map[string][]EventBonus  `json:"event_extra_bonuses"`
}
