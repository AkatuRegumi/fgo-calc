package model

type HistoryEntry struct {
	ID        int64  `json:"id"`
	Timestamp int64  `json:"timestamp"`
	State     string `json:"state"`
	Result    string `json:"result"`
	Name      string `json:"name"`
	Pinned    bool   `json:"pinned"`
}

type User struct {
	Username string         `json:"username"`
	State    string         `json:"state,omitempty"`
	History  []HistoryEntry `json:"history,omitempty"`
}
