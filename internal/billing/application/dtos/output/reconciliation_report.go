package output

// ReconciliationReport resume o resultado de um ciclo de reconciliação horária.
type ReconciliationReport struct {
	Inspected int
	Diverged  int
	Synced    int
}
