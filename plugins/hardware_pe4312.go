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

	dataLine, err := chip.RequestLine(dataPin, gpiocdev.AsOutput(0), gpiocdev.WithConsumer("pe4312-data"))
	if err != nil {
		chip.Close()
		return nil, fmt.Errorf("failed to request DATA pin %d: %w", dataPin, err)
	}
	ctrl.dataLine = dataLine

	clkLine, err := chip.RequestLine(clkPin, gpiocdev.AsOutput(0), gpiocdev.WithConsumer("pe4312-clk"))
	if err != nil {
		dataLine.Close()
		chip.Close()
		return nil, fmt.Errorf("failed to request CLK pin %d: %w", clkPin, err)
	}
	ctrl.clkLine = clkLine

	le1Line, err := chip.RequestLine(le1Pin, gpiocdev.AsOutput(0), gpiocdev.WithConsumer("pe4312-le1"))
	if err != nil {
		dataLine.Close()
		clkLine.Close()
		chip.Close()
		return nil, fmt.Errorf("failed to request LE1 pin %d: %w", le1Pin, err)
	}
	ctrl.le1Line = le1Line

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

func pe4312Tick() {
	time.Sleep(1 * time.Microsecond)
}

// sendWord clocks out a 6-bit value MSB-first on the shared DATA/CLK lines,
// then strobes the specified LE line to latch the value.
func (p *PE4312Controller) sendWord(bits uint8, leLine *gpiocdev.Line) error {
	if err := leLine.SetValue(0); err != nil {
		return fmt.Errorf("failed to set LE low: %w", err)
	}
	pe4312Tick()

	for i := 5; i >= 0; i-- {
		value := (bits >> i) & 1
		if err := p.dataLine.SetValue(int(value)); err != nil {
			return fmt.Errorf("failed to set DATA: %w", err)
		}
		pe4312Tick()

		if err := p.clkLine.SetValue(1); err != nil {
			return fmt.Errorf("failed to set CLK high: %w", err)
		}
		pe4312Tick()

		if err := p.clkLine.SetValue(0); err != nil {
			return fmt.Errorf("failed to set CLK low: %w", err)
		}
		pe4312Tick()
	}

	if err := leLine.SetValue(1); err != nil {
		return fmt.Errorf("failed to set LE high: %w", err)
	}
	pe4312Tick()
	if err := leLine.SetValue(0); err != nil {
		return fmt.Errorf("failed to set LE low: %w", err)
	}
	pe4312Tick()

	return nil
}

// SetAttenuation sets the attenuation for the specified chip (1 or 2).
// db: attenuation in dB (0.0–31.5, 0.5 dB resolution).
func (p *PE4312Controller) SetAttenuation(chip int, db float64) error {
	bits := int(db*2.0 + 0.5)
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

// GetAttenuation returns the last-set attenuation bits value for the specified chip.
// Returns bits (0-63), actual dB = bits * 0.5.
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
		"chip":     p.chipPath,
		"data_pin": p.dataPin,
		"clk_pin":  p.clkPin,
		"le1_pin":  p.le1Pin,
		"le2_pin":  p.le2Pin,
		"chip1_db": float64(p.lastAtt1) * 0.5,
		"chip2_db": float64(p.lastAtt2) * 0.5,
	}
}
