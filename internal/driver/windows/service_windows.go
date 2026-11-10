// SPDX-License-Identifier: MIT

//go:build windows

package windows

import (
	"context"

	"golang.org/x/sys/windows/svc"
)

const ServiceName = "AnyProxySvc"

type winServiceHandler struct {
	engine *WinDivertEngine
}

func (h *winServiceHandler) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	if err := h.engine.Start(context.Background()); err != nil {
		return false, 1
	}

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			break loop
		}
	}

	changes <- svc.Status{State: svc.StopPending}
	_ = h.engine.Stop()
	return false, 0
}

// IsWindowsService reports whether the current process is running under the Windows Service Control Manager.
func IsWindowsService() bool {
	inService, err := svc.IsWindowsService()
	return err == nil && inService
}

// RunWindowsService runs the WinDivertEngine under the Windows Service Control Manager.
func RunWindowsService(engine *WinDivertEngine) error {
	return svc.Run(ServiceName, &winServiceHandler{engine: engine})
}

