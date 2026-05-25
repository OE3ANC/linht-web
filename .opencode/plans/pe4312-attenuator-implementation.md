# PE4312 Attenuator Implementation — Full Implementation Plan

## Overview

Move TX/RX switch from GPIO pin 13 → 15, implement PE4312 dual-chip digital step attenuator
driver with web interface. DATA/CLK shared between chips, individual LE pins per chip.

### Pin Reallocation

| GPIO Line | Before | After |
|-----------|--------|-------|
| 15 | unused | **TX/RX switch** |
| 13 | TX/RX switch | **ATT_LE2** (chip 2 latch) |
| 12 | unused | **ATT_LE1** (chip 1 latch) |
| 7 | unused | **ATT_CLK** (shared clock) |
| 6 | unused | **ATT_DATA** (shared data) |

---

## Files Modified (in order)

### 1. config.yaml
**Changes:**
- Line 42: `tx_rx_pin: 13` → `tx_rx_pin: 15`
- After `clock_freq` line, add attenuator section

**Full file:**

```yaml
# Linht Web Manager Configuration

server:
  port: "80"
  host: "0.0.0.0"

# Docker daemon socket
docker:
  socket: "unix:///var/run/docker.sock" # Docker
  #socket: "unix:///run/user/1000/podman/podman.sock" # Podman Service
  container_stop_timeout: 10  # seconds
  default_log_lines: "100"    # default number of log lines to show

# Enabled plugins (Does not change the UI - TODO!)
plugins:
  - docker
  - webshell
  - filemanager
  - hardware
  - cps
  - services

# CPS plugin settings
cps:
  settings_path: "/usr/share/linht/settings.yaml"

# Webshell plugin settings
webshell:
  shell: "/bin/bash"  # Default shell command

# File manager plugin settings
filemanager:
  max_upload_size: 2147483648  # 2GB in bytes (increased for embedded device testing)

# Hardware plugin settings
hardware:
  sx1255:
    spi_device: "/dev/spidev0.0"
    spi_speed: 500000  # 500 kHz
    gpio_chip: "/dev/gpiochip0"
    reset_pin: 22
    tx_rx_pin: 15  # TX/RX switch control
    clock_freq: 32000000  # 32 MHz crystal frequency
  attenuator:
    data_pin: 6   # PE4312 DATA (shared)
    clk_pin: 7    # PE4312 CLK (shared)
    le1_pin: 12   # PE4312 LE1 (chip 1 latch)
    le2_pin: 13   # PE4312 LE2 (chip 2 latch)

# Services plugin settings
services:
  prefix: "linht-"            # Service name prefix filter
  default_log_lines: "100"    # default number of log lines to show
```

### 2. main.go
**Changes:**
- Add `Attenuator` struct inside `Hardware` struct
- Pass attenuator config in `initPlugins()` hardware case

**Replace the Hardware struct block (lines 50-59):**

```go
	Hardware struct {
		SX1255 struct {
			SPIDevice string `yaml:"spi_device"`
			SPISpeed  uint32 `yaml:"spi_speed"`
			GPIOChip  string `yaml:"gpio_chip"`
			ResetPin  int    `yaml:"reset_pin"`
			TxRxPin   int    `yaml:"tx_rx_pin"`
			ClockFreq uint32 `yaml:"clock_freq"`
		} `yaml:"sx1255"`
		Attenuator struct {
			DataPin int `yaml:"data_pin"`
			ClkPin  int `yaml:"clk_pin"`
			LE1Pin  int `yaml:"le1_pin"`
			LE2Pin  int `yaml:"le2_pin"`
		} `yaml:"attenuator"`
	} `yaml:"hardware"`
```

**Replace the hardware case in initPlugins (lines 207-217):**

