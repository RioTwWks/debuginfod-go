package dedup

// compressPool ограничивает число одновременных compressOne (xdelta/dwz/objcopy).
type compressPool struct {
	sem chan struct{}
}

func newCompressPool(workers int) *compressPool {
	if workers <= 0 {
		workers = 8
	}
	return &compressPool{sem: make(chan struct{}, workers)}
}

func (p *compressPool) acquire() {
	if p == nil {
		return
	}
	p.sem <- struct{}{}
}

func (p *compressPool) release() {
	if p == nil {
		return
	}
	<-p.sem
}

func fileWorkersFor(opts Options) int {
	if opts.FileWorkers > 0 {
		return opts.FileWorkers
	}
	if opts.Workers > 0 {
		return opts.Workers * 2
	}
	return 8
}
