package daemon

import "context"

type DaemonStatus string

const (
	DaemonStatusRunning DaemonStatus = "running"
	DaemonStatusStopped DaemonStatus = "stopped"
	DaemonStatusUnknown DaemonStatus = "unknown"
)

type DaemonManager interface {
	// StartDaemon starts the daemon with the given name.
	// If the daemon is already running, this is a no-op.
	StartDaemon(name string) error
	// StopDaemon stops the daemon with the given name.
	// If the daemon is not running, this is a no-op.
	StopDaemon(name string) error
	// RestartDaemon restarts the daemon with the given name.
	// If the daemon is not running, it will be started.
	RestartDaemon(ctx context.Context, name string, opts ...OperationOption) error
	// GetDaemonStatus returns the status of the daemon with the given name.
	GetDaemonStatus(name string) (DaemonStatus, error)
	// EnableDaemon enables the daemon with the given name.
	// If the daemon is already enabled, this is a no-op.
	EnableDaemon(name string) error
	// DisableDaemon disables the daemon with the given name.
	// If the daemon is not enabled, this is a no-op.
	DisableDaemon(name string) error
	// DaemonReload will reload all the daemons
	DaemonReload() error
	// Close cleans up any underlying resources used by the daemon manager.
	Close()
}

// OperationOptions allows to customize the behavior of a daemon operation.
type OperationOptions struct {
	Result chan<- OperationResult
	Mode   string
}

// ApplyAll allows OperationOptions to be used as an OperationOption.
func (o *OperationOptions) ApplyAll(in *OperationOptions) {
	if o.Result != nil {
		in.Result = o.Result
	}
	if o.Mode != "" {
		in.Mode = o.Mode
	}
}

// OperationOption is a function that modifies the OperationOptions.
type OperationOption func(*OperationOptions)

// OperationResult represents the result of a daemon operation.
type OperationResult string

const (
	// TODO(gaslor): ideally we should decouple the OperationResult values from the dbus library.
	// For now this is just returning the same strings as the dbus library.
	// However, this breaks the daemon manager abstraction, as it leaks the dbus library
	// implementation details, forcing other possible implementations to mimic the
	// same behavior.

	// Done indicates successful execution of a job.
	Done OperationResult = "done"
	// Canceled indicates that a job has been canceled before it finished execution.
	Canceled OperationResult = "canceled"
	// Timeout indicates that the job timeout was reached.
	Timeout OperationResult = "timeout"
	// Failed indicates that the job failed.
	Failed OperationResult = "failed"
	// Dependency indicates that a job this job has been depending on failed and the job hence has been removed too.
	Dependency OperationResult = "dependency"
	// Skipped indicates that a job was skipped because it didn't apply to the units current state.
	Skipped OperationResult = "skipped"
)
