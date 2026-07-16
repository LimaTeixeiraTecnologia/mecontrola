package workflows

func replaceWorkingMemorySection(content, heading, newBody string) string {
	return goalEditReplaceSection(content, heading, newBody)
}
