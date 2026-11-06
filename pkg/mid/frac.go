package mid

// Fraction represents a rational number
type Fraction struct {
	Num, Den int64
}

func NewFraction(n, d int64) Fraction {
	return Fraction{Num: n, Den: d}.Reduce()
}

func (f Fraction) Eval() float64 {
	return float64(f.Num) / float64(f.Den)
}

func (f Fraction) Mul(o Fraction) Fraction {
	return Fraction{Num: f.Num * o.Num, Den: f.Den * o.Den}.Reduce()
}

func (f Fraction) Reduce() Fraction {
	gcd := func(a, b int64) int64 {
		for b != 0 {
			a, b = b, a%b
		}
		return a
	}
	g := gcd(f.Num, f.Den)
	return Fraction{Num: f.Num / g, Den: f.Den / g}
}
