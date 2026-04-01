package models

type FrontendMetrics struct {
	ExperimentID string `json:"experiment_id"`
	Phase        string `json:"phase"`
	Page         string `json:"page"`

	Metrics struct {
		LCP float64 `json:"lcp"`
		CLS float64 `json:"cls"`
		INP float64 `json:"inp"`

		LongTasks           int `json:"long_tasks"`
		Errors              int `json:"errors"`
		UnhandledRejections int `json:"unhandled_rejections"`
	} `json:"metrics"`

	APICalls []struct {
		URL      string  `json:"url"`
		Duration float64 `json:"duration"`
		Status   int     `json:"status"`
	} `json:"api_calls"`

	TimeStamp int64 `json:"timestamp"`
}

// batch support
type FrontendMetricsBatch struct {
	Metrics []FrontendMetrics `json:"metrics"`
}
