package workflows

func CorrelationKey(resourceID, threadID, workflowID string) string {
	return resourceID + ":" + threadID + ":" + workflowID
}
