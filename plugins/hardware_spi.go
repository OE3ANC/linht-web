package plugins

import (
	"fmt"
	"time"

	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
)

// SPIDevice represents an SPI device using periph.io
type SPIDevice struct {
	conn   spi.Conn
	port   spi.PortCloser
	device string
	speed  physic.Frequency
}

// NewSPIDevice opens and initializes an SPI device using periph.io
func NewSPIDevice(device string, speed uint32) (*SPIDevice, error) {
	// Initialize periph.io host
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize periph.io: %w", err)
	}

	// Open SPI port
	port, err := spireg.Open(device)
	if err != nil {
		return nil, fmt.Errorf("failed to open SPI device %s: %w", device, err)
	}

	// Create SPI connection with configuration
	// SX1255 requires SPI Mode 0 (CPOL=0, CPHA=0)
	conn, err := port.Connect(physic.Frequency(speed)*physic.Hertz, spi.Mode0, 8)
	if err != nil {
		port.Close()
		return nil, fmt.Errorf("failed to connect to SPI device: %w", err)
	}

	return &SPIDevice{
		conn:   conn,
		port:   port,
		device: device,
		speed:  physic.Frequency(speed) * physic.Hertz,
	}, nil
}

// Close closes the SPI device
func (s *SPIDevice) Close() error {
	if s.port != nil {
		return s.port.Close()
	}
	return nil
}

// Transfer performs a full-duplex SPI transfer
func (s *SPIDevice) Transfer(tx []byte, rx []byte) error {
	if len(tx) != len(rx) {
		return fmt.Errorf("tx and rx buffers must be the same length")
	}

	if s.conn == nil {
		return fmt.Errorf("SPI device not open")
	}

	// Perform SPI transaction
	if err := s.conn.Tx(tx, rx); err != nil {
		return fmt.Errorf("SPI transfer failed: %w", err)
	}

	return nil
}

// WriteRegister writes a value to an SX1255 register
func (s *SPIDevice) WriteRegister(addr uint8, value uint8) error {
	// SX1255 write operation: MSB of address is 1
	tx := []byte{addr | 0x80, value}
	rx := make([]byte, 2)

	if err := s.Transfer(tx, rx); err != nil {
		return fmt.Errorf("failed to write register 0x%02X: %w", addr, err)
	}

	// Small delay per SX1255 spec
	time.Sleep(10 * time.Microsecond)

	return nil
}

// ReadRegister reads a value from an SX1255 register
func (s *SPIDevice) ReadRegister(addr uint8) (uint8, error) {
	// SX1255 read operation: MSB of address is 0
	tx := []byte{addr & 0x7F, 0x00}
	rx := make([]byte, 2)

	if err := s.Transfer(tx, rx); err != nil {
		return 0, fmt.Errorf("failed to read register 0x%02X: %w", addr, err)
	}

	// Small delay per SX1255 spec
	time.Sleep(10 * time.Microsecond)

	// Return the second byte (register value)
	return rx[1], nil
}

// BurstWrite writes multiple values to consecutive registers
func (s *SPIDevice) BurstWrite(startAddr uint8, values []uint8) error {
	if len(values) == 0 {
		return fmt.Errorf("no values to write")
	}

	// Prepare buffer: address byte + data bytes
	tx := make([]byte, len(values)+1)
	tx[0] = startAddr | 0x80 // Write operation
	copy(tx[1:], values)

	rx := make([]byte, len(tx))

	if err := s.Transfer(tx, rx); err != nil {
		return fmt.Errorf("failed to burst write starting at 0x%02X: %w", startAddr, err)
	}

	time.Sleep(10 * time.Microsecond)

	return nil
}

// BurstRead reads multiple values from consecutive registers
func (s *SPIDevice) BurstRead(startAddr uint8, count int) ([]uint8, error) {
	if count <= 0 {
		return nil, fmt.Errorf("invalid count: %d", count)
	}

	// Prepare buffer: address byte + dummy bytes for reading
	tx := make([]byte, count+1)
	tx[0] = startAddr & 0x7F // Read operation
	// Rest are dummy bytes

	rx := make([]byte, len(tx))

	if err := s.Transfer(tx, rx); err != nil {
		return nil, fmt.Errorf("failed to burst read starting at 0x%02X: %w", startAddr, err)
	}

	time.Sleep(10 * time.Microsecond)

	// Return data bytes (skip first address byte)
	return rx[1:], nil
}

// CheckDevice verifies SPI communication by reading the version register
func (s *SPIDevice) CheckDevice() (uint8, error) {
	version, err := s.ReadRegister(RegVersion)
	if err != nil {
		return 0, fmt.Errorf("failed to read version register: %w", err)
	}
	return version, nil
}

// DeviceInfo provides information about the SPI device
func (s *SPIDevice) DeviceInfo() string {
	if s.conn == nil {
		return fmt.Sprintf("Device: %s (closed)", s.device)
	}
	return fmt.Sprintf("Device: %s, Speed: %s", s.device, s.speed)
}

// IsOpen returns true if the SPI device is open
func (s *SPIDevice) IsOpen() bool {
	return s.conn != nil && s.port != nil
}

// Reopen closes and reopens the SPI device
func (s *SPIDevice) Reopen() error {
	device := s.device
	speed := uint32(s.speed / physic.Hertz)

	if err := s.Close(); err != nil {
		return fmt.Errorf("failed to close device during reopen: %w", err)
	}

	newSpi, err := NewSPIDevice(device, speed)
	if err != nil {
		return err
	}

	s.conn = newSpi.conn
	s.port = newSpi.port
	s.device = device
	s.speed = newSpi.speed

	return nil
}

// ValidateSPIDevice checks if the device can be opened
func ValidateSPIDevice(device string) error {
	// Initialize periph.io
	if _, err := host.Init(); err != nil {
		return fmt.Errorf("failed to initialize periph.io: %w", err)
	}

	// Try to open the device
	port, err := spireg.Open(device)
	if err != nil {
		return fmt.Errorf("SPI device %s not accessible: %w", device, err)
	}
	defer port.Close()

	return nil
}
