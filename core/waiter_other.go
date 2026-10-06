//go:build !linux

package core

// 非 Linux 平台没有 runtime poller 的这层用法, 保持阻塞式等待。
type epollWaiter struct{}

func newEpollWaiter(int) *epollWaiter { return nil }

func (w *epollWaiter) close() {}