```go
		case "hardware":
			pluginConfig = map[string]interface{}{
				"sx1255": map[string]interface{}{
					"spi_device": config.Hardware.SX1255.SPIDevice,
					"spi_speed":  config.Hardware.SX1255.SPISpeed,
					"gpio_chip":  config.Hardware.SX1255.GPIOChip,
					"reset_pin":  config.Hardware.SX1255.ResetPin,
					"tx_rx_pin":  config.Hardware.SX1255.TxRxPin,
					"clock_freq": config.Hardware.SX1255.ClockFreq,
				},
				"attenuator": map[string]interface{}{
					"data_pin": config.Hardware.Attenuator.DataPin,
					"clk_pin":  config.Hardware.Attenuator.ClkPin,
					"le1_pin":  config.Hardware.Attenuator.LE1Pin,
					"le2_pin":  config.Hardware.Attenuator.LE2Pin,
				},
			}
```

**Set meaningful defaults (in main.go config struct, add these defaults before loadConfig call, or after loadConfig):**

Actually, defaults should be set if not present in YAML. Add this after `loadConfig("config.yaml")` succeeds (around line 83):

```go
	// Set hardware defaults if not configured
	if config.Hardware.SX1255.TxRxPin == 0 {
		config.Hardware.SX1255.TxRxPin = 15
	}
	if config.Hardware.Attenuator.DataPin == 0 {
		config.Hardware.Attenuator.DataPin = 6
	}
	if config.Hardware.Attenuator.ClkPin == 0 {
		config.Hardware.Attenuator.ClkPin = 7
	}
	if config.Hardware.Attenuator.LE1Pin == 0 {
		config.Hardware.Attenuator.LE1Pin = 12
	}
	if config.Hardware.Attenuator.LE2Pin == 0 {
		config.Hardware.Attenuator.LE2Pin = 13
	}
```

Wait — Go zero values. If the YAML doesn't specify, the int fields will be 0. But 0 is also a valid GPIO pin (though unlikely used). Better approach: check in the hardware plugin factory init, or assign defaults after config parsing explicitly.

Actually simpler: defaults in `plugins/hardware.go` factory init function (which already has default logic for SPI speed, clock freq, etc.)

### 3. NEW FILE: plugins/hardware_pe4312.go

