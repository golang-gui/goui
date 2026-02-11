package common

type EventQueue interface {
	Destroy()
	Post()
	Poll()
	Wait()
}
