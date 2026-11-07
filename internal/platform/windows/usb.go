//go:build windows

package windows

import (
	"errors"
	"fmt"
	"strings"

	"github.com/MatheusCampagnolo/wintonicz/internal/usb"
	"golang.org/x/sys/windows"
)

func (Collector) ListConnectedDevices() ([]usb.Device, error) {
	deviceSet, err := windows.SetupDiGetClassDevsEx(nil, "", 0, windows.DIGCF_ALLCLASSES|windows.DIGCF_PRESENT, 0, "")
	if err != nil {
		return nil, fmt.Errorf("enumerate present Windows devices: %w", err)
	}
	defer deviceSet.Close()

	devices := make([]usb.Device, 0)
	for index := 0; ; index++ {
		info, err := deviceSet.EnumDeviceInfo(index)
		if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read Windows device %d: %w", index, err)
		}

		enumerator, _ := deviceStringProperty(deviceSet, info, windows.SPDRP_ENUMERATOR_NAME)
		if !strings.EqualFold(enumerator, "USB") {
			continue
		}
		instanceID, err := deviceSet.DeviceInstanceID(info)
		if err != nil {
			return nil, fmt.Errorf("read USB device identifier %d: %w", index, err)
		}
		if strings.HasPrefix(strings.ToUpper(instanceID), `USB\ROOT_HUB`) {
			continue
		}

		name, _ := deviceStringProperty(deviceSet, info, windows.SPDRP_FRIENDLYNAME)
		if name == "" {
			name, _ = deviceStringProperty(deviceSet, info, windows.SPDRP_DEVICEDESC)
		}
		if name == "" {
			name = "Unknown USB device"
		}
		manufacturer, _ := deviceStringProperty(deviceSet, info, windows.SPDRP_MFG)
		class, _ := deviceStringProperty(deviceSet, info, windows.SPDRP_CLASS)
		location, _ := deviceStringProperty(deviceSet, info, windows.SPDRP_LOCATION_INFORMATION)
		devices = append(devices, usb.Device{
			Name:         name,
			Manufacturer: manufacturer,
			Class:        class,
			Location:     location,
		})
	}
	return devices, nil
}

func deviceStringProperty(deviceSet windows.DevInfo, info *windows.DevInfoData, property windows.SPDRP) (string, error) {
	value, err := deviceSet.DeviceRegistryProperty(info, property)
	if err != nil {
		return "", err
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("device property %d is not a string", property)
	}
	return strings.TrimSpace(text), nil
}
