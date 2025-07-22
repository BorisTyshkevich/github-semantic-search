package embed

// Mock is a simple embedder used for tests.
type Mock struct{}

func (Mock) Vector(text string, debug bool) ([]float32, error) {
	// Return small vector for deterministic tests
	return []float32{0.1, 0.2, 0.3, 0.4}, nil
}

var _ Embedder = (*Mock)(nil)
