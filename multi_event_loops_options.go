package pulse

import "reflect"

type Options struct {
	callback Callback
}

func WithCallback(callback Callback) func(*Options) {
	val := reflect.ValueOf(callback)
	if val.Type().Kind() != reflect.Func {
		return nil
	}
	return func(o *Options) {
		o.callback = callback
	}
}
