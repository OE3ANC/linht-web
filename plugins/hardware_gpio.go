package plugins

import (
	"fmt"
	"time"

	"github.com/warthog618/go-gpiocdev"
)

// GPIOController manages GPIO operations for the SX1255
type GPIOController struct {
	chip      *gpiocdev.Chip
	resetLine *gpiocdev.Line
	txRxLine  *gpiocdev.Line
	chipPath  string
	resetPin  int
	txRxPin   int
}

// NewGPIOController creates a new GPIO controller
func NewGPIOController(chipPath string, resetPin int, txRxPin int) (*GPIOController, error) {
	// Open GPIO chip
	chip, err := gpiocdev.NewChip(chipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open GPIO chip %s: %w", chipPath, err)
	}

	controller := &GPIOController{
		chip:     chip,
		chipPath: chipPath,
		resetPin: resetPin,
		txRxPin:  txRxPin,
	}

	// Request the reset pin as output, initially low
	resetLine, err := chip.RequestLine(
		resetPin,
		gpiocdev.AsOutput(0),
		gpiocdev.WithConsumer("sx1255-reset"),
	)
	if err != nil {
		chip.Close()
		return nil, fmt.Errorf("failed to request reset pin %d: %w", resetPin, err)
	}
	controller.resetLine = resetLine

	// Request the TX/RX switch pin as output, initially low (RX mode)
	txRxLine, err := chip.RequestLine(
		txRxPin,
		gpiocdev.AsOutput(0),
		gpiocdev.WithConsumer("sx1255-txrx"),
	)
	if err != nil {
		resetLine.Close()
		chip.Close()
		return nil, fmt.Errorf("failed to request TX/RX pin %d: %w", txRxPin, err)
	}
	controller.txRxLine = txRxLine

	return controller, nil
}

// Close releases all GPIO resources
func (g *GPIOController) Close() error {
	var errs []error

	if g.txRxLine != nil {
		if err := g.txRxLine.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close TX/RX line: %w", err))
		}
		g.txRxLine = nil
	}

	if g.resetLine != nil {
		if err := g.resetLine.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close reset line: %w", err))
		}
		g.resetLine = nil
	}

	if g.chip != nil {
		if err := g.chip.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close GPIO chip: %w", err))
		}
		g.chip = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing GPIO: %v", errs)
	}

	return nil
}

// Reset performs a hardware reset of the SX1255
// According to the datasheet:
// - Pull reset pin HIGH for at least 100us
// - Release to LOW
// - Wait 5ms before further operations
func (g *GPIOController) Reset() error {
	if g.resetLine == nil {
		return fmt.Errorf("reset line not initialized")
	}

	// Set reset pin HIGH
	if err := g.resetLine.SetValue(1); err != nil {
		return fmt.Errorf("failed to set reset pin HIGH: %w", err)
	}

	// Hold HIGH for 100us (datasheet minimum)
	time.Sleep(100 * time.Microsecond)

	// Set reset pin LOW
	if err := g.resetLine.SetValue(0); err != nil {
		return fmt.Errorf("failed to set reset pin LOW: %w", err)
	}

	// Wait 5ms for chip to be ready (per datasheet)
	time.Sleep(5 * time.Millisecond)

	return nil
}

// SetResetPin manually controls the reset pin state
func (g *GPIOController) SetResetPin(high bool) error {
	if g.resetLine == nil {
		return fmt.Errorf("reset line not initialized")
	}

	value := 0
	if high {
		value = 1
	}

	if err := g.resetLine.SetValue(value); err != nil {
		return fmt.Errorf("failed to set reset pin to %v: %w", high, err)
	}

	return nil
}

// GetResetPin reads the current state of the reset pin
func (g *GPIOController) GetResetPin() (bool, error) {
	if g.resetLine == nil {
		return false, fmt.Errorf("reset line not initialized")
	}

	value, err := g.resetLine.Value()
	if err != nil {
		return false, fmt.Errorf("failed to read reset pin: %w", err)
	}

	return value == 1, nil
}

