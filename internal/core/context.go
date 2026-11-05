package core

import "context"

// operationContext starts a deadline for one complete operation. Callers must
// cancel on failure or after operations without rows. After handing rows to a
// consumer, the deadline must remain active until it expires or the parent ends.
func (c *Client) operationContext() (context.Context, context.CancelFunc) {
	if c.config.Timeout == 0 {
		return c.config.Context, func() {}
	}
	
	return context.WithTimeout(c.config.Context, c.config.Timeout)
}