```go
package plugins

import (
	"fmt"
	"time"

	"github.com/warthog618/go-gpiocdev"
)

// PE4312Controller manages GPIO for two PE4312 digital step attenuator chips
// sharing DATA and CLK lines with individual LE (latch enable) pins.
type PE4312Controller struct {
	chip     *gpiocdev.Chip
	dataLine *gpiocdev.Line
	clkLine  *gpiocdev.Line
	le1Line  *gpiocdev.Line
	le2Line  *gpiocdev.Line
	chipPath string
	dataPin  int
	clkPin   int
	le1Pin   int
	le2Pin   int
	// last-set attenuation values (0-63, where val = db * 2)
	lastAtt1 int
	lastAtt2 int
}

// NewPE4312Controller creates a new PE4312 attenuator controller.
// All pins are initialized as outputs, initially LOW.
func NewPE4312Controller(chipPath string, dataPin, clkPin, le1Pin, le2Pin int) (*PE4312Controller, error) {
	chip, err := gpiocdev.NewChip(chipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open GPIO chip %s: %w", chipPath, err)
	}

	ctrl := &PE4312Controller{
		chip:     chip,
		chipPath: chipPath,
		dataPin:  dataPin,
		clkPin:   clkPin,
		le1Pin:   le1Pin,
		le2Pin:   le2Pin,
	}

	// Request DATA line (output, initially low)
	dataLine, err := chip.RequestLine(dataPin, gpiocdev.AsOutput(0), gpiocdev.WithConsumer("pe4312-data"))
	if err != nil {
		chip.Close()
		return nil, fmt.Errorf("failed to request DATA pin %d: %w", dataPin, err)
	}
	ctrl.dataLine = dataLine

	// Request CLK line
	clkLine, err := chip.RequestLine(clkPin, gpiocdev.AsOutput(0), gpiocdev.WithConsumer("pe4312-clk"))
	if err != nil {
		dataLine.Close()
		chip.Close()
		return nil, fmt.Errorf("failed to request CLK pin %d: %w", clkPin, err)
	}
	ctrl.clkLine = clkLine

	// Request LE1 line (chip 1 latch)
	le1Line, err := chip.RequestLine(le1Pin, gpiocdev.AsOutput(0), gpiocdev.WithConsumer("pe4312-le1"))
	if err != nil {
		dataLine.Close()
		clkLine.Close()
		chip.Close()
		return nil, fmt.Errorf("failed to request LE1 pin %d: %w", le1Pin, err)
	}
	ctrl.le1Line = le1Line

	// Request LE2 line (chip 2 latch)
	le2Line, err := chip.RequestLine(le2Pin, gpiocdev.AsOutput(0), gpiocdev.WithConsumer("pe4312-le2"))
	if err != nil {
		dataLine.Close()
		clkLine.Close()
		le1Line.Close()
		chip.Close()
		return nil, fmt.Errorf("failed to request LE2 pin %d: %w", le2Pin, err)
	}
	ctrl.le2Line = le2Line

	return ctrl, nil
}

// Close releases all GPIO resources.
func (p *PE4312Controller) Close() error {
	var errs []error

	if p.le2Line != nil {
		p.le2Line.SetValue(0)
		if err := p.le2Line.Close(); err != nil {
			errs = append(errs, err)
		}
		p.le2Line = nil
	}
	if p.le1Line != nil {
		p.le1Line.SetValue(0)
		if err := p.le1Line.Close(); err != nil {
			errs = append(errs, err)
		}
		p.le1Line = nil
	}
	if p.clkLine != nil {
		p.clkLine.SetValue(0)
		if err := p.clkLine.Close(); err != nil {
			errs = append(errs, err)
		}
		p.clkLine = nil
	}
	if p.dataLine != nil {
		p.dataLine.SetValue(0)
		if err := p.dataLine.Close(); err != nil {
			errs = append(errs, err)
		}
		p.dataLine = nil
	}
	if p.chip != nil {
		if err := p.chip.Close(); err != nil {
			errs = append(errs, err)
		}
		p.chip = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing PE4312 GPIO: %v", errs)
	}
	return nil
}

// tick produces a ~1µs delay for PE4312 serial protocol timing.
func tick() {
	time.Sleep(1 * time.Microsecond)
}

// sendWord clocks out a 6-bit value MSB-first on the shared DATA/CLK lines,
// then strobes the specified LE line to latch the value.
// leLine must be the requested GPIO line pointer for the target chip.
func (p *PE4312Controller) sendWord(bits uint8, leLine *gpiocdev.Line) error {
	// Ensure LE is low during clocking
	if err := leLine.SetValue(0); err != nil {
		return fmt.Errorf("failed to set LE low: %w", err)
	}
	tick()

	// Clock out 6 bits MSB-first (bit5 = 16dB ... bit0 = 0.5dB)
	for i := 5; i >= 0; i-- {
		value := (bits >> i) & 1
		if err := p.dataLine.SetValue(int(value)); err != nil {
			return fmt.Errorf("failed to set DATA: %w", err)
		}
		tick()

		// Rising edge: sample
		if err := p.clkLine.SetValue(1); err != nil {
			return fmt.Errorf("failed to set CLK high: %w", err)
		}
		tick()

		// Falling edge
		if err := p.clkLine.SetValue(0); err != nil {
			return fmt.Errorf("failed to set CLK low: %w", err)
		}
		tick()
	}

	// Latch: strobe LE high → low
	if err := leLine.SetValue(1); err != nil {
		return fmt.Errorf("failed to set LE high: %w", err)
	}
	tick()
	if err := leLine.SetValue(0); err != nil {
		return fmt.Errorf("failed to set LE low: %w", err)
	}
	tick()

	return nil
}

// SetAttenuation sets the attenuation for the specified chip.
// chip: 1 or 2
// db: attenuation in dB (0.0–31.5, 0.5 dB resolution)
func (p *PE4312Controller) SetAttenuation(chip int, db float64) error {
	bits := int(db*2.0 + 0.5) // round to nearest 0.5 dB step
	if bits < 0 {
		bits = 0
	}
	if bits > 63 {
		bits = 63
	}

	var leLine *gpiocdev.Line
	switch chip {
	case 1:
		leLine = p.le1Line
		p.lastAtt1 = bits
	case 2:
		leLine = p.le2Line
		p.lastAtt2 = bits
	default:
		return fmt.Errorf("invalid chip %d (use 1 or 2)", chip)
	}

	return p.sendWord(uint8(bits), leLine)
}

// SetBoth sets both attenuator chips to the same value.
func (p *PE4312Controller) SetBoth(db float64) error {
	if err := p.SetAttenuation(1, db); err != nil {
		return err
	}
	return p.SetAttenuation(2, db)
}

// GetAttenuation returns the last-set attenuation for the specified chip.
// Returns bits value (0-63), actual dB = bits * 0.5.
func (p *PE4312Controller) GetAttenuation(chip int) (int, error) {
	switch chip {
	case 1:
		return p.lastAtt1, nil
	case 2:
		return p.lastAtt2, nil
	default:
		return 0, fmt.Errorf("invalid chip %d (use 1 or 2)", chip)
	}
}

// Info returns configuration information about the PE4312 controller.
func (p *PE4312Controller) Info() map[string]interface{} {
	return map[string]interface{}{
		"chip":      p.chipPath,
		"data_pin":  p.dataPin,
		"clk_pin":   p.clkPin,
		"le1_pin":   p.le1Pin,
		"le2_pin":   p.le2Pin,
		"chip1_db":  float64(p.lastAtt1) * 0.5,
		"chip2_db":  float64(p.lastAtt2) * 0.5,
	}
}
```