// SetTxRxPin controls the TX/RX switch pin
// true = TX mode, false = RX mode
func (g *GPIOController) SetTxRxPin(tx bool) error {
	if g.txRxLine == nil {
		return fmt.Errorf("TX/RX line not initialized")
	}

	value := 0
	if tx {
		value = 1
	}

	if err := g.txRxLine.SetValue(value); err != nil {
		return fmt.Errorf("failed to set TX/RX pin to %v: %w", tx, err)
	}

	return nil
}

// GetTxRxPin reads the current state of the TX/RX switch pin
func (g *GPIOController) GetTxRxPin() (bool, error) {
	if g.txRxLine == nil {
		return false, fmt.Errorf("TX/RX line not initialized")
	}

	value, err := g.txRxLine.Value()
	if err != nil {
		return false, fmt.Errorf("failed to read TX/RX pin: %w", err)
	}

	return value == 1, nil
}

// Info returns information about the GPIO controller
func (g *GPIOController) Info() string {
	if g.chip == nil {
		return fmt.Sprintf("GPIO: %s (closed)", g.chipPath)
	}

	name := g.chip.Name
	label := g.chip.Label

	return fmt.Sprintf("GPIO: %s (%s, %s), Reset Pin: %d, TX/RX Pin: %d",
		g.chipPath, name, label, g.resetPin, g.txRxPin)
}

// IsInitialized checks if the GPIO controller is properly initialized
func (g *GPIOController) IsInitialized() bool {
	return g.chip != nil && g.resetLine != nil && g.txRxLine != nil
}

// Reinitialize closes and reopens the GPIO controller
func (g *GPIOController) Reinitialize() error {
	chipPath := g.chipPath
	resetPin := g.resetPin
	txRxPin := g.txRxPin

	if err := g.Close(); err != nil {
		return fmt.Errorf("failed to close during reinitialization: %w", err)
	}

	newController, err := NewGPIOController(chipPath, resetPin, txRxPin)
	if err != nil {
		return err
	}

	g.chip = newController.chip
	g.resetLine = newController.resetLine
	g.txRxLine = newController.txRxLine

	return nil
}

// ValidateGPIOChip checks if the GPIO chip exists and is accessible
func ValidateGPIOChip(chipPath string) error {
	// Try to open the chip
	chip, err := gpiocdev.NewChip(chipPath)
	if err != nil {
		return fmt.Errorf("cannot access GPIO chip %s: %w", chipPath, err)
	}
	defer chip.Close()

	// Verify chip name is accessible
	if chip.Name == "" {
		return fmt.Errorf("GPIO chip %s has invalid name", chipPath)
	}

	return nil
}

// ValidateGPIOPin checks if a specific pin number is valid for the chip
func ValidateGPIOPin(chipPath string, pin int) error {
	chip, err := gpiocdev.NewChip(chipPath)
	if err != nil {
		return fmt.Errorf("cannot access GPIO chip %s: %w", chipPath, err)
	}
	defer chip.Close()

	if pin < 0 {
		return fmt.Errorf("invalid pin %d: must be non-negative", pin)
	}

	// Try to get line info to validate pin
	info, err := chip.LineInfo(pin)
	if err != nil {
		return fmt.Errorf("invalid pin %d for chip %s: %w", pin, chipPath, err)
	}

	if info.Name == "" && info.Consumer == "" {
		// Pin exists but may not be available
	}

	return nil
}

// GetChipInfo returns detailed information about the GPIO chip
func GetChipInfo(chipPath string) (map[string]interface{}, error) {
	chip, err := gpiocdev.NewChip(chipPath)
	if err != nil {
		return nil, fmt.Errorf("cannot access GPIO chip %s: %w", chipPath, err)
	}
	defer chip.Close()

	result := map[string]interface{}{
		"name":  chip.Name,
		"label": chip.Label,
		"path":  chipPath,
	}

	return result, nil
}
