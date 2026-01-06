package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// detachTargetSession safely detaches the current chromedp session from a tab
// while keeping the actual browser tab alive.
func detachTargetSession(ctx context.Context) {
	if ctx == nil {
		return
	}

	chromedpCtx := chromedp.FromContext(ctx)
	if chromedpCtx == nil || chromedpCtx.Target == nil {
		return
	}
	if chromedpCtx.Target.SessionID != "" {
		detachCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		exec := cdp.WithExecutor(detachCtx, chromedpCtx.Browser)
		if err := target.DetachFromTarget().WithSessionID(chromedpCtx.Target.SessionID).Do(exec); err != nil {
			slog.Debug("detach target", slog.String("target", string(chromedpCtx.Target.TargetID)), slog.Any("err", err))
		}
	}
	chromedpCtx.Target = nil
}

// validateCDPAllocator ensures the allocator context can still open a tab.
func validateCDPAllocator(ctx context.Context) error {
	if ctx == nil {
		return errors.New("nil cdp allocator context")
	}
	testCtx, cancel := chromedp.NewContext(ctx)
	defer cancel()
	timeoutCtx, timeoutCancel := context.WithTimeout(testCtx, 3*time.Second)
	defer timeoutCancel()
	return chromedp.Run(timeoutCtx, chromedp.ActionFunc(func(context.Context) error { return nil }))
}
