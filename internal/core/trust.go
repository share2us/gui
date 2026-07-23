package core

import "context"

// Device-access trust management: a recipient controls which of their own
// devices a given contact (by email) may target under the 'approvals' inbound
// mode. These wrap the cli-core contacts/senders/devices endpoints.

// ExposedDevices returns the caller's device session ids currently exposed to a
// contact.
func (c *Client) ExposedDevices(ctx context.Context, contactEmail string) ([]string, error) {
	return c.api.ListExposedDevicesForContact(ctx, contactEmail)
}

// ExposeDevice allows a contact to target one of the caller's devices.
func (c *Client) ExposeDevice(ctx context.Context, contactEmail, deviceSessionID string) error {
	return c.api.ExposeDeviceToContact(ctx, contactEmail, deviceSessionID)
}

// UnexposeDevice revokes a contact's access to one of the caller's devices.
func (c *Client) UnexposeDevice(ctx context.Context, contactEmail, deviceSessionID string) error {
	return c.api.UnexposeDeviceFromContact(ctx, contactEmail, deviceSessionID)
}