### 4. plugins/hardware.go — modifications

#### 4a. Update `HardwareConfig` struct — add `Attenuator` section

Replace the `HardwareConfig` struct (lines 47-57):

```go
// HardwareConfig holds hardware configuration
type HardwareConfig struct {
	SX1255 struct {
		SPIDevice string `yaml:"spi_device"`
		SPISpeed  uint32 `yaml:"spi_speed"`
		GPIOChip  string `yaml:"gpio_chip"`
		ResetPin  int    `yaml:"reset_pin"`
		TxRxPin   int    `yaml:"tx_rx_pin"`
		ClockFreq uint32 `yaml:"clock_freq"`
	} `yaml:"sx1255"`
	Attenuator struct {
		DataPin int
		ClkPin  int
		LE1Pin  int
		LE2Pin  int
	}
}
```

#### 4b. Add attenuator routes in `RegisterRoutes` (after line 125, before line 127):

```go
	// Attenuator control (PE4312)
	api.Post("/attenuator/1", p.handleSetAttenuator1)
	api.Post("/attenuator/2", p.handleSetAttenuator2)
	api.Post("/attenuator/both", p.handleSetAttenuatorBoth)
	api.Get("/attenuator/1", p.handleGetAttenuator1)
	api.Get("/attenuator/2", p.handleGetAttenuator2)
	api.Get("/attenuator/info", p.handleAttenuatorInfo)
```

#### 4c. Add attenuation handler methods (after TX/RX switch handlers, before `init()`):

