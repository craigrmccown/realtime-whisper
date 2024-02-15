package predict

type ZeroRng struct{}

func (ZeroRng) Intn(int) int { return 0 }
