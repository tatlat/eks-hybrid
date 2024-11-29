package cmd_test

import (
	"context"
	"os/exec"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/aws/eks-hybrid/internal/util/cmd"
)

type testCmd struct {
	sync.RWMutex
	path string
	args []string
}

func (f *testCmd) build(ctx context.Context) *exec.Cmd {
	f.RLock()
	defer f.RUnlock()
	return exec.CommandContext(ctx, f.path, f.args...)
}

func (f *testCmd) set(path string, args ...string) {
	f.Lock()
	defer f.Unlock()
	f.path = path
	f.args = args
}

func TestRetrySuccessAfterFailure(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	cmdBuilder := &testCmd{}
	cmdBuilder.set("fake-command")

	go func() {
		time.Sleep(60 * time.Millisecond)
		cmdBuilder.set("echo", "hello")
	}()

	g.Expect(cmd.Retry(ctx, cmdBuilder.build, 1*time.Millisecond)).To(Succeed())
}

func TestRetryTimeout(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	cmdBuilder := &testCmd{}
	cmdBuilder.set("fake-command")

	g.Expect(
		cmd.Retry(ctx, cmdBuilder.build, 1*time.Millisecond),
	).To(MatchError(ContainSubstring(`running command [fake-command]:  [Err exec: "fake-command": executable file not found in $PATH]`)))
}