```go
// Attenuator control handlers

func (p *HardwarePlugin) handleSetAttenuator1(c *fiber.Ctx) error {
	var req struct {
		Db float64 `json:"db"`
	}
	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	cfg := p.config.Attenuator
	ctrl, err := NewPE4312Controller(p.config.SX1255.GPIOChip, cfg.DataPin, cfg.ClkPin, cfg.LE1Pin, cfg.LE2Pin)
	if err != nil {
		return SendError(c, 500, err)
	}
	defer ctrl.Close()

	if err := ctrl.SetAttenuation(1, req.Db); err != nil {
		return SendError(c, 500, err)
	}

	actualDb := float64(int(req.Db*2.0+0.5)) * 0.5
	if actualDb > 31.5 {
		actualDb = 31.5
	}

	slog.Info("Attenuator chip 1 set", "db", actualDb)
	return SendSuccess(c, map[string]interface{}{
		"chip":  1,
		"db":    actualDb,
		"value": int(actualDb * 2),
	}, fmt.Sprintf("Chip 1 set to %.1f dB", actualDb))
}

func (p *HardwarePlugin) handleSetAttenuator2(c *fiber.Ctx) error {
	var req struct {
		Db float64 `json:"db"`
	}
	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	cfg := p.config.Attenuator
	ctrl, err := NewPE4312Controller(p.config.SX1255.GPIOChip, cfg.DataPin, cfg.ClkPin, cfg.LE1Pin, cfg.LE2Pin)
	if err != nil {
		return SendError(c, 500, err)
	}
	defer ctrl.Close()

	if err := ctrl.SetAttenuation(2, req.Db); err != nil {
		return SendError(c, 500, err)
	}

	actualDb := float64(int(req.Db*2.0+0.5)) * 0.5
	if actualDb > 31.5 {
		actualDb = 31.5
	}

	slog.Info("Attenuator chip 2 set", "db", actualDb)
	return SendSuccess(c, map[string]interface{}{
		"chip":  2,
		"db":    actualDb,
		"value": int(actualDb * 2),
	}, fmt.Sprintf("Chip 2 set to %.1f dB", actualDb))
}

func (p *HardwarePlugin) handleSetAttenuatorBoth(c *fiber.Ctx) error {
	var req struct {
		Db float64 `json:"db"`
	}
	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	cfg := p.config.Attenuator
	ctrl, err := NewPE4312Controller(p.config.SX1255.GPIOChip, cfg.DataPin, cfg.ClkPin, cfg.LE1Pin, cfg.LE2Pin)
	if err != nil {
		return SendError(c, 500, err)
	}
	defer ctrl.Close()

	if err := ctrl.SetBoth(req.Db); err != nil {
		return SendError(c, 500, err)
	}

	actualDb := float64(int(req.Db*2.0+0.5)) * 0.5
	if actualDb > 31.5 {
		actualDb = 31.5
	}

	slog.Info("Attenuator both chips set", "db", actualDb)
	return SendSuccess(c, map[string]interface{}{
		"db":    actualDb,
		"value": int(actualDb * 2),
	}, fmt.Sprintf("Both chips set to %.1f dB", actualDb))
}

func (p *HardwarePlugin) handleGetAttenuator1(c *fiber.Ctx) error {
	cfg := p.config.Attenuator
	ctrl, err := NewPE4312Controller(p.config.SX1255.GPIOChip, cfg.DataPin, cfg.ClkPin, cfg.LE1Pin, cfg.LE2Pin)
	if err != nil {
		return SendError(c, 500, err)
	}
	defer ctrl.Close()

	val, err := ctrl.GetAttenuation(1)
	if err != nil {
		return SendError(c, 500, err)
	}

	return SendSuccess(c, map[string]interface{}{
		"chip":  1,
		"db":    float64(val) * 0.5,
		"value": val,
	}, "")
}

func (p *HardwarePlugin) handleGetAttenuator2(c *fiber.Ctx) error {
	cfg := p.config.Attenuator
	ctrl, err := NewPE4312Controller(p.config.SX1255.GPIOChip, cfg.DataPin, cfg.ClkPin, cfg.LE1Pin, cfg.LE2Pin)
	if err != nil {
		return SendError(c, 500, err)
	}
	defer ctrl.Close()

	val, err := ctrl.GetAttenuation(2)
	if err != nil {
		return SendError(c, 500, err)
	}

	return SendSuccess(c, map[string]interface{}{
		"chip":  2,
		"db":    float64(val) * 0.5,
		"value": val,
	}, "")
}

func (p *HardwarePlugin) handleAttenuatorInfo(c *fiber.Ctx) error {
	cfg := p.config.Attenuator
	return SendSuccess(c, map[string]interface{}{
		"config": map[string]interface{}{
			"data_pin": cfg.DataPin,
			"clk_pin":  cfg.ClkPin,
			"le1_pin":  cfg.LE1Pin,
			"le2_pin":  cfg.LE2Pin,
		},
	}, "")
}
```

#### 4d. Update the plugin factory `init()` — parse attenuator config + update TxRxPin default

Replace the factory init block (lines 764-806). Key changes:
- Parse attenuator config section
- Change default `tx_rx_pin` from 13 → 15

