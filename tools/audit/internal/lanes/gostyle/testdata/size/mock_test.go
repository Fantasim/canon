package size

type mockStore struct{}

func (mockStore) Query(a, b, c, d, e, f int) (int, int, int, error) { return a, b, c, nil }
