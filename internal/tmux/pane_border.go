// Package tmux provides a wrapper around tmux commands.
// pane_border.go implements per-pane border style control for activity indicators.
package tmux

import (
	"context"
	"fmt"
)

// SetPaneBorderStyle sets the border style for a specific pane using
// select-pane -t <target> -P 'pane-border-style=fg=<color>'.
// The color should be a tmux color name or hex value (e.g., "#00ff00").
func (c *Client) SetPaneBorderStyle(target, color string) error {
	return c.SetPaneBorderStyleContext(context.Background(), target, color)
}

// SetPaneBorderStyleContext sets the pane border style with context/cancellation support.
func (c *Client) SetPaneBorderStyleContext(ctx context.Context, target, color string) error {
	style := fmt.Sprintf("fg=%s", color)
	return c.RunSilentContext(ctx, "select-pane", "-t", target, "-P",
		fmt.Sprintf("pane-border-style=%s", style))
}

// SetPaneBorderStyle sets the border style for a pane (default client).
func SetPaneBorderStyle(target, color string) error {
	return DefaultClient.SetPaneBorderStyle(target, color)
}

// SetPaneBorderStyleContext sets the border style with context support (default client).
func SetPaneBorderStyleContext(ctx context.Context, target, color string) error {
	return DefaultClient.SetPaneBorderStyleContext(ctx, target, color)
}

// ResetPaneBorderStyle resets a pane's border style to the default.
func (c *Client) ResetPaneBorderStyle(target string) error {
	return c.ResetPaneBorderStyleContext(context.Background(), target)
}

// ResetPaneBorderStyleContext resets border style with context support.
func (c *Client) ResetPaneBorderStyleContext(ctx context.Context, target string) error {
	return c.RunSilentContext(ctx, "select-pane", "-t", target, "-P",
		"pane-border-style=default")
}

// ResetPaneBorderStyle resets a pane's border style (default client).
func ResetPaneBorderStyle(target string) error {
	return DefaultClient.ResetPaneBorderStyle(target)
}

// ResetPaneBorderStyleContext resets border style with context support (default client).
func ResetPaneBorderStyleContext(ctx context.Context, target string) error {
	return DefaultClient.ResetPaneBorderStyleContext(ctx, target)
}
