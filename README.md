# Shitbox Fixer - Tuya Device Monitor

A Python-based monitoring tool for Tuya smart devices that automatically detects and fixes stuck states using local API. Specifically designed to monitor a cat litter box device that occasionally gets stuck in "Clean_Pause" state because of a defective gravity sensor.

## Purpose

This application monitors a Tuya-enabled smart device locally (without cloud) and automatically resets it when the device enters a "Clean_Pause" state.

When a reset is needed, it performs a complete reset sequence:
1. Sends OFF command (switch = false)
2. Waits 1 second
3. Sends ON command (switch = true)
4. Waits 2 seconds
5. Sends manual clean command

## Requirements

- Python 3.7+
- Device on the same local network
- Device local key (obtained from Tuya Cloud)

## Installation

### 1. Get Local Key

You need the device local key to communicate with the device locally. You can obtain it from Tuya IoT Platform:

1. Go to https://iot.tuya.com
2. Create a Cloud Project (you only need this once to get the key)
3. Link your Tuya app account
4. Go to Devices tab and find your device
5. Copy the Device ID and Local Key

### 2. Find Device IP Address

Find your device's local IP address from your router's admin panel or use:

```bash
nmap -p 6668 192.168.1.0/24
```

### 3. Project Setup

```bash
git clone git@github.com:kaanklky/shitbox-fixer.git
cd shitbox-fixer
git checkout feature/local-api
pip3 install -r requirements.txt
cp .env.example .env
```

Edit the `.env` file:
```
DEVICE_IP=192.168.2.221
DEVICE_ID=your_device_id_here
LOCAL_KEY=your_local_key_here
PROTOCOL_VERSION=3.4
DEBUG=false
SHUTDOWN_DELAY=0
```

Environment variables:
- `DEVICE_IP` - Device local IP address (required)
- `DEVICE_ID` - Your device ID from Tuya platform (required)
- `LOCAL_KEY` - Device local key from Tuya platform (required)
- `PROTOCOL_VERSION` - Tuya protocol version (default: `3.4`)
- `DEBUG` - Enable verbose logging (default: `false`)
- `SHUTDOWN_DELAY` - Sleep before exit for scheduled loops (default: `0`, e.g., `1m`, `30s`)

## Usage

### Check Version

```bash
python3 main.py version
```

### One-time Execution

```bash
python3 main.py
```

### Building Standalone Binary

Use PyInstaller to create a standalone executable:

```bash
pip3 install pyinstaller
pyinstaller --onefile --name shitbox-fixer main.py
```

The binary will be in `dist/shitbox-fixer` and can be deployed to any system without Python installed.

### Scheduled Execution

This application is designed to be run periodically using cron, systemd timers, or any other task scheduler of your choice.

## How It Works

1. Connects to device via local network (TCP port 6668)
2. Retrieves device status using Tuya local protocol
3. Checks if device is in "Clean_Pause" state
4. If detected, sends reset sequence:
   - Sends OFF command (switch = false)
   - Waits 1 second
   - Sends ON command (switch = true)
   - Waits 2 seconds
   - Sends manual clean command (triggers gravity sensor)
5. All operations are logged

## Debug Mode

Set `DEBUG=true` in `.env` to enable verbose logging:
- Device status with all data points
- Command execution details
- Protocol-level debugging

Set `DEBUG=false` for production to only show essential info messages.

## Customization

You can customize the `needs_reset()` function in `main.py` to match your needs.

Example: Check for specific status conditions
```python
def needs_reset(status):
  if not status or 'dps' not in status:
    return False

  dps = status['dps']
  operation = dps.get('110')
  return operation == 'Error_State'
```

To modify the control commands, edit the `control_device()` function.

## Technical Details

This application uses the tinytuya library which implements Tuya local protocol:
- Protocol: Tuya Protocol 3.4
- Encryption: AES-128-ECB with MD5-hashed local key
- Transport: TCP on port 6668
- No cloud dependencies, no API limits, completely free to use
