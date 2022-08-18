package collector

type Collector interface {
	// Start will start collecting operations.
	Start() error

	// Sync synchronizes the in memory collector to the filesystem.
	Sync() error

	// Stop stops and waits for all collecting operations are finish.
	Stop() error
}
