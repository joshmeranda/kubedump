package collector

type Collector interface {
	// Start will start collecting operations.
	Start() error

	// Stop stops and waits for all collecting operations are finish.
	Stop() error
}
