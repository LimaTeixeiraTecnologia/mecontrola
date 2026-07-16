package workflows

func parseWorkingMemorySections(content string) []goalEditSection {
	return goalEditParseSections(content)
}

func workingMemorySectionBody(content, heading string) string {
	return goalEditSectionBody(content, heading)
}

func replaceWorkingMemorySection(content, heading, newBody string) string {
	return goalEditReplaceSection(content, heading, newBody)
}
