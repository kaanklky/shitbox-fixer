package main

import (
  "bufio"
  "context"
  "encoding/json"
  "fmt"
  "io"
  "log"
  "os"
  "path/filepath"
  "strings"
  "time"

  "github.com/tuya/tuya-connector-go/connector"
  "github.com/tuya/tuya-connector-go/connector/env"
  "github.com/tuya/tuya-connector-go/connector/logger"
)

var (
  Version   = "dev"
  GitCommit = "unknown"
  BuildDate = "unknown"
)

type Config struct {
  AccessID       string
  AccessKey      string
  Region         string
  DeviceID       string
  ShutdownDelay  time.Duration
  Debug          bool
}

var regionConfig = map[string]struct {
  ApiHost string
  MsgHost string
}{
  "eu": {
    ApiHost: "https://openapi.tuyaeu.com",
    MsgHost: "pulsar+ssl://mqe.tuyaeu.com:7285/",
  },
  "us": {
    ApiHost: "https://openapi.tuyaus.com",
    MsgHost: "pulsar+ssl://mqe.tuyaus.com:7285/",
  },
  "cn": {
    ApiHost: "https://openapi.tuyacn.com",
    MsgHost: "pulsar+ssl://mqe.tuyacn.com:7285/",
  },
  "in": {
    ApiHost: "https://openapi.tuyain.com",
    MsgHost: "pulsar+ssl://mqe.tuyain.com:7285/",
  },
}

type DeviceInfoResponse struct {
  Code    int                    `json:"code"`
  Msg     string                 `json:"msg"`
  Success bool                   `json:"success"`
  Result  map[string]interface{} `json:"result"`
  T       int64                  `json:"t"`
}

type DeviceCmdResponse struct {
  Code    int    `json:"code"`
  Msg     string `json:"msg"`
  Success bool   `json:"success"`
  Result  bool   `json:"result"`
  T       int64  `json:"t"`
}

func loadEnvFile(filepath string) error {
  file, err := os.Open(filepath)
  if err != nil {
    return err
  }
  defer file.Close()

  scanner := bufio.NewScanner(file)
  for scanner.Scan() {
    line := strings.TrimSpace(scanner.Text())
    if line == "" || strings.HasPrefix(line, "#") {
      continue
    }
    parts := strings.SplitN(line, "=", 2)
    if len(parts) == 2 {
      key := strings.TrimSpace(parts[0])
      value := strings.TrimSpace(parts[1])
      os.Setenv(key, value)
    }
  }
  return scanner.Err()
}

func loadConfig() (*Config, error) {
  cfg := &Config{
    AccessID:      os.Getenv("TUYA_ACCESS_ID"),
    AccessKey:     os.Getenv("TUYA_ACCESS_KEY"),
    Region:        os.Getenv("TUYA_REGION"),
    DeviceID:      os.Getenv("TUYA_DEVICE_ID"),
    ShutdownDelay: 0,
    Debug:         os.Getenv("DEBUG") == "true",
  }

  if cfg.AccessID == "" || cfg.AccessKey == "" || cfg.DeviceID == "" {
    return nil, fmt.Errorf("missing required environment variables")
  }

  if cfg.Region == "" {
    cfg.Region = "eu"
  }

  if _, ok := regionConfig[cfg.Region]; !ok {
    return nil, fmt.Errorf("invalid region: %s (valid: eu, us, cn, in)", cfg.Region)
  }

  shutdownDelayStr := os.Getenv("SHUTDOWN_DELAY")
  if shutdownDelayStr != "" {
    duration, err := time.ParseDuration(shutdownDelayStr)
    if err != nil {
      return nil, fmt.Errorf("invalid SHUTDOWN_DELAY: %w", err)
    }
    cfg.ShutdownDelay = duration
  }

  return cfg, nil
}

func getDeviceStatus(deviceID string) (*DeviceInfoResponse, error) {
  resp := &DeviceInfoResponse{}
  err := connector.MakeGetRequest(
    context.Background(),
    connector.WithAPIUri(fmt.Sprintf("/v1.0/devices/%s", deviceID)),
    connector.WithResp(resp),
  )

  if err != nil {
    return nil, fmt.Errorf("failed to get device status: %w", err)
  }

  if !resp.Success {
    return nil, fmt.Errorf("API returned success=false: %s", resp.Msg)
  }

  return resp, nil
}

