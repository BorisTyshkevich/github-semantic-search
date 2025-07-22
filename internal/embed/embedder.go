package embed

type Embedder interface {
	Vector(text string, debug bool) ([]float32, error)
}
