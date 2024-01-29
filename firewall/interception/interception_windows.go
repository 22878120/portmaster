package interception

import (
	"context"
	"fmt"
	"time"

	"github.com/safing/portmaster/firewall/interception/windowskext2"
	"github.com/safing/portmaster/network"
	"github.com/safing/portmaster/network/packet"
	"github.com/safing/portmaster/updates"
)

// start starts the interception.
func startInterception(packets chan packet.Packet) error {
	kextFile, err := updates.GetPlatformFile("kext/portmaster-kext.sys")
	if err != nil {
		return fmt.Errorf("interception: could not get kext sys: %s", err)
	}

	err = windowskext.Init(kextFile.Path())
	if err != nil {
		return fmt.Errorf("interception: could not init windows kext: %s", err)
	}

	err = windowskext.Start()
	if err != nil {
		return fmt.Errorf("interception: could not start windows kext: %s", err)
	}

	// Start packet handler.
	module.StartServiceWorker("kext packet handler", 0, func(ctx context.Context) error {
		windowskext.Handler(ctx, packets, BandwidthUpdates)
		return nil
	})

	// Start bandwidth stats monitor.
	module.StartServiceWorker("kext bandwidth request worker", 0, func(ctx context.Context) error {
		timer := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-timer.C:
				{
					err := windowskext.SendBandwidthStatsRequest()
					if err != nil {
						return err
					}
				}
			case <-ctx.Done():
				{
					return nil
				}
			}

		}
	})

	// Start kext logging. The worker will periodically send request to the kext to send logs.
	module.StartServiceWorker("kext log request worker", 0, func(ctx context.Context) error {
		timer := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-timer.C:
				{
					err := windowskext.SendLogRequest()
					if err != nil {
						return err
					}
				}
			case <-ctx.Done():
				{
					return nil
				}
			}

		}
	})

	return nil
}

// stop starts the interception.
func stopInterception() error {
	return windowskext.Stop()
}

// ResetVerdictOfAllConnections resets all connections so they are forced to go thought the firewall again.
func ResetVerdictOfAllConnections() error {
	return windowskext.ClearCache()
}

// UpdateVerdictOfConnection updates the verdict of the given connection in the kernel extension.
func UpdateVerdictOfConnection(conn *network.Connection) error {
	return windowskext.UpdateVerdict(conn)
}

// GetKextVersion returns the version of the kernel extension.
func GetKextVersion() (string, error) {
	version, err := windowskext.GetVersion()
	if err != nil {
		return "", err
	}

	return version.String(), nil
}
