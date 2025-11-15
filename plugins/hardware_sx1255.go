package plugins

import (
	"fmt"
	"math"
)

// SX1255Controller provides high-level control of the SX1255 transceiver
type SX1255Controller struct {
	spi         *SPIDevice
	gpio        *GPIOController
	clockFreq   uint32
	initialized bool
}

// NewSX1255Controller creates a new SX1255 controller
func NewSX1255Controller(spiDevice string, spiSpeed uint32, gpioChip string, resetPin int, txRxPin int, clockFreq uint32) (*SX1255Controller, error) {
	controller := &SX1255Controller{
		clockFreq:   clockFreq,
		initialized: false,
	}

	// Initialize SPI
	spi, err := NewSPIDevice(spiDevice, spiSpeed)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize SPI: %w", err)
	}
	controller.spi = spi

	// Initialize GPIO
	gpio, err := NewGPIOController(gpioChip, resetPin, txRxPin)
	if err != nil {
		spi.Close()
		return nil, fmt.Errorf("failed to initialize GPIO: %w", err)
	}
	controller.gpio = gpio

	controller.initialized = true
	return controller, nil
}

// Close releases all resources
func (s *SX1255Controller) Close() error {
	var errs []error

	if s.spi != nil {
		if err := s.spi.Close(); err != nil {
			errs = append(errs, fmt.Errorf("SPI close error: %w", err))
		}
	}

	if s.gpio != nil {
		if err := s.gpio.Close(); err != nil {
			errs = append(errs, fmt.Errorf("GPIO close error: %w", err))
		}
	}

	s.initialized = false

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	return nil
}

// Reset performs a hardware reset
func (s *SX1255Controller) Reset() error {
	if !s.initialized {
		return fmt.Errorf("controller not initialized")
	}

	return s.gpio.Reset()
}

// GetVersion reads the chip version
func (s *SX1255Controller) GetVersion() (uint8, error) {
	if !s.initialized {
		return 0, fmt.Errorf("controller not initialized")
	}

	return s.spi.ReadRegister(RegVersion)
}

// GetVersionString returns a human-readable version string
func (s *SX1255Controller) GetVersionString() (string, error) {
	version, err := s.GetVersion()
	if err != nil {
		return "", err
	}

	major := (version >> 4) & 0x0F
	minor := version & 0x0F

	if minor > 0 {
		return fmt.Sprintf("V%d%c", major, 'A'+minor-1), nil
	}
	return fmt.Sprintf("V%d", major), nil
}

// ReadRegister reads a single register
func (s *SX1255Controller) ReadRegister(addr uint8) (uint8, error) {
	if !s.initialized {
		return 0, fmt.Errorf("controller not initialized")
	}

	return s.spi.ReadRegister(addr)
}

// WriteRegister writes to a single register
func (s *SX1255Controller) WriteRegister(addr uint8, value uint8) error {
	if !s.initialized {
		return fmt.Errorf("controller not initialized")
	}

	return s.spi.WriteRegister(addr, value)
}

// ReadAllRegisters reads all configuration registers (0x00-0x13)
func (s *SX1255Controller) ReadAllRegisters() (map[uint8]uint8, error) {
	if !s.initialized {
		return nil, fmt.Errorf("controller not initialized")
	}

	registers := make(map[uint8]uint8)

	for addr := uint8(0x00); addr <= RegDigBridge; addr++ {
		value, err := s.spi.ReadRegister(addr)
		if err != nil {
			return nil, fmt.Errorf("failed to read register 0x%02X: %w", addr, err)
		}
		registers[addr] = value
	}

	return registers, nil
}

// SetMode sets the operating mode
func (s *SX1255Controller) SetMode(mode uint8) error {
	if !s.initialized {
		return fmt.Errorf("controller not initialized")
	}

	return s.spi.WriteRegister(RegMode, mode)
}

// GetMode reads the current operating mode
func (s *SX1255Controller) GetMode() (uint8, error) {
	if !s.initialized {
		return 0, fmt.Errorf("controller not initialized")
	}

	return s.spi.ReadRegister(RegMode)
}

