package flows

import (
	"context"
)

type Upgrader struct {
	*Uninstaller
	*Installer
	*Initer
}

func (u *Upgrader) Run(ctx context.Context) error {
	if err := u.Uninstaller.Run(ctx); err != nil {
		return err
	}

	if err := u.Installer.Run(ctx); err != nil {
		return err
	}

	return u.Initer.Run()
}
