// Package platform provides low-level bindings for windows, event loops,
// graphics, typography, and other operating-system services.
//
// Platform objects are thread-affine and use a single-threaded model. The
// caller is responsible for creating and using them on the same OS thread.
// Unless an API explicitly says otherwise, methods are not safe for concurrent
// use. EventLoop.Post and EventLoop.Quit are the only cross-thread operations.
package platform