```go
// Register the plugin
func init() {
	Register("hardware", func(config interface{}) (Plugin, error) {
		configMap, ok := config.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid config for hardware plugin")
		}

		var hwConfig HardwareConfig

		// Parse SX1255 config with helper functions for type conversion
		if sx1255Cfg, ok := configMap["sx1255"].(map[string]interface{}); ok {
			if spiDevice, ok := sx1255Cfg["spi_device"].(string); ok {
				hwConfig.SX1255.SPIDevice = spiDevice
			}
			if spiSpeed, ok := toUint32(sx1255Cfg["spi_speed"]); ok {
				hwConfig.SX1255.SPISpeed = spiSpeed
			}
			if gpioChip, ok := sx1255Cfg["gpio_chip"].(string); ok {
				hwConfig.SX1255.GPIOChip = gpioChip
			}
			if resetPin, ok := toInt(sx1255Cfg["reset_pin"]); ok {
				hwConfig.SX1255.ResetPin = resetPin
			}
			if txRxPin, ok := toInt(sx1255Cfg["tx_rx_pin"]); ok {
				hwConfig.SX1255.TxRxPin = txRxPin
			} else {
				hwConfig.SX1255.TxRxPin = 15 // Default TX/RX pin
			}
			if clockFreq, ok := toUint32(sx1255Cfg["clock_freq"]); ok {
				hwConfig.SX1255.ClockFreq = clockFreq
			}
		}

		// Parse attenuator config
		if attCfg, ok := configMap["attenuator"].(map[string]interface{}); ok {
			if dataPin, ok := toInt(attCfg["data_pin"]); ok {
				hwConfig.Attenuator.DataPin = dataPin
			} else {
				hwConfig.Attenuator.DataPin = 6
			}
			if clkPin, ok := toInt(attCfg["clk_pin"]); ok {
				hwConfig.Attenuator.ClkPin = clkPin
			} else {
				hwConfig.Attenuator.ClkPin = 7
			}
			if le1Pin, ok := toInt(attCfg["le1_pin"]); ok {
				hwConfig.Attenuator.LE1Pin = le1Pin
			} else {
				hwConfig.Attenuator.LE1Pin = 12
			}
			if le2Pin, ok := toInt(attCfg["le2_pin"]); ok {
				hwConfig.Attenuator.LE2Pin = le2Pin
			} else {
				hwConfig.Attenuator.LE2Pin = 13
			}
		} else {
			hwConfig.Attenuator.DataPin = 6
			hwConfig.Attenuator.ClkPin = 7
			hwConfig.Attenuator.LE1Pin = 12
			hwConfig.Attenuator.LE2Pin = 13
		}

		slog.Info("Hardware plugin config parsed",
			"spi_device", hwConfig.SX1255.SPIDevice,
			"spi_speed", hwConfig.SX1255.SPISpeed,
			"gpio_chip", hwConfig.SX1255.GPIOChip,
			"reset_pin", hwConfig.SX1255.ResetPin,
			"tx_rx_pin", hwConfig.SX1255.TxRxPin,
			"clock_freq", hwConfig.SX1255.ClockFreq,
			"att_data_pin", hwConfig.Attenuator.DataPin,
			"att_clk_pin", hwConfig.Attenuator.ClkPin,
			"att_le1_pin", hwConfig.Attenuator.LE1Pin,
			"att_le2_pin", hwConfig.Attenuator.LE2Pin)

		return NewHardwarePlugin(hwConfig)
	})
}
```

### 5. web/index.html — Add attenuator section

Insert between "QUICK CONTROLS" section end (after `</div>` closing the QUICK CONTROLS div, around line 143) and the "REGISTER VIEWER" section start (around line 146).

The new section:

```html
            <!-- Attenuator Control Section -->
            <div class="hw-section">
                <h3 class="hw-section-title">&gt; ATTENUATOR CONTROL (PE4312)</h3>
                <div class="hw-controls-grid">
                    <div class="hw-control-group">
                        <label class="hw-label">Chip 1 (dB):</label>
                        <div class="hw-control-row">
                            <input type="number" id="hw-att1-db-input"
                                   value="0" step="0.5" min="0" max="31.5">
                            <button id="hw-set-att1-btn" class="btn btn-sm">Set</button>
                            <span id="hw-att1-status" class="hw-value">--</span>
                        </div>
                    </div>

                    <div class="hw-control-group">
                        <label class="hw-label">Chip 2 (dB):</label>
                        <div class="hw-control-row">
                            <input type="number" id="hw-att2-db-input"
                                   value="0" step="0.5" min="0" max="31.5">
                            <button id="hw-set-att2-btn" class="btn btn-sm">Set</button>
                            <span id="hw-att2-status" class="hw-value">--</span>
                        </div>
                    </div>

                    <div class="hw-control-group">
                        <label class="hw-label">Both Chips (dB):</label>
                        <div class="hw-control-row">
                            <input type="number" id="hw-att-both-db-input"
                                   value="0" step="0.5" min="0" max="31.5">
                            <button id="hw-set-att-both-btn" class="btn btn-sm">Set Both</button>
                        </div>
                    </div>
                </div>
            </div>
```