// SetRxFrequency sets the RX frequency in Hz
func (s *SX1255Controller) SetRxFrequency(freqHz uint32) error {
	if !s.initialized {
		return fmt.Errorf("controller not initialized")
	}

	// Validate frequency range (400-510 MHz per datasheet)
	if freqHz < 400000000 || freqHz > 510000000 {
		return fmt.Errorf("frequency %d Hz out of range (400-510 MHz)", freqHz)
	}

	// Calculate frequency register value
	// Frf = (FXOSC * Frfxx) / 2^20
	// Frfxx = (Frf * 2^20) / FXOSC
	frf := uint32(math.Round(float64(freqHz) * math.Pow(2, 20) / float64(s.clockFreq)))

	// Split into 3 bytes (MSB, Mid, LSB)
	msb := uint8((frf >> 16) & 0xFF)
	mid := uint8((frf >> 8) & 0xFF)
	lsb := uint8(frf & 0xFF)

	// Write frequency registers
	if err := s.spi.WriteRegister(RegFrfhRx, msb); err != nil {
		return fmt.Errorf("failed to write RX frequency MSB: %w", err)
	}
	if err := s.spi.WriteRegister(RegFrfmRx, mid); err != nil {
		return fmt.Errorf("failed to write RX frequency mid: %w", err)
	}
	if err := s.spi.WriteRegister(RegFrflRx, lsb); err != nil {
		return fmt.Errorf("failed to write RX frequency LSB: %w", err)
	}

	return nil
}

// GetRxFrequency reads the RX frequency in Hz
func (s *SX1255Controller) GetRxFrequency() (uint32, error) {
	if !s.initialized {
		return 0, fmt.Errorf("controller not initialized")
	}

	msb, err := s.spi.ReadRegister(RegFrfhRx)
	if err != nil {
		return 0, fmt.Errorf("failed to read RX frequency MSB: %w", err)
	}

	mid, err := s.spi.ReadRegister(RegFrfmRx)
	if err != nil {
		return 0, fmt.Errorf("failed to read RX frequency mid: %w", err)
	}

	lsb, err := s.spi.ReadRegister(RegFrflRx)
	if err != nil {
		return 0, fmt.Errorf("failed to read RX frequency LSB: %w", err)
	}

	// Combine bytes
	frf := (uint32(msb) << 16) | (uint32(mid) << 8) | uint32(lsb)

	// Calculate frequency: Frf = (FXOSC * Frfxx) / 2^20
	freqHz := uint32(math.Round(float64(s.clockFreq) * float64(frf) / math.Pow(2, 20)))

	return freqHz, nil
}

// SetTxFrequency sets the TX frequency in Hz
func (s *SX1255Controller) SetTxFrequency(freqHz uint32) error {
	if !s.initialized {
		return fmt.Errorf("controller not initialized")
	}

	// Validate frequency range (400-510 MHz per datasheet)
	if freqHz < 400000000 || freqHz > 510000000 {
		return fmt.Errorf("frequency %d Hz out of range (400-510 MHz)", freqHz)
	}

	// Calculate frequency register value
	frf := uint32(math.Round(float64(freqHz) * math.Pow(2, 20) / float64(s.clockFreq)))

	// Split into 3 bytes
	msb := uint8((frf >> 16) & 0xFF)
	mid := uint8((frf >> 8) & 0xFF)
	lsb := uint8(frf & 0xFF)

	// Write frequency registers
	if err := s.spi.WriteRegister(RegFrfhTx, msb); err != nil {
		return fmt.Errorf("failed to write TX frequency MSB: %w", err)
	}
	if err := s.spi.WriteRegister(RegFrfmTx, mid); err != nil {
		return fmt.Errorf("failed to write TX frequency mid: %w", err)
	}
	if err := s.spi.WriteRegister(RegFrflTx, lsb); err != nil {
		return fmt.Errorf("failed to write TX frequency LSB: %w", err)
	}

	return nil
}

// GetTxFrequency reads the TX frequency in Hz
func (s *SX1255Controller) GetTxFrequency() (uint32, error) {
	if !s.initialized {
		return 0, fmt.Errorf("controller not initialized")
	}

	msb, err := s.spi.ReadRegister(RegFrfhTx)
	if err != nil {
		return 0, fmt.Errorf("failed to read TX frequency MSB: %w", err)
	}

	mid, err := s.spi.ReadRegister(RegFrfmTx)
	if err != nil {
		return 0, fmt.Errorf("failed to read TX frequency mid: %w", err)
	}

	lsb, err := s.spi.ReadRegister(RegFrflTx)
	if err != nil {
		return 0, fmt.Errorf("failed to read TX frequency LSB: %w", err)
	}

	// Combine bytes
	frf := (uint32(msb) << 16) | (uint32(mid) << 8) | uint32(lsb)

	// Calculate frequency
	freqHz := uint32(math.Round(float64(s.clockFreq) * float64(frf) / math.Pow(2, 20)))

	return freqHz, nil
}

