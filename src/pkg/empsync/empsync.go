// Package empsync reads the enrolled-user table (the names) from a device over
// the pull protocol and stores it in the local employee registry.
//
// It is deliberately separate from pkg/collector: the collector owns scheduling,
// while this owns one device's sync. That lets the management API run the same
// sync on demand ("sync now") without pulling the poll loop into a controller,
// and keeps both paths on identical upsert semantics.
package empsync

import (
	"fmt"
	"time"

	employeedb "ismila/erp/zkbio/database/employee_db"
	devicestructs "ismila/erp/zkbio/mila_structs/device_structs"
	employeestructs "ismila/erp/zkbio/mila_structs/employee_structs"
	"ismila/erp/zkbio/pkg/zk"
)

// SyncDevice connects to one device, reads its user table and upserts every
// user into the registry. Names arriving here are device-sourced, so an
// operator's imported or hand-edited name is preserved (see
// employeedb.UpsertFromDevice).
//
// Requires a pull-reachable device: push-only units have no IP to dial, and send
// their user table to /iclock/cdata instead.
func SyncDevice(dev *devicestructs.ZKDevice, connTimeout time.Duration) (employeestructs.SyncResult, error) {
	res := employeestructs.SyncResult{DeviceID: dev.ID}

	if dev.IPAddress == "" {
		return res, fmt.Errorf("device has no IP address (push-only devices send their user table to /iclock/cdata instead)")
	}
	port := dev.Port
	if port == 0 {
		port = 4370
	}

	client, err := zk.Dial(dev.IPAddress, port, dev.CommPassword, connTimeout)
	if err != nil {
		return res, err
	}
	defer client.Close()

	if err := client.Connect(); err != nil {
		return res, err
	}
	defer client.Disconnect()

	users, err := client.ReadUsers()
	if err != nil {
		return res, err
	}
	res.Read = len(users)

	for _, u := range users {
		if u.UserID == "" {
			res.Skipped++
			continue
		}
		du := &employeestructs.DeviceUser{
			EmployeeNumber: u.UserID,
			Name:           u.Name,
			CardNumber:     u.CardNumber,
			Privilege:      u.Privilege,
			DeviceUserID:   fmt.Sprintf("%d", u.UID),
		}
		switch outcome, resp := employeedb.UpsertFromDevice(du, dev.ID, employeestructs.SourcePull); outcome {
		case employeedb.Created:
			res.Created++
		case employeedb.Updated:
			res.Updated++
		default:
			res.Skipped++
			if resp != nil {
				err = fmt.Errorf("store user %s: %s", u.UserID, resp.Message)
			}
		}
	}
	return res, err
}
