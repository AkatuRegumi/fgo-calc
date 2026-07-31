package model

type HistoryEntry struct {
	Timestamp int64  `json:"timestamp"`
	State     string `json:"state"`
	Result    string `json:"result"`
}

type User struct {
	Username string         `json:"username"`
	State    string         `json:"state,omitempty"`
	History  []HistoryEntry `json:"history,omitempty"`
}
