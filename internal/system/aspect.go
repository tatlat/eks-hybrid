package system

type SystemAspect interface {
	Name() string
	Setup() error
}
