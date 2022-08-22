package wrappers

type DastRiskCount struct {
	HighCount   int32 `json:"high_count"`
	MediumCount int32 `json:"medium_count"`
	LowCount    int32 `json:"low_count"`
	InfoCount   int32 `json:"info_count"`
}

type DastRiskSummary struct {
	SeverityCounter *DastRiskCount `json:"severity_counter"`
	TotalCount      int32          `json:"total"`
}
