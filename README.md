# Shitbox Fixer - Tuya Device Monitor

A Go-based monitoring tool for Tuya smart devices that automatically detects and fixes stuck states. Specifically designed to monitor a cat litter box device that occasionally gets stuck in "Clean_Pause" state because of a defective weight sensor.

## Purpose

This application monitors a Tuya-enabled smart device and automatically resets it when the device enters a "Clean_Pause" state (detected from device logs in the last 10 minutes).

When a reset is needed, it performs a complete reset sequence:
1. Sends OFF command (switch = false)
2. Waits 1 second
3. Sends ON command (switch = true)
4. Waits 2 seconds
5. Sends manual clean command

## Requirements

- Go 1.18+
- Tuya Cloud account and API credentials

## Installation

### 1. Tuya Cloud Setup

1. Go to https://iot.tuya.com
2. Click on Cloud > Development
3. Create a new Cloud Project
4. Click on the project and get Access ID and Access Key from "Authorization Key" section
5. Link your mobile app account using "Link Tuya App Account"
6. Get your device ID from the "Devices" tab

### 2. Project Setup

```bash
git clone <repo_url>
cd shitbox-fixer
cp .env.example .env
```

Edit the `.env` file:
```
TUYA_ACCESS_ID=your_access_id
TUYA_ACCESS_KEY=your_access_key
TUYA_REGION=eu
TUYA_DEVICE_ID=your_device_id
DEBUG=false
```

Available regions:
- `eu` - Europe (default)
- `us` - United States
- `cn` - China
- `in` - India

### 3. Build

```bash
go build -o shitbox-fixer
```

## Usage

### Check Version

```bash
./shitbox-fixer version
```

### One-time Execution

```bash
./shitbox-fixer
```

### Docker

Pull the latest image from GitHub Container Registry:

```bash
docker pull ghcr.io/kaanklky/shitbox-fixer:latest
```

Run with environment variables:

```bash
docker run --rm \
  -e TUYA_ACCESS_ID=your_access_id \
  -e TUYA_ACCESS_KEY=your_access_key \
  -e TUYA_REGION=eu \
  -e TUYA_DEVICE_ID=your_device_id \
  -e DEBUG=false \
  ghcr.io/kaanklky/shitbox-fixer:latest
```

Or use an env file:

```bash
docker run --rm --env-file .env ghcr.io/kaanklky/shitbox-fixer:latest
```

### Scheduled Execution

This application is designed to be run periodically using cron, systemd timers, or any other task scheduler of your choice.

## How It Works

1. Retrieves device status from Tuya API
2. Checks device logs for the last 10 minutes
3. If "Clean_Pause" state is detected in logs:
   - Sends OFF command (switch = false)
   - Waits 1 second
   - Sends ON command (switch = true)
   - Waits 2 seconds
   - Sends manual clean command
4. All operations are logged

## Debug Mode

Set `DEBUG=true` in `.env` to enable verbose logging:
- Device status with all data points
- Last 5 device logs with timestamps
- Detailed command execution steps

Set `DEBUG=false` for production to only show essential info messages.

## Customization

You can customize the `needsReset()` function in `main.go` to match your needs.

Example: Check for specific status conditions
```go
func needsReset(deviceInfo *DeviceInfoResponse, lastLogs []interface{}) bool {
    for _, logEntry := range lastLogs {
        if logMap, ok := logEntry.(map[string]interface{}); ok {
            if value, ok := logMap["value"].(string); ok && value == "Error_State" {
                return true
            }
        }
    }

    return false
}
```

To modify the control commands, edit the `controlDevice()` function.
