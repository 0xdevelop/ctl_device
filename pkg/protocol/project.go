package protocol

// Project represents a registered project in the system.
type Project struct {
	Name           string `json:"name"`
	Dir            string `json:"dir"`
	Tech           string `json:"tech"`
	TestCmd        string `json:"test_cmd"`
	Executor       string `json:"executor"`
	TimeoutMinutes int    `json:"timeout_minutes"`
	NotifyChannel  string `json:"notify_channel"`
	NotifyTarget   string `json:"notify_target,omitempty"`
}
