package stats

import "math"

// Welford's algorithm with Kahan summation:
// https://en.wikipedia.org/wiki/Algorithms_for_calculating_variance#Welford's_online_algorithm
// https://en.wikipedia.org/wiki/Kahan_summation_algorithm

type welford struct {
	m1, m2 kahan
	n      uint64
}

func (w welford) average() float64 {
	return w.m1.hi
}

func (w welford) var_pop() float64 {
	return w.m2.hi / float64(w.n)
}

func (w welford) var_samp() float64 {
	return w.m2.hi / float64(w.n-1) // Bessel's correction
}

func (w welford) stddev_pop() float64 {
	return math.Sqrt(w.var_pop())
}

func (w welford) stddev_samp() float64 {
	return math.Sqrt(w.var_samp())
}

func (w *welford) enqueue(x float64) {
	w.n++
	d1 := x - w.m1.hi - w.m1.lo
	w.m1.add(d1 / float64(w.n))
	d2 := x - w.m1.hi - w.m1.lo
	w.m2.add(d1 * d2)
}

func (w *welford) dequeue(x float64) {
	w.n--
	d1 := x - w.m1.hi - w.m1.lo
	w.m1.sub(d1 / float64(w.n))
	d2 := x - w.m1.hi - w.m1.lo
	w.m2.sub(d1 * d2)
}

type welford2 struct {
	m1x, m2x kahan
	m1y, m2y kahan
	cov      kahan
	n        uint64
}

func (w welford2) covar_pop() float64 {
	return w.cov.hi / float64(w.n)
}

func (w welford2) covar_samp() float64 {
	return w.cov.hi / float64(w.n-1) // Bessel's correction
}

func (w welford2) correlation() float64 {
	return w.cov.hi / math.Sqrt(w.m2x.hi*w.m2y.hi)
}

func (w *welford2) enqueue(x, y float64) {
	w.n++
	d1x := x - w.m1x.hi - w.m1x.lo
	d1y := y - w.m1y.hi - w.m1y.lo
	w.m1x.add(d1x / float64(w.n))
	w.m1y.add(d1y / float64(w.n))
	d2x := x - w.m1x.hi - w.m1x.lo
	d2y := y - w.m1y.hi - w.m1y.lo
	w.m2x.add(d1x * d2x)
	w.m2y.add(d1y * d2y)
	w.cov.add(d1x * d2y)
}

func (w *welford2) dequeue(x, y float64) {
	w.n--
	d1x := x - w.m1x.hi - w.m1x.lo
	d1y := y - w.m1y.hi - w.m1y.lo
	w.m1x.sub(d1x / float64(w.n))
	w.m1y.sub(d1y / float64(w.n))
	d2x := x - w.m1x.hi - w.m1x.lo
	d2y := y - w.m1y.hi - w.m1y.lo
	w.m2x.sub(d1x * d2x)
	w.m2y.sub(d1y * d2y)
	w.cov.sub(d1x * d2y)
}

type kahan struct{ hi, lo float64 }

func (k *kahan) add(x float64) {
	y := k.lo + x
	t := k.hi + y
	k.lo = y - (t - k.hi)
	k.hi = t
}

func (k *kahan) sub(x float64) {
	y := k.lo - x
	t := k.hi + y
	k.lo = y - (t - k.hi)
	k.hi = t
}
