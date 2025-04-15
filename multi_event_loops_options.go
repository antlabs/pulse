package pulse

import "reflect"

type Options[T any] struct {
	callback Callback[T]
	decoder  Decoder[T]
}

func WithCallback[T any](callback Callback[T]) func(*Options[T]) {
	val := reflect.ValueOf(callback)
	if val.Type().Kind() != reflect.Func {
		return nil
	}

	return func(o *Options[T]) {
		o.callback = callback
	}
}

func WithDecoder[T any](decoder Decoder[T]) func(*Options[T]) {

	return func(o *Options[T]) {
		o.decoder = decoder
	}
}
