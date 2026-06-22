package services

import (
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

type PolicyDecision int

const (
	PolicyDecisionProceed PolicyDecision = iota + 1
	PolicyDecisionClarify
)

type PolicyEvaluator struct {
	minConfidence valueobjects.Confidence
}

func NewPolicyEvaluator(minConfidence valueobjects.Confidence) PolicyEvaluator {
	return PolicyEvaluator{minConfidence: minConfidence}
}

func (p PolicyEvaluator) Evaluate(kind intent.Kind, confidence valueobjects.Confidence) PolicyDecision {
	if !kind.IsWrite() {
		return PolicyDecisionProceed
	}
	if confidence.Below(p.minConfidence) {
		return PolicyDecisionClarify
	}
	return PolicyDecisionProceed
}
