package output

// AnonymizationReport resume o resultado de um ciclo do job de anonimização diário.
type AnonymizationReport struct {
	Processed int
	Errors    int
}
