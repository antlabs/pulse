package pulse

import (
	"runtime"

	"github.com/antlabs/pulse/core"
)

type MultiEventLoop struct {
	eventLoops []core.PollingApi
}

func NewMultiEventLoop(options ...Options) (e *MultiEventLoop, err error) {
	eventLoops := make([]core.PollingApi, runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		eventLoops[i], err = core.Create()
		if err != nil {
			return nil, err
		}
	}

	return &MultiEventLoop{
		eventLoops: eventLoops,
	}, nil
}
