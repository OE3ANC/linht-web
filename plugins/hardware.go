package plugins

import (
	"fmt"
	"log/slog"

	"github.com/gofiber/fiber/v2"
)

// HardwarePlugin provides SX1255 transceiver control
// Uses transient connections - initializes and releases for each operation
type HardwarePlugin struct {
	config HardwareConfig
}

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
}

// NewHardwarePlugin creates a new hardware plugin instance
func NewHardwarePlugin(cfg HardwareConfig) (*HardwarePlugin, error) {
	// Set defaults if not configured
	if cfg.SX1255.SPISpeed == 0 {
		cfg.SX1255.SPISpeed = 500000 // Default 500 kHz
	}
	if cfg.SX1255.ClockFreq == 0 {
		cfg.SX1255.ClockFreq = 32000000 // Default 32 MHz
	}

	slog.Info("Hardware plugin initializing",
		"spi_device", cfg.SX1255.SPIDevice,
		"spi_speed", cfg.SX1255.SPISpeed,
		"gpio_chip", cfg.SX1255.GPIOChip,
		"reset_pin", cfg.SX1255.ResetPin,
		"clock_freq", cfg.SX1255.ClockFreq)

	return &HardwarePlugin{
		config: cfg,
	}, nil
}

// Name returns the plugin identifier
func (p *HardwarePlugin) Name() string {
	return "hardware"
}

// RegisterRoutes adds the plugin's HTTP routes
func (p *HardwarePlugin) RegisterRoutes(app *fiber.App) {
	api := app.Group("/api/hardware")

	// Device control endpoints
	api.Post("/init", p.handleInit)
	api.Post("/reset", p.handleReset)
	api.Post("/close", p.handleClose)
	api.Get("/status", p.handleStatus)
	api.Get("/info", p.handleInfo)

	// Register access endpoints
	api.Get("/register/:addr", p.handleReadRegister)
	api.Post("/register/:addr", p.handleWriteRegister)
	api.Get("/registers", p.handleReadAllRegisters)
	api.Post("/registers/burst", p.handleBurstWrite)

	// High-level control endpoints
	api.Post("/frequency/rx", p.handleSetRxFrequency)
	api.Get("/frequency/rx", p.handleGetRxFrequency)
	api.Post("/frequency/tx", p.handleSetTxFrequency)
	api.Get("/frequency/tx", p.handleGetTxFrequency)

	api.Post("/mode", p.handleSetMode)
	api.Get("/mode", p.handleGetMode)

	api.Post("/gain/lna", p.handleSetLNAGain)
	api.Post("/gain/pga", p.handleSetPGAGain)
	api.Post("/gain/dac", p.handleSetDACGain)
	api.Post("/gain/mixer", p.handleSetMixerGain)

	api.Post("/enable/rx", p.handleEnableRx)
	api.Post("/enable/tx", p.handleEnableTx)
	api.Post("/enable/pa", p.handleEnablePA)

	api.Get("/pll-status", p.handleGetPLLStatus)

	// TX/RX switch control
	api.Post("/txrx-switch", p.handleSetTxRxSwitch)
	api.Get("/txrx-switch", p.handleGetTxRxSwitch)

	slog.Info("Hardware plugin routes registered")
}

// Shutdown performs cleanup
func (p *HardwarePlugin) Shutdown() error {
	// No persistent resources to clean up
	return nil
}

// createController creates a temporary controller for an operation
func (p *HardwarePlugin) createController() (*SX1255Controller, error) {
	cfg := p.config.SX1255
	return NewSX1255Controller(
		cfg.SPIDevice,
		cfg.SPISpeed,
		cfg.GPIOChip,
		cfg.ResetPin,
		cfg.TxRxPin,
		cfg.ClockFreq,
	)
}

// withController executes a function with a temporary controller
func (p *HardwarePlugin) withController(fn func(*SX1255Controller) error) error {
	controller, err := p.createController()
	if err != nil {
		return err
	}
	defer controller.Close()

	return fn(controller)
}

// Device control handlers

