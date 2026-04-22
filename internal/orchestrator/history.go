package orchestrator

import "errors"

func (o *Orchestrator) GetExperimentHistory(apiKeyID string, limit int, offset int) ([]map[string]interface{}, error) {
	if o.db == nil {
		return nil, errors.New("storage not configured")
	}
	return o.db.GetExperimentHistoryByAPIKey(apiKeyID, limit, offset)
}

func (o *Orchestrator) GetExperimentHistoryCount(apiKeyID string) (int, error) {
	if o.db == nil {
		return 0, errors.New("storage not configured")
	}
	return o.db.GetExperimentHistoryCountByAPIKey(apiKeyID)
}

func (o *Orchestrator) GetExperimentHistoryDetail(apiKeyID string, experimentID string) (map[string]interface{}, error) {
	if o.db == nil {
		return nil, errors.New("storage not configured")
	}
	return o.db.GetExperimentHistoryDetailByAPIKey(apiKeyID, experimentID)
}