// GetPLLStatus reads the PLL lock status for both TX and RX
func (s *SX1255Controller) GetPLLStatus() (txLocked bool, rxLocked bool, err error) {
	if !s.initialized {
		return false, false, fmt.Errorf("controller not initialized")
	}

	stat, err := s.spi.ReadRegister(RegStat)
	if err != nil {
		return false, false, fmt.Errorf("failed to read status register: %w", err)
	}

	txLocked = (stat & StatPllLockTx) != 0
	rxLocked = (stat & StatPllLockRx) != 0

	return txLocked, rxLocked, nil
}

// GetStatus reads all status bits
func (s *SX1255Controller) GetStatus() (map[string]bool, error) {
	if !s.initialized {
		return nil, fmt.Errorf("controller not initialized")
	}

	stat, err := s.spi.ReadRegister(RegStat)
	if err != nil {
		return nil, fmt.Errorf("failed to read status register: %w", err)
	}

	status := map[string]bool{
		"eol":         (stat & StatEol) != 0,
		"xosc_ready":  (stat & StatXoscReady) != 0,
		"pll_lock_rx": (stat & StatPllLockRx) != 0,
		"pll_lock_tx": (stat & StatPllLockTx) != 0,
	}

	return status, nil
}

// SetLNAGain sets the LNA gain (0-48 dB range)
func (s *SX1255Controller) SetLNAGain(gainDb uint8) error {
	if !s.initialized {
		return fmt.Errorf("controller not initialized")
	}

	// Determine LNA gain setting based on dB value
	var lnaGainSetting uint8
	if gainDb > 45 {
		lnaGainSetting = LnaGainMax // 0 dB
	} else if gainDb > 39 {
		lnaGainSetting = LnaGainMinus6 // -6 dB
	} else if gainDb > 30 {
		lnaGainSetting = LnaGainMinus12 // -12 dB
	} else if gainDb > 18 {
		lnaGainSetting = LnaGainMinus24 // -24 dB
	} else if gainDb > 6 {
		lnaGainSetting = LnaGainMinus36 // -36 dB
	} else {
		lnaGainSetting = LnaGainMinus48 // -48 dB
	}

	// Read current register value
	reg, err := s.spi.ReadRegister(RegRxfe1)
	if err != nil {
		return fmt.Errorf("failed to read RXFE1 register: %w", err)
	}

	// Clear LNA gain bits (7:5) and set new value
	reg = (reg & 0x1F) | (lnaGainSetting << 5)

	return s.spi.WriteRegister(RegRxfe1, reg)
}

// SetPGAGain sets the PGA gain (0-30 dB in 2 dB steps)
func (s *SX1255Controller) SetPGAGain(gainDb uint8) error {
	if !s.initialized {
		return fmt.Errorf("controller not initialized")
	}

	if gainDb > 30 {
		gainDb = 30
	}

	// PGA gain in 2 dB steps
	pgaGainSetting := gainDb / 2

	// Read current register value
	reg, err := s.spi.ReadRegister(RegRxfe1)
	if err != nil {
		return fmt.Errorf("failed to read RXFE1 register: %w", err)
	}

	// Clear PGA gain bits (4:1) and set new value
	reg = (reg & 0xE1) | (pgaGainSetting << 1)

	return s.spi.WriteRegister(RegRxfe1, reg)
}

// SetDACGain sets the TX DAC gain
func (s *SX1255Controller) SetDACGain(gainDb int8) error {
	if !s.initialized {
		return fmt.Errorf("controller not initialized")
	}

	var dacGainSetting uint8
	switch gainDb {
	case 0:
		dacGainSetting = DacGainMax
	case -3:
		dacGainSetting = DacGainMinus3
	case -6:
		dacGainSetting = DacGainMinus6
	case -9:
		dacGainSetting = DacGainMinus9
	default:
		dacGainSetting = DacGainMinus3 // Default to -3 dB
	}

	// Read current register value
	reg, err := s.spi.ReadRegister(RegTxfe1)
	if err != nil {
		return fmt.Errorf("failed to read TXFE1 register: %w", err)
	}

	// Clear DAC gain bits (6:4) and set new value
	reg = (reg & 0x8F) | (dacGainSetting << 4)

	return s.spi.WriteRegister(RegTxfe1, reg)
}

