package plugins

// SX1255 Register addresses
const (
	// General registers
	RegMode    = 0x00 // Operating mode control
	RegFrfhRx  = 0x01 // RX frequency MSB
	RegFrfmRx  = 0x02 // RX frequency middle byte
	RegFrflRx  = 0x03 // RX frequency LSB
	RegFrfhTx  = 0x04 // TX frequency MSB
	RegFrfmTx  = 0x05 // TX frequency middle byte
	RegFrflTx  = 0x06 // TX frequency LSB
	RegVersion = 0x07 // Chip version

	// Transmitter registers
	RegTxfe1 = 0x08 // TX front-end 1: DAC and mixer gain
	RegTxfe2 = 0x09 // TX front-end 2: mixer tank settings
	RegTxfe3 = 0x0A // TX front-end 3: PLL bandwidth and filter
	RegTxfe4 = 0x0B // TX front-end 4: DAC bandwidth

	// Receiver registers
	RegRxfe1 = 0x0C // RX front-end 1: LNA and PGA gain
	RegRxfe2 = 0x0D // RX front-end 2: ADC bandwidth and trim
	RegRxfe3 = 0x0E // RX front-end 3: PLL bandwidth and temp sensor

	// IRQ and pin mapping
	RegIoMap = 0x0F // DIO pin mapping

	// Misc registers
	RegCkSel     = 0x10 // Clock select and loopback
	RegStat      = 0x11 // Status register
	RegIism      = 0x12 // I/Q interface mode
	RegDigBridge = 0x13 // Digital bridge configuration
)

// Register bit masks and values

// RegMode (0x00) bits
const (
	ModeBitDriverEnable = 1 << 3 // Enable PA driver
	ModeBitTxEnable     = 1 << 2 // Enable TX path (except PA)
	ModeBitRxEnable     = 1 << 1 // Enable RX path
	ModeBitRefEnable    = 1 << 0 // Enable PDS & XOSC
)

// Operating modes
const (
	ModeSleep      = 0x00                                                                       // Sleep mode
	ModeStandby    = ModeBitRefEnable                                                           // Standby with XOSC enabled
	ModeRx         = ModeBitRefEnable | ModeBitRxEnable                                         // RX mode
	ModeTx         = ModeBitRefEnable | ModeBitTxEnable                                         // TX mode (no PA)
	ModeTxFull     = ModeBitRefEnable | ModeBitTxEnable | ModeBitDriverEnable                   // TX with PA
	ModeFullDuplex = ModeBitRefEnable | ModeBitRxEnable | ModeBitTxEnable | ModeBitDriverEnable // Full-duplex with PA
)

// RegCkSel (0x10) bits
const (
	CkSelDigLoopback = 1 << 3 // Enable digital loopback
	CkSelRfLoopback  = 1 << 2 // Enable RF loopback
	CkSelCkoutEnable = 1 << 1 // Enable CLK_OUT
	CkSelTxDacExtClk = 1 << 0 // Use external clock for TX DAC
)

// RegStat (0x11) bits
const (
	StatEol       = 1 << 3 // End of life (battery low)
	StatXoscReady = 1 << 2 // XOSC ready
	StatPllLockRx = 1 << 1 // RX PLL locked
	StatPllLockTx = 1 << 0 // TX PLL locked
)

// Register descriptions for UI
var RegisterDescriptions = map[uint8]string{
	RegMode:      "MODE - Operating mode control",
	RegFrfhRx:    "FRFH_RX - RX frequency MSB",
	RegFrfmRx:    "FRFM_RX - RX frequency middle",
	RegFrflRx:    "FRFL_RX - RX frequency LSB",
	RegFrfhTx:    "FRFH_TX - TX frequency MSB",
	RegFrfmTx:    "FRFM_TX - TX frequency middle",
	RegFrflTx:    "FRFL_TX - TX frequency LSB",
	RegVersion:   "VERSION - Chip version",
	RegTxfe1:     "TXFE1 - TX DAC and mixer gain",
	RegTxfe2:     "TXFE2 - TX mixer tank settings",
	RegTxfe3:     "TXFE3 - TX PLL BW and filter",
	RegTxfe4:     "TXFE4 - TX DAC bandwidth",
	RegRxfe1:     "RXFE1 - RX LNA and PGA gain",
	RegRxfe2:     "RXFE2 - RX ADC BW and trim",
	RegRxfe3:     "RXFE3 - RX PLL BW and temp",
	RegIoMap:     "IO_MAP - DIO pin mapping",
	RegCkSel:     "CK_SEL - Clock and loopback",
	RegStat:      "STAT - Status register",
	RegIism:      "IISM - I/Q interface mode",
	RegDigBridge: "DIG_BRIDGE - Digital bridge config",
}

// LNA gain settings (RegRxfe1 bits 7:5)
const (
	LnaGainMax     = 1 // 0 dB (highest gain)
	LnaGainMinus6  = 2 // -6 dB
	LnaGainMinus12 = 3 // -12 dB
	LnaGainMinus24 = 4 // -24 dB
	LnaGainMinus36 = 5 // -36 dB
	LnaGainMinus48 = 6 // -48 dB
)

// DAC gain settings (RegTxfe1 bits 6:4)
const (
	DacGainMinus9 = 0 // Max gain - 9 dB
	DacGainMinus6 = 1 // Max gain - 6 dB
	DacGainMinus3 = 2 // Max gain - 3 dB
	DacGainMax    = 3 // Max gain (0 dBFS)
)

// Default register values
var DefaultRegisterValues = map[uint8]uint8{
	RegMode:   0x01, // Standby mode
	RegFrfhRx: 0xC0, // 434 MHz RX
	RegFrfmRx: 0xE3,
	RegFrflRx: 0x8E,
	RegFrfhTx: 0xC0, // 434 MHz TX
	RegFrfmTx: 0xE3,
	RegFrflTx: 0x8E,
	RegTxfe1:  0x2E, // DAC -3dB, Mixer -9.5dB
	RegTxfe2:  0x24, // Tank cap/res default
	RegTxfe3:  0x60, // PLL BW default
	RegTxfe4:  0x02, // DAC BW 32 taps
	RegRxfe1:  0x2F, // LNA max, PGA 30dB, 200ohm
	RegRxfe2:  0xA5, // ADC BW config
	RegRxfe3:  0x06, // RX PLL BW
	RegCkSel:  0x02, // CLK_OUT enabled
}