func getLastDeviceLogs(deviceID string) ([]interface{}, error) {
  now := time.Now().UnixMilli()
  startTime := now - (10 * 60 * 1000)

  dpIds := "1,2,3,4,5,6,7,8,9"

  resp := &DeviceInfoResponse{}
  err := connector.MakeGetRequest(
    context.Background(),
    connector.WithAPIUri(fmt.Sprintf("/v2.0/cloud/thing/%s/logs?query_type=1&type=%s&start_time=%d&end_time=%d", deviceID, dpIds, startTime, now)),
    connector.WithResp(resp),
  )

  if err != nil {
    return nil, fmt.Errorf("failed to get device logs: %w", err)
  }

  if !resp.Success {
    return nil, fmt.Errorf("API returned success=false: %s", resp.Msg)
  }

  if logs, ok := resp.Result["logs"].([]interface{}); ok && len(logs) > 0 {
    limit := 5
    if len(logs) < limit {
      limit = len(logs)
    }
    return logs[:limit], nil
  }

  return nil, fmt.Errorf("no logs found")
}

func needsReset(deviceInfo *DeviceInfoResponse, lastLogs []interface{}) bool {
  online, ok := deviceInfo.Result["online"].(bool)
  if !ok || !online {
    return true
  }

  for _, logEntry := range lastLogs {
    if logMap, ok := logEntry.(map[string]interface{}); ok {
      if value, ok := logMap["value"].(string); ok && value == "Clean_Pause" {
        return true
      }
    }
  }

  return false
}

func controlDevice(deviceID string, debug bool, appLog *log.Logger) error {
  commandsOff := map[string]interface{}{
    "commands": []map[string]interface{}{
      {
        "code":  "switch",
        "value": false,
      },
    },
  }

  payloadOff, _ := json.Marshal(commandsOff)

  respOff := &DeviceCmdResponse{}
  err := connector.MakePostRequest(
    context.Background(),
    connector.WithAPIUri(fmt.Sprintf("/v1.0/devices/%s/commands", deviceID)),
    connector.WithPayload(payloadOff),
    connector.WithResp(respOff),
  )

  if err != nil {
    return fmt.Errorf("failed to send OFF command: %w", err)
  }

  if !respOff.Success {
    return fmt.Errorf("OFF command failed: %s", respOff.Msg)
  }

  if debug {
    appLog.Println("Device turned OFF, waiting 1 second...")
  }
  time.Sleep(1 * time.Second)

  commandsOn := map[string]interface{}{
    "commands": []map[string]interface{}{
      {
        "code":  "switch",
        "value": true,
      },
    },
  }

  payloadOn, _ := json.Marshal(commandsOn)

  respOn := &DeviceCmdResponse{}
  err = connector.MakePostRequest(
    context.Background(),
    connector.WithAPIUri(fmt.Sprintf("/v1.0/devices/%s/commands", deviceID)),
    connector.WithPayload(payloadOn),
    connector.WithResp(respOn),
  )

  if err != nil {
    return fmt.Errorf("failed to send ON command: %w", err)
  }

  if !respOn.Success {
    return fmt.Errorf("ON command failed: %s", respOn.Msg)
  }

  if debug {
    appLog.Println("Device turned ON, waiting 2 seconds...")
  }
  time.Sleep(2 * time.Second)

  commandsClean := map[string]interface{}{
    "commands": []map[string]interface{}{
      {
        "code":  "manual_clean",
        "value": true,
      },
    },
  }

  payloadClean, _ := json.Marshal(commandsClean)

  respClean := &DeviceCmdResponse{}
  err = connector.MakePostRequest(
    context.Background(),
    connector.WithAPIUri(fmt.Sprintf("/v1.0/devices/%s/commands", deviceID)),
    connector.WithPayload(payloadClean),
    connector.WithResp(respClean),
  )

  if err != nil {
    return fmt.Errorf("failed to send CLEAN command: %w", err)
  }

  if !respClean.Success {
    return fmt.Errorf("CLEAN command failed: %s", respClean.Msg)
  }

  if debug {
    appLog.Println("Clean command sent")
  }
  return nil
}

