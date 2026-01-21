package main

import (
	"context"
	"errors"
	"fmt"
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

func ensureBrowser(ctx context.Context) (*chromedp.Context, error) {
	if ctx == nil {
		return nil, errors.New("nil chromedp context")
	}
	chromedpCtx := chromedp.FromContext(ctx)
	if chromedpCtx == nil || chromedpCtx.Allocator == nil {
		return nil, errors.New("invalid chromedp context")
	}
	if chromedpCtx.Browser == nil {
		browser, err := chromedpCtx.Allocator.Allocate(ctx)
		if err != nil {
			return nil, err
		}
		chromedpCtx.Browser = browser
	}
	return chromedpCtx, nil
}

func createBackgroundTarget(base context.Context) (target.ID, error) {
	if base == nil {
		return "", errors.New("nil cdp context")
	}
	ctx, cancel := chromedp.NewContext(base)
	defer cancel()
	chromedpCtx, err := ensureBrowser(ctx)
	if err != nil {
		return "", err
	}
	exec := cdp.WithExecutor(ctx, chromedpCtx.Browser)
	id, err := target.CreateTarget("about:blank").WithBackground(true).Do(exec)
	if err != nil {
		return "", fmt.Errorf("create background target: %w", err)
	}
	return id, nil
}

// validateCDPAllocator ensures the allocator context can still open a tab.
func validateCDPAllocator(ctx context.Context) error {
	if ctx == nil {
		return errors.New("nil cdp allocator context")
	}
	testCtx, cancel := chromedp.NewContext(ctx)
	defer cancel()
	chromedpCtx, err := ensureBrowser(testCtx)
	if err != nil {
		return err
	}
	timeoutCtx, timeoutCancel := context.WithTimeout(testCtx, 3*time.Second)
	defer timeoutCancel()
	exec := cdp.WithExecutor(timeoutCtx, chromedpCtx.Browser)
	_, err = target.GetTargets().Do(exec)
	return err
}