// SetMixerGain sets the TX mixer gain (-37.5 to -7.5 dB in 2 dB steps)
func (s *SX1255Controller) SetMixerGain(gainDb float32) error {
	if !s.initialized {
		return fmt.Errorf("controller not initialized")
	}

	// Clamp to valid range
	if gainDb < -37.5 {
		gainDb = -37.5
	}
	if gainDb > -7.5 {
		gainDb = -7.5
	}

	// Calculate mixer gain setting
	mixerGainSetting := uint8(math.Round(float64(gainDb+37.5) / 2.0))

	// Read current register value
	reg, err := s.spi.ReadRegister(RegTxfe1)
	if err != nil {
		return fmt.Errorf("failed to read TXFE1 register: %w", err)
	}

	// Clear mixer gain bits (3:0) and set new value
	reg = (reg & 0xF0) | (mixerGainSetting & 0x0F)

	return s.spi.WriteRegister(RegTxfe1, reg)
}

// EnableRx enables or disables the RX path
func (s *SX1255Controller) EnableRx(enable bool) error {
	if !s.initialized {
		return fmt.Errorf("controller not initialized")
	}

	reg, err := s.spi.ReadRegister(RegMode)
	if err != nil {
		return err
	}

	if enable {
		reg |= ModeBitRxEnable
	} else {
		reg &= ^uint8(ModeBitRxEnable)
	}

	return s.spi.WriteRegister(RegMode, reg)
}

// EnableTx enables or disables the TX path (without PA)
func (s *SX1255Controller) EnableTx(enable bool) error {
	if !s.initialized {
		return fmt.Errorf("controller not initialized")
	}

	reg, err := s.spi.ReadRegister(RegMode)
	if err != nil {
		return err
	}

	if enable {
		reg |= ModeBitTxEnable
	} else {
		reg &= ^uint8(ModeBitTxEnable)
	}

	return s.spi.WriteRegister(RegMode, reg)
}

// EnablePA enables or disables the PA driver
func (s *SX1255Controller) EnablePA(enable bool) error {
	if !s.initialized {
		return fmt.Errorf("controller not initialized")
	}

	reg, err := s.spi.ReadRegister(RegMode)
	if err != nil {
		return err
	}

	if enable {
		reg |= ModeBitDriverEnable
	} else {
		reg &= ^uint8(ModeBitDriverEnable)
	}

	return s.spi.WriteRegister(RegMode, reg)
}

// SetTxRxSwitch controls the external TX/RX antenna switch
// true = TX mode, false = RX mode
func (s *SX1255Controller) SetTxRxSwitch(tx bool) error {
	if !s.initialized {
		return fmt.Errorf("controller not initialized")
	}

	return s.gpio.SetTxRxPin(tx)
}

// GetTxRxSwitch reads the current TX/RX switch state
func (s *SX1255Controller) GetTxRxSwitch() (bool, error) {
	if !s.initialized {
		return false, fmt.Errorf("controller not initialized")
	}

	return s.gpio.GetTxRxPin()
}

// Initialize performs basic initialization without modifying the device
// Only verifies SPI communication by reading the version register
func (s *SX1255Controller) Initialize() error {
	if !s.initialized {
		return fmt.Errorf("controller not initialized")
	}

	// Verify SPI communication by reading version register
	_, err := s.GetVersion()
	if err != nil {
		return fmt.Errorf("failed to verify SPI communication: %w", err)
	}

	// Initialization successful - device is accessible
	return nil
}

// InitializeWithDefaults performs full initialization with default register values
// Use this if you want to apply recommended configuration
func (s *SX1255Controller) InitializeWithDefaults() error {
	// First do basic init
	if err := s.Initialize(); err != nil {
		return err
	}

	// Apply default register values
	for addr, value := range DefaultRegisterValues {
		if err := s.spi.WriteRegister(addr, value); err != nil {
			return fmt.Errorf("failed to write default value to register 0x%02X: %w", addr, err)
		}
	}

	return nil
}

// IsInitialized returns true if the controller is initialized
func (s *SX1255Controller) IsInitialized() bool {
	return s.initialized
}

// Info returns information about the controller
func (s *SX1255Controller) Info() map[string]interface{} {
	info := map[string]interface{}{
		"initialized": s.initialized,
		"clock_freq":  s.clockFreq,
	}

	if s.spi != nil {
		info["spi"] = s.spi.DeviceInfo()
	}

	if s.gpio != nil {
		info["gpio"] = s.gpio.Info()
	}

	return info
}