func (p *HardwarePlugin) handleInit(c *fiber.Ctx) error {
	var version string
	var info map[string]interface{}

	err := p.withController(func(ctrl *SX1255Controller) error {
		// Verify communication
		if err := ctrl.Initialize(); err != nil {
			return err
		}

		// Get version
		ver, err := ctrl.GetVersionString()
		if err != nil {
			version = "unknown"
		} else {
			version = ver
		}

		info = ctrl.Info()
		return nil
	})

	if err != nil {
		slog.Error("Failed to initialize hardware", "error", err)
		return SendError(c, 500, err)
	}

	slog.Info("Hardware connection verified", "version", version)
	return SendSuccess(c, map[string]interface{}{
		"version": version,
		"info":    info,
	}, "Hardware connection verified")
}

func (p *HardwarePlugin) handleReset(c *fiber.Ctx) error {
	err := p.withController(func(ctrl *SX1255Controller) error {
		return ctrl.Reset()
	})

	if err != nil {
		slog.Error("Failed to reset hardware", "error", err)
		return SendError(c, 500, err)
	}

	slog.Info("Hardware reset successful")
	return SendSuccess(c, nil, "Hardware reset successful")
}

func (p *HardwarePlugin) handleClose(c *fiber.Ctx) error {
	// No persistent connection to close
	return SendSuccess(c, nil, "No persistent connection (transient mode)")
}