### 6. web/hardware.js — Add attenuator functions

Add at end of `initHardwareTab()` function (after existing event listeners, before closing brace):

```js
    // Attenuator control
    document.getElementById('hw-set-att1-btn').addEventListener('click', () => setAttenuator(1));
    document.getElementById('hw-set-att2-btn').addEventListener('click', () => setAttenuator(2));
    document.getElementById('hw-set-att-both-btn').addEventListener('click', setAttenuatorBoth);
```

Add new functions at end of file:

```js
// Set attenuator for a single chip
async function setAttenuator(chip) {
    const inputId = chip === 1 ? 'hw-att1-db-input' : 'hw-att2-db-input';
    const statusId = chip === 1 ? 'hw-att1-status' : 'hw-att2-status';
    const db = parseFloat(document.getElementById(inputId).value);

    if (isNaN(db) || db < 0 || db > 31.5) {
        showToast('Attenuation must be 0–31.5 dB in 0.5 dB steps', 'error');
        return;
    }

    await apiCall('Setting attenuator...', `/api/hardware/attenuator/${chip}`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({db: db})
    }, `Chip ${chip} set to ${db.toFixed(1)} dB`, (data) => {
        if (data && data.data && data.data.db !== undefined) {
            document.getElementById(statusId).textContent = data.data.db.toFixed(1) + ' dB';
            document.getElementById(statusId).className = 'hw-value status-ok';
            document.getElementById(inputId).value = data.data.db.toFixed(1);
        }
    });
}

// Set both chips to same attenuation
async function setAttenuatorBoth() {
    const db = parseFloat(document.getElementById('hw-att-both-db-input').value);

    if (isNaN(db) || db < 0 || db > 31.5) {
        showToast('Attenuation must be 0–31.5 dB in 0.5 dB steps', 'error');
        return;
    }

    await apiCall('Setting attenuators...', '/api/hardware/attenuator/both', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({db: db})
    }, `Both chips set to ${db.toFixed(1)} dB`, (data) => {
        if (data && data.data && data.data.db !== undefined) {
            const roundedDb = data.data.db.toFixed(1);
            document.getElementById('hw-att1-db-input').value = roundedDb;
            document.getElementById('hw-att2-db-input').value = roundedDb;
            document.getElementById('hw-att-both-db-input').value = roundedDb;
            document.getElementById('hw-att1-status').textContent = roundedDb + ' dB';
            document.getElementById('hw-att1-status').className = 'hw-value status-ok';
            document.getElementById('hw-att2-status').textContent = roundedDb + ' dB';
            document.getElementById('hw-att2-status').className = 'hw-value status-ok';
        }
    });
}
```

---

## Summary of all changes

| File | Action | Lines affected |
|------|--------|---------------|
| `config.yaml` | Modify | Change pin 13→15, add attenuator section (5 lines) |
| `main.go` | Modify | Add Attenuator struct (~5 lines), pass to plugin (~6 lines) |
| `plugins/hardware.go` | Modify | Update HardwareConfig (~4 lines), add routes (~7 lines), add handlers (~120 lines), update factory init (~30 lines), default pin 13→15 |
| `plugins/hardware_pe4312.go` | **NEW** | ~230 lines (entire file) |
| `web/index.html` | Modify | Add attenuator section (~30 lines) |
| `web/hardware.js` | Modify | Add event listeners (~3 lines), add functions (~55 lines) |

**Total:** ~1 new file, ~5 modified files, ~490 lines of changes.
