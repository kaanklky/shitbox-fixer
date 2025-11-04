#!/usr/bin/env python3

import os
import sys
import time
import json
from pathlib import Path

try:
  import tinytuya
except ImportError:
  print("ERROR: tinytuya module not found. Install with: pip3 install tinytuya")
  sys.exit(1)

VERSION = "dev"
GIT_COMMIT = "unknown"
BUILD_DATE = "unknown"

DPS_MAPPING = {
  "1": "reset",
  "2": "mode",
  "3": "unknown_3",
  "4": "unknown_4",
  "5": "cleaning_delay",
  "6": "last_weight",
  "7": "usage_today",
  "8": "last_duration",
  "9": "gravity_sensor_problem",
  "101": "unknown_101",
  "102": "led_color",
  "103": "cleaning_count",
  "105": "unknown_105",
  "106": "dash_brightness",
  "107": "light_brightness",
  "108": "unknown_108",
  "109": "control_source",
  "110": "operation"
}

def format_dps_value(key, value):
  if key == "6" and isinstance(value, (int, float)):
    return f"{value / 10:.1f} kg"
  return value

def format_status(status):
  if not status or 'dps' not in status:
    return {}

  dps = status['dps']
  formatted = {}

  for key, value in dps.items():
    field_name = DPS_MAPPING.get(key, f"dps_{key}")
    formatted_value = format_dps_value(key, value)
    formatted[field_name] = formatted_value

  return formatted

def load_env(env_file=".env"):
  if not os.path.exists(env_file):
    exe_dir = os.path.dirname(os.path.abspath(sys.argv[0]))
    env_file = os.path.join(exe_dir, ".env")
    if not os.path.exists(env_file):
      return

  with open(env_file, 'r') as f:
    for line in f:
      line = line.strip()
      if line and not line.startswith('#'):
        if '=' in line:
          key, value = line.split('=', 1)
          os.environ[key.strip()] = value.strip()

def needs_reset(status):
  if not status or 'dps' not in status:
    return False

  dps = status['dps']
  operation = dps.get('110')
  return operation == 'Clean_Pause'

def control_device(device):
  device.set_value(1, False)
  time.sleep(1)
  device.set_value(1, True)
  time.sleep(2)
  device.set_value(9, True)
  return "Device stuck in 'Clean_Pause' state, sending reset-clean commands consecutively."

def main():
  if len(sys.argv) > 1 and sys.argv[1] == 'version':
    print(f"Version: {VERSION}")
    print(f"Commit: {GIT_COMMIT}")
    print(f"Built: {BUILD_DATE}")
    sys.exit(0)

  load_env()

  device_ip = os.getenv('DEVICE_IP')
  device_id = os.getenv('DEVICE_ID')
  local_key = os.getenv('LOCAL_KEY')
  protocol_version = os.getenv('PROTOCOL_VERSION', '3.3')
  shutdown_delay = os.getenv('SHUTDOWN_DELAY', '0')

  if not device_ip or not device_id or not local_key:
    print(json.dumps({"error": "Missing required environment variables: DEVICE_IP, DEVICE_ID, LOCAL_KEY"}))
    sys.exit(1)

  try:
    version_float = float(protocol_version)
  except ValueError:
    print(json.dumps({"error": f"Invalid PROTOCOL_VERSION: {protocol_version}"}))
    sys.exit(1)

  device = tinytuya.Device(
    dev_id=device_id,
    address=device_ip,
    local_key=local_key,
    version=version_float
  )

  status = device.status()

  if not status:
    print(json.dumps({"error": "Failed to get device status: No response"}))
    sys.exit(1)

  if 'Error' in status:
    print(json.dumps({"error": f"Failed to get device status: {status['Error']}"}))
    sys.exit(1)

  formatted_dps = format_status(status)
  message = ""

  if needs_reset(status):
    try:
      message = control_device(device)
    except Exception as e:
      print(json.dumps({"error": f"Failed to control device: {e}"}))
      sys.exit(1)
  else:
    message = "Device is working properly, no action needed."

  output = {
    "dps": formatted_dps,
    "message": message
  }

  print(json.dumps(output, indent=2))

  if shutdown_delay and shutdown_delay != '0':
    try:
      delay_seconds = 0
      if shutdown_delay.endswith('m'):
        delay_seconds = int(shutdown_delay[:-1]) * 60
      elif shutdown_delay.endswith('s'):
        delay_seconds = int(shutdown_delay[:-1])
      else:
        delay_seconds = int(shutdown_delay)

      if delay_seconds > 0:
        time.sleep(delay_seconds)
    except ValueError:
      pass

if __name__ == '__main__':
  main()