func (p *HardwarePlugin) handleStatus(c *fiber.Ctx) error {
	var status map[string]bool
	var version string
	var rxFreq, txFreq uint32
	var mode uint8

	err := p.withController(func(ctrl *SX1255Controller) error {
		var err error
		status, err = ctrl.GetStatus()
		if err != nil {
			return err
		}

		version, _ = ctrl.GetVersionString()
		rxFreq, _ = ctrl.GetRxFrequency()
		txFreq, _ = ctrl.GetTxFrequency()
		mode, _ = ctrl.GetMode()
		return nil
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	return SendSuccess(c, map[string]interface{}{
		"initialized": true,
		"version":     version,
		"status":      status,
		"rx_freq":     rxFreq,
		"tx_freq":     txFreq,
		"mode":        mode,
	}, "")
}

func (p *HardwarePlugin) handleInfo(c *fiber.Ctx) error {
	return SendSuccess(c, map[string]interface{}{
		"config": p.config,
		"mode":   "transient",
	}, "")
}

// Register access handlers

func (p *HardwarePlugin) handleReadRegister(c *fiber.Ctx) error {
	addr, err := c.ParamsInt("addr")
	if err != nil || addr < 0 || addr > 0xFF {
		return SendErrorMessage(c, 400, "Invalid register address")
	}

	var value uint8
	err = p.withController(func(ctrl *SX1255Controller) error {
		var err error
		value, err = ctrl.ReadRegister(uint8(addr))
		return err
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	desc := RegisterDescriptions[uint8(addr)]
	if desc == "" {
		desc = "Unknown register"
	}

	return SendSuccess(c, map[string]interface{}{
		"address":     fmt.Sprintf("0x%02X", addr),
		"value":       fmt.Sprintf("0x%02X", value),
		"value_dec":   value,
		"description": desc,
	}, "")
}

func (p *HardwarePlugin) handleWriteRegister(c *fiber.Ctx) error {
	addr, err := c.ParamsInt("addr")
	if err != nil || addr < 0 || addr > 0xFF {
		return SendErrorMessage(c, 400, "Invalid register address")
	}

	var req struct {
		Value uint8 `json:"value"`
	}
	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	err = p.withController(func(ctrl *SX1255Controller) error {
		return ctrl.WriteRegister(uint8(addr), req.Value)
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	slog.Info("Register write", "address", fmt.Sprintf("0x%02X", addr), "value", fmt.Sprintf("0x%02X", req.Value))
	return SendSuccess(c, nil, "Register written successfully")
}

func (p *HardwarePlugin) handleReadAllRegisters(c *fiber.Ctx) error {
	var registers map[uint8]uint8

	err := p.withController(func(ctrl *SX1255Controller) error {
		var err error
		registers, err = ctrl.ReadAllRegisters()
		return err
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	// Format for JSON response
	regList := make([]map[string]interface{}, 0)
	for addr := uint8(0x00); addr <= RegDigBridge; addr++ {
		value, ok := registers[addr]
		if !ok {
			continue
		}

		desc := RegisterDescriptions[addr]
		if desc == "" {
			desc = "Unknown"
		}

		regList = append(regList, map[string]interface{}{
			"address":     fmt.Sprintf("0x%02X", addr),
			"value":       fmt.Sprintf("0x%02X", value),
			"value_dec":   value,
			"description": desc,
		})
	}

	return SendSuccess(c, map[string]interface{}{
		"registers": regList,
		"count":     len(regList),
	}, "")
}

func (p *HardwarePlugin) handleBurstWrite(c *fiber.Ctx) error {
	var req struct {
		Registers []struct {
			Address uint8 `json:"address"`
			Value   uint8 `json:"value"`
		} `json:"registers"`
	}

	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	err := p.withController(func(ctrl *SX1255Controller) error {
		// Write each register
		for _, reg := range req.Registers {
			if err := ctrl.WriteRegister(reg.Address, reg.Value); err != nil {
				return fmt.Errorf("failed to write register 0x%02X: %w", reg.Address, err)
			}
		}
		return nil
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	slog.Info("Burst write completed", "count", len(req.Registers))
	return SendSuccess(c, nil, fmt.Sprintf("Wrote %d registers successfully", len(req.Registers)))
}

// Frequency control handlers

func (p *HardwarePlugin) handleSetRxFrequency(c *fiber.Ctx) error {
	var req struct {
		Frequency uint32 `json:"frequency"`
	}
	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	err := p.withController(func(ctrl *SX1255Controller) error {
		return ctrl.SetRxFrequency(req.Frequency)
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	slog.Info("RX frequency set", "frequency", req.Frequency)
	return SendSuccess(c, map[string]interface{}{
		"frequency": req.Frequency,
	}, "RX frequency set successfully")
}

func (p *HardwarePlugin) handleGetRxFrequency(c *fiber.Ctx) error {
	var freq uint32

	err := p.withController(func(ctrl *SX1255Controller) error {
		var err error
		freq, err = ctrl.GetRxFrequency()
		return err
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	return SendSuccess(c, map[string]interface{}{
		"frequency": freq,
	}, "")
}

func (p *HardwarePlugin) handleSetTxFrequency(c *fiber.Ctx) error {
	var req struct {
		Frequency uint32 `json:"frequency"`
	}
	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	err := p.withController(func(ctrl *SX1255Controller) error {
		return ctrl.SetTxFrequency(req.Frequency)
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	slog.Info("TX frequency set", "frequency", req.Frequency)
	return SendSuccess(c, map[string]interface{}{
		"frequency": req.Frequency,
	}, "TX frequency set successfully")
}

func (p *HardwarePlugin) handleGetTxFrequency(c *fiber.Ctx) error {
	var freq uint32

	err := p.withController(func(ctrl *SX1255Controller) error {
		var err error
		freq, err = ctrl.GetTxFrequency()
		return err
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	return SendSuccess(c, map[string]interface{}{
		"frequency": freq,
	}, "")
}

// Mode control handlers

func (p *HardwarePlugin) handleSetMode(c *fiber.Ctx) error {
	var req struct {
		Mode string `json:"mode"`
	}
	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	var modeValue uint8
	switch req.Mode {
	case "sleep":
		modeValue = ModeSleep
	case "standby":
		modeValue = ModeStandby
	case "rx":
		modeValue = ModeRx
	case "tx":
		modeValue = ModeTx
	case "tx_full":
		modeValue = ModeTxFull
	case "full_duplex":
		modeValue = ModeFullDuplex
	default:
		return SendErrorMessage(c, 400, "Invalid mode. Use: sleep, standby, rx, tx, tx_full, or full_duplex")
	}

	err := p.withController(func(ctrl *SX1255Controller) error {
		return ctrl.SetMode(modeValue)
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	slog.Info("Mode set", "mode", req.Mode)
	return SendSuccess(c, map[string]interface{}{
		"mode": req.Mode,
	}, "Mode set successfully")
}

func (p *HardwarePlugin) handleGetMode(c *fiber.Ctx) error {
	var modeValue uint8

	err := p.withController(func(ctrl *SX1255Controller) error {
		var err error
		modeValue, err = ctrl.GetMode()
		return err
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	var modeName string
	switch modeValue {
	case ModeSleep:
		modeName = "sleep"
	case ModeStandby:
		modeName = "standby"
	case ModeRx:
		modeName = "rx"
	case ModeTx:
		modeName = "tx"
	case ModeTxFull:
		modeName = "tx_full"
	case ModeFullDuplex:
		modeName = "full_duplex"
	default:
		modeName = "unknown"
	}

	return SendSuccess(c, map[string]interface{}{
		"mode":       modeName,
		"mode_value": modeValue,
	}, "")
}

// Gain control handlers

func (p *HardwarePlugin) handleSetLNAGain(c *fiber.Ctx) error {
	var req struct {
		Gain uint8 `json:"gain"`
	}
	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	err := p.withController(func(ctrl *SX1255Controller) error {
		return ctrl.SetLNAGain(req.Gain)
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	slog.Info("LNA gain set", "gain", req.Gain)
	return SendSuccess(c, nil, "LNA gain set successfully")
}

func (p *HardwarePlugin) handleSetPGAGain(c *fiber.Ctx) error {
	var req struct {
		Gain uint8 `json:"gain"`
	}
	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	err := p.withController(func(ctrl *SX1255Controller) error {
		return ctrl.SetPGAGain(req.Gain)
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	slog.Info("PGA gain set", "gain", req.Gain)
	return SendSuccess(c, nil, "PGA gain set successfully")
}

func (p *HardwarePlugin) handleSetDACGain(c *fiber.Ctx) error {
	var req struct {
		Gain int8 `json:"gain"`
	}
	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	err := p.withController(func(ctrl *SX1255Controller) error {
		return ctrl.SetDACGain(req.Gain)
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	slog.Info("DAC gain set", "gain", req.Gain)
	return SendSuccess(c, nil, "DAC gain set successfully")
}

func (p *HardwarePlugin) handleSetMixerGain(c *fiber.Ctx) error {
	var req struct {
		Gain float32 `json:"gain"`
	}
	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	err := p.withController(func(ctrl *SX1255Controller) error {
		return ctrl.SetMixerGain(req.Gain)
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	slog.Info("Mixer gain set", "gain", req.Gain)
	return SendSuccess(c, nil, "Mixer gain set successfully")
}

// Enable control handlers

func (p *HardwarePlugin) handleEnableRx(c *fiber.Ctx) error {
	var req struct {
		Enable bool `json:"enable"`
	}
	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	err := p.withController(func(ctrl *SX1255Controller) error {
		return ctrl.EnableRx(req.Enable)
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	slog.Info("RX enable", "enable", req.Enable)
	return SendSuccess(c, nil, fmt.Sprintf("RX %s", map[bool]string{true: "enabled", false: "disabled"}[req.Enable]))
}

func (p *HardwarePlugin) handleEnableTx(c *fiber.Ctx) error {
	var req struct {
		Enable bool `json:"enable"`
	}
	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	err := p.withController(func(ctrl *SX1255Controller) error {
		return ctrl.EnableTx(req.Enable)
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	slog.Info("TX enable", "enable", req.Enable)
	return SendSuccess(c, nil, fmt.Sprintf("TX %s", map[bool]string{true: "enabled", false: "disabled"}[req.Enable]))
}

func (p *HardwarePlugin) handleEnablePA(c *fiber.Ctx) error {
	var req struct {
		Enable bool `json:"enable"`
	}
	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	err := p.withController(func(ctrl *SX1255Controller) error {
		return ctrl.EnablePA(req.Enable)
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	slog.Info("PA enable", "enable", req.Enable)
	return SendSuccess(c, nil, fmt.Sprintf("PA %s", map[bool]string{true: "enabled", false: "disabled"}[req.Enable]))
}

// Status handlers

func (p *HardwarePlugin) handleGetPLLStatus(c *fiber.Ctx) error {
	var txLocked, rxLocked bool

	err := p.withController(func(ctrl *SX1255Controller) error {
		var err error
		txLocked, rxLocked, err = ctrl.GetPLLStatus()
		return err
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	return SendSuccess(c, map[string]interface{}{
		"tx_locked": txLocked,
		"rx_locked": rxLocked,
	}, "")
}

// TX/RX switch handlers

func (p *HardwarePlugin) handleSetTxRxSwitch(c *fiber.Ctx) error {
	var req struct {
		Tx bool `json:"tx"`
	}
	if err := c.BodyParser(&req); err != nil {
		return SendErrorMessage(c, 400, "Invalid request body")
	}

	err := p.withController(func(ctrl *SX1255Controller) error {
		return ctrl.SetTxRxSwitch(req.Tx)
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	mode := "RX"
	if req.Tx {
		mode = "TX"
	}

	slog.Info("TX/RX switch set", "mode", mode)
	return SendSuccess(c, map[string]interface{}{
		"tx":   req.Tx,
		"mode": mode,
	}, fmt.Sprintf("TX/RX switch set to %s", mode))
}

func (p *HardwarePlugin) handleGetTxRxSwitch(c *fiber.Ctx) error {
	var tx bool

	err := p.withController(func(ctrl *SX1255Controller) error {
		var err error
		tx, err = ctrl.GetTxRxSwitch()
		return err
	})

	if err != nil {
		return SendError(c, 500, err)
	}

	mode := "RX"
	if tx {
		mode = "TX"
	}

	return SendSuccess(c, map[string]interface{}{
		"tx":   tx,
		"mode": mode,
	}, "")
}

// Register the plugin
func init() {
	Register("hardware", func(config interface{}) (Plugin, error) {
		configMap, ok := config.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid config for hardware plugin")
		}

		var hwConfig HardwareConfig

		// Parse SX1255 config with proper type handling
		if sx1255Cfg, ok := configMap["sx1255"].(map[string]interface{}); ok {
			if spiDevice, ok := sx1255Cfg["spi_device"].(string); ok {
				hwConfig.SX1255.SPIDevice = spiDevice
			}
			// Handle both int and uint32 for spi_speed
			if spiSpeed, ok := sx1255Cfg["spi_speed"].(int); ok {
				hwConfig.SX1255.SPISpeed = uint32(spiSpeed)
			} else if spiSpeed, ok := sx1255Cfg["spi_speed"].(uint32); ok {
				hwConfig.SX1255.SPISpeed = spiSpeed
			} else if spiSpeed, ok := sx1255Cfg["spi_speed"].(int64); ok {
				hwConfig.SX1255.SPISpeed = uint32(spiSpeed)
			}
			if gpioChip, ok := sx1255Cfg["gpio_chip"].(string); ok {
				hwConfig.SX1255.GPIOChip = gpioChip
			}
			if resetPin, ok := sx1255Cfg["reset_pin"].(int); ok {
				hwConfig.SX1255.ResetPin = resetPin
			}
			if txRxPin, ok := sx1255Cfg["tx_rx_pin"].(int); ok {
				hwConfig.SX1255.TxRxPin = txRxPin
			} else {
				// Default TX/RX pin if not specified
				hwConfig.SX1255.TxRxPin = 13
			}
			// Handle both int and uint32 for clock_freq
			if clockFreq, ok := sx1255Cfg["clock_freq"].(int); ok {
				hwConfig.SX1255.ClockFreq = uint32(clockFreq)
			} else if clockFreq, ok := sx1255Cfg["clock_freq"].(uint32); ok {
				hwConfig.SX1255.ClockFreq = clockFreq
			} else if clockFreq, ok := sx1255Cfg["clock_freq"].(int64); ok {
				hwConfig.SX1255.ClockFreq = uint32(clockFreq)
			}
		}

		slog.Info("Hardware plugin config parsed",
			"spi_device", hwConfig.SX1255.SPIDevice,
			"spi_speed", hwConfig.SX1255.SPISpeed,
			"gpio_chip", hwConfig.SX1255.GPIOChip,
			"reset_pin", hwConfig.SX1255.ResetPin,
			"tx_rx_pin", hwConfig.SX1255.TxRxPin,
			"clock_freq", hwConfig.SX1255.ClockFreq)

		return NewHardwarePlugin(hwConfig)
	})
}