func main() {
  if len(os.Args) > 1 && os.Args[1] == "version" {
    fmt.Printf("Version: %s\nCommit: %s\nBuilt: %s\n", Version, GitCommit, BuildDate)
    os.Exit(0)
  }

  envPath := ".env"
  if _, err := os.Stat(envPath); err == nil {
    if err := loadEnvFile(envPath); err != nil {
      log.Printf("Warning: Failed to load .env file: %v", err)
    }
  } else {
    exePath, err := os.Executable()
    if err == nil {
      exeDir := filepath.Dir(exePath)
      envPath = filepath.Join(exeDir, ".env")
      if _, err := os.Stat(envPath); err == nil {
        if err := loadEnvFile(envPath); err != nil {
          log.Printf("Warning: Failed to load .env file: %v", err)
        }
      }
    }
  }

  cfg, err := loadConfig()
  if err != nil {
    log.Fatalf("Failed to load config: %v", err)
  }

  var appLog *log.Logger
  if !cfg.Debug {
    log.SetOutput(io.Discard)
    logger.Log.SetLevel(999)
    appLog = log.New(os.Stdout, "", 0)
  } else {
    log.SetFlags(0)
    appLog = log.New(os.Stdout, "", 0)
  }

  region := regionConfig[cfg.Region]

  connector.InitWithOptions(
    env.WithApiHost(region.ApiHost),
    env.WithAccessID(cfg.AccessID),
    env.WithAccessKey(cfg.AccessKey),
    env.WithMsgHost(region.MsgHost),
  )

  deviceStatus, err := getDeviceStatus(cfg.DeviceID)
  if err != nil {
    log.Fatalf("Failed to get device status: %v", err)
  }

  if cfg.Debug {
    appLog.Println("========== DEVICE STATUS ==========")
    appLog.Printf("Online: %v\n", deviceStatus.Result["online"])

    if statusArray, ok := deviceStatus.Result["status"].([]interface{}); ok {
      for _, item := range statusArray {
        if statusItem, ok := item.(map[string]interface{}); ok {
          code := statusItem["code"]
          value := statusItem["value"]
          appLog.Printf("  %-25s = %-15v (type: %T)", code, value, value)
        }
      }
    }
    appLog.Println("===================================")
  }

  lastLogs, err := getLastDeviceLogs(cfg.DeviceID)
  if err != nil {
    if cfg.Debug {
      appLog.Printf("\nWarning: Failed to get device logs: %v\n", err)
    }
  }
  if len(lastLogs) > 0 && cfg.Debug {
    appLog.Println("\n========== LAST 5 LOGS ==========")
    amsterdamTZ, _ := time.LoadLocation("Europe/Amsterdam")
    for _, logEntry := range lastLogs {
      if logMap, ok := logEntry.(map[string]interface{}); ok {
        if eventTime, ok := logMap["event_time"].(float64); ok {
          dt := time.Unix(int64(eventTime)/1000, 0).In(amsterdamTZ)
          logMap["event_time_readable"] = dt.Format("2006-01-02 15:04:05")
        }
      }
    }
    logsJSON, _ := json.MarshalIndent(lastLogs, "", "  ")
    appLog.Println(string(logsJSON))
    appLog.Println("=================================")
  }

  if needsReset(deviceStatus, lastLogs) {
    appLog.Println("Device needs reset, sending control command...")
    if err := controlDevice(cfg.DeviceID, cfg.Debug, appLog); err != nil {
      log.Fatalf("Failed to control device: %v", err)
    }
    appLog.Println("Control command sent successfully")
  } else {
    appLog.Println("Device is working properly, no action needed")
  }

  if cfg.ShutdownDelay > 0 {
    if cfg.Debug {
      appLog.Printf("Sleeping for %s before exit...\n", cfg.ShutdownDelay)
    }
    time.Sleep(cfg.ShutdownDelay)
  }
}
