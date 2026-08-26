package scope

type Master struct{}

// GATE: 배선 필수
func (m *Master) ClassifyObserved(id string) bool { return false }
