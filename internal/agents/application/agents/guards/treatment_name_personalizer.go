package guards

import (
	"context"
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const (
	treatmentNameMetadataKey = "nome_tratamento"
	workingMemoryMetadataH2  = "## Working Memory Metadata"
	treatmentNameH2          = "## Nome de Tratamento"
)

type treatmentNamePersonalizerGuard struct{}

func NewTreatmentNamePersonalizerGuard() PostGuard {
	return &treatmentNamePersonalizerGuard{}
}

func (g *treatmentNamePersonalizerGuard) Name() string {
	return "treatment_name_personalizer"
}

func (g *treatmentNamePersonalizerGuard) Inspect(_ context.Context, in agent.Request, out agent.Result) GuardDecision {
	content := strings.TrimSpace(out.Content)
	if content == "" {
		return GuardDecision{}
	}
	name := extractTreatmentName(in)
	if name == "" {
		return GuardDecision{}
	}
	if strings.Contains(content, name) {
		return GuardDecision{}
	}
	result := out
	if responseContainsTreatmentName(content, name) {
		result.Content = replaceTreatmentNameCaseInsensitive(content, name)
		return GuardDecision{Handled: true, Result: result}
	}
	result.Content = name + ", " + lowerFirstRune(content)
	return GuardDecision{Handled: true, Result: result}
}

func extractTreatmentName(in agent.Request) string {
	for _, msg := range in.Messages {
		if msg.Role != "system" {
			continue
		}
		if name := extractTreatmentNameFromMetadata(msg.Content); name != "" {
			return name
		}
		if name := extractTreatmentNameFromWorkingMemory(msg.Content); name != "" {
			return name
		}
	}
	return ""
}

func extractTreatmentNameFromMetadata(content string) string {
	section := extractMarkdownSection(content, workingMemoryMetadataH2)
	if section == "" {
		return ""
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(section), &metadata); err != nil {
		return ""
	}
	name, _ := metadata[treatmentNameMetadataKey].(string)
	return normalizeTreatmentName(name)
}

func extractTreatmentNameFromWorkingMemory(content string) string {
	section := extractMarkdownSection(content, treatmentNameH2)
	lines := strings.Split(section, "\n")
	for _, line := range lines {
		if name := normalizeTreatmentName(line); name != "" {
			return name
		}
	}
	return ""
}

func extractMarkdownSection(content string, heading string) string {
	idx := strings.LastIndex(content, heading)
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(content[idx+len(heading):])
	if rest == "" {
		return ""
	}
	if next := strings.Index(rest, "\n## "); next >= 0 {
		rest = rest[:next]
	}
	return strings.TrimSpace(rest)
}

func normalizeTreatmentName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 40 {
		return ""
	}
	return name
}

func responseContainsTreatmentName(content string, name string) bool {
	return strings.Contains(strings.ToLower(content), strings.ToLower(name))
}

func replaceTreatmentNameCaseInsensitive(content string, name string) string {
	loweredContent := strings.ToLower(content)
	loweredName := strings.ToLower(name)
	if loweredName == "" {
		return content
	}
	var b strings.Builder
	start := 0
	for {
		idx := strings.Index(loweredContent[start:], loweredName)
		if idx < 0 {
			b.WriteString(content[start:])
			return b.String()
		}
		idx += start
		b.WriteString(content[start:idx])
		b.WriteString(name)
		start = idx + len(loweredName)
	}
}

func lowerFirstRune(content string) string {
	first, size := utf8.DecodeRuneInString(content)
	if first == utf8.RuneError && size == 0 {
		return content
	}
	if !unicode.IsUpper(first) {
		return content
	}
	return string(unicode.ToLower(first)) + content[size:]
}
