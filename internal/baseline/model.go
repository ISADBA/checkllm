package baseline

type Baseline struct {
	Provider  string
	Model     string
	APIStyle  string
	UpdatedAt string
	Ranges    map[string]Range
	Notes     []string
}

type Range struct {
	Min *float64
	Max *float64
}
