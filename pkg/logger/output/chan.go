package output

func NewLimitedChanWriter(limit int) LimitedChanWriter {
	return make(LimitedChanWriter, limit)
}

type LimitedChanWriter chan string

func (lcw LimitedChanWriter) Write(p []byte) (int, error) { // nolint:unparam // err is needed to implement io.Writer
	if len(lcw) == cap(lcw) {
		<-lcw
	}

	lcw <- string(p)

	return len(p), nil
}
