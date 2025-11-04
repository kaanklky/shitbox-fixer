#!/usr/bin/env python3

import os
import sys
import time
import json
import logging
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
    return status

  dps = status['dps']
  formatted = {}

  for key, value in dps.items():
    field_name = DPS_MAPPING.get(key, f"dps_{key}")
    formatted_value = format_dps_value(key, value)
    formatted[field_name] = formatted_value

  return {"dps": formatted}

def print_aligned_status(status):
  if not status or 'dps' not in status:
    return

  dps = status['dps']
  max_key_len = max(len(key) for key in dps.keys())

  print("  \"dps\": {")
  items = list(dps.items())
  for i, (key, value) in enumerate(items):
    comma = "," if i < len(items) - 1 else ""
    value_str = json.dumps(value)
    print(f"    \"{key}\"{' ' * (max_key_len - len(key))}: {value_str}{comma}")
  print("  }")

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

def control_device(device, debug=False):
  logger = logging.getLogger(__name__)

  if debug:
    logger.info("Sending OFF command...")
  device.set_value(1, False)

  if debug:
    logger.info("Device turned OFF, waiting 1 second...")
  time.sleep(1)

  if debug:
    logger.info("Sending ON command...")
  device.set_value(1, True)

  if debug:
    logger.info("Device turned ON, waiting 2 seconds...")
  time.sleep(2)

  if debug:
    logger.info("Sending CLEAN command (DPS 9: mop attached)...")
  device.set_value(9, True)

  if debug:
    logger.info("Clean command sent")

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
  debug = os.getenv('DEBUG', 'false').lower() == 'true'
  shutdown_delay = os.getenv('SHUTDOWN_DELAY', '0')

  if not device_ip or not device_id or not local_key:
    print("ERROR: Missing required environment variables: DEVICE_IP, DEVICE_ID, LOCAL_KEY")
    sys.exit(1)

  if debug:
    logging.basicConfig(level=logging.INFO, format='%(message)s')
  else:
    logging.basicConfig(level=logging.WARNING, format='%(message)s')

  logger = logging.getLogger(__name__)

  try:
    version_float = float(protocol_version)
  except ValueError:
    logger.error(f"Invalid PROTOCOL_VERSION: {protocol_version}")
    sys.exit(1)

  device = tinytuya.Device(
    dev_id=device_id,
    address=device_ip,
    local_key=local_key,
    version=version_float
  )

  status = device.status()

  if not status:
    logger.error("Failed to get device status: No response")
    sys.exit(1)

  if 'Error' in status:
    logger.error(f"Failed to get device status: {status['Error']}")
    sys.exit(1)

  if debug:
    logger.info("========== DEVICE STATUS ==========")
    formatted_status = format_status(status)
    print_aligned_status(formatted_status)
    logger.info("===================================")

  if needs_reset(status):
    logger.info("Device needs reset, sending control command...")
    try:
      control_device(device, debug)
      logger.info("Control command sent successfully")
    except Exception as e:
      logger.error(f"Failed to control device: {e}")
      sys.exit(1)
  else:
    logger.info("Device is working properly, no action needed")

  if shutdown_delay and shutdown_delay != '0':
    try:
      delay_seconds = 0
      if shutdown_delay.endswith('m'):
        delay_seconds = int(shutdown_delay[:-1]) * 60
      elif shutdown_delay.endswith('s'):
        delay_seconds = int(shutdown_delay[:-1])
      else:
        delay_seconds = int(shutdown_delay)

      if delay_seconds > 0 and debug:
        logger.info(f"Sleeping for {delay_seconds} seconds before exit...")
        time.sleep(delay_seconds)
    except ValueError:
      pass

if __name__ == '__main__':
  main()
