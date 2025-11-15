// Hardware module JavaScript

// Hardware state
let hardwareInitialized = false;

// DOM ready
document.addEventListener('DOMContentLoaded', function() {
    initHardwareTab();
});

function initHardwareTab() {
    // Initialization buttons
    document.getElementById('hw-init-btn').addEventListener('click', initializeHardware);
    document.getElementById('hw-reset-btn').addEventListener('click', resetHardware);
    document.getElementById('hw-close-btn').addEventListener('click', closeHardware);
    document.getElementById('hw-refresh-btn').addEventListener('click', refreshHardwareStatus);

    // Control buttons
    document.getElementById('hw-set-mode-btn').addEventListener('click', setMode);
    document.getElementById('hw-set-rx-freq-btn').addEventListener('click', setRxFrequency);
    document.getElementById('hw-set-tx-freq-btn').addEventListener('click', setTxFrequency);
    document.getElementById('hw-set-lna-gain-btn').addEventListener('click', setLNAGain);
    document.getElementById('hw-set-pga-gain-btn').addEventListener('click', setPGAGain);
    
    // TX/RX switch buttons
    document.getElementById('hw-txrx-rx-btn').addEventListener('click', () => setTxRxSwitch(false));
    document.getElementById('hw-txrx-tx-btn').addEventListener('click', () => setTxRxSwitch(true));

    // Register viewer buttons
    document.getElementById('hw-read-all-regs-btn').addEventListener('click', readAllRegisters);

    // Check status on tab load
    refreshHardwareStatus();
}

// Initialize hardware
async function initializeHardware() {
    try {
        showLoading('Initializing hardware...');
        const response = await fetch('/api/hardware/init', {
            method: 'POST'
        });
        const data = await response.json();
        hideLoading();

        if (data.success) {
            hardwareInitialized = true;
            showToast('Hardware initialized successfully!', 'success');
            await refreshHardwareStatus();
        } else {
            showToast('Failed to initialize hardware: ' + data.message, 'error');
        }
    } catch (error) {
        hideLoading();
        showToast('Error initializing hardware: ' + error.message, 'error');
    }
}

// Reset hardware
async function resetHardware() {
    if (!hardwareInitialized) {
        showToast('Hardware not initialized', 'error');
        return;
    }

    try {
        showLoading('Resetting hardware...');
        const response = await fetch('/api/hardware/reset', {
            method: 'POST'
        });
        const data = await response.json();
        hideLoading();

        if (data.success) {
            showToast('Hardware reset successful!', 'success');
            await refreshHardwareStatus();
        } else {
            showToast('Failed to reset hardware: ' + data.message, 'error');
        }
    } catch (error) {
        hideLoading();
        showToast('Error resetting hardware: ' + error.message, 'error');
    }
}

// Close hardware
async function closeHardware() {
    try {
        showLoading('Closing hardware...');
        const response = await fetch('/api/hardware/close', {
            method: 'POST'
        });
        const data = await response.json();
        hideLoading();

        if (data.success) {
            hardwareInitialized = false;
            showToast('Hardware closed', 'success');
            clearHardwareStatus();
        } else {
            showToast('Failed to close hardware: ' + data.message, 'error');
        }
    } catch (error) {
        hideLoading();
        showToast('Error closing hardware: ' + error.message, 'error');
    }
}

// Refresh hardware status
async function refreshHardwareStatus() {
    try {
        const response = await fetch('/api/hardware/status');
        const data = await response.json();

        if (data.success && data.data.initialized) {
            hardwareInitialized = true;
            updateStatusDisplay(data.data);
        } else {
            hardwareInitialized = false;
            clearHardwareStatus();
        }
    } catch (error) {
        console.error('Error refreshing status:', error);
        clearHardwareStatus();
    }
}

// Update status display
function updateStatusDisplay(status) {
    // Connection status
    document.getElementById('hw-conn-status').textContent = 'Connected';
    document.getElementById('hw-conn-status').className = 'hw-value status-ok';

    // Version
    document.getElementById('hw-version').textContent = status.version || '--';

    // Frequencies
    const rxFreqMHz = status.rx_freq ? (status.rx_freq / 1000000).toFixed(3) : '--';
    const txFreqMHz = status.tx_freq ? (status.tx_freq / 1000000).toFixed(3) : '--';
    document.getElementById('hw-rx-freq').textContent = rxFreqMHz + ' MHz';
    document.getElementById('hw-tx-freq').textContent = txFreqMHz + ' MHz';

    // Update input fields
    if (status.rx_freq) {
        document.getElementById('hw-rx-freq-input').value = rxFreqMHz;
    }
    if (status.tx_freq) {
        document.getElementById('hw-tx-freq-input').value = txFreqMHz;
    }

    // Mode
    let modeText = '--';
    if (status.mode !== undefined) {
        const modes = {
            0: 'Sleep',
            1: 'Standby',
            3: 'RX',
            5: 'TX',
            13: 'TX Full',
            15: 'Full-Duplex'
        };
        modeText = modes[status.mode] || 'Unknown (' + status.mode + ')';
    }
    document.getElementById('hw-mode').textContent = modeText;

    // Status flags
    if (status.status) {
        // RX PLL
        const rxPll = document.getElementById('hw-rx-pll');
        if (status.status.pll_lock_rx) {
            rxPll.textContent = 'Locked';
            rxPll.className = 'hw-value status-locked';
        } else {
            rxPll.textContent = 'Unlocked';
            rxPll.className = 'hw-value status-unlocked';
        }

        // TX PLL
        const txPll = document.getElementById('hw-tx-pll');
        if (status.status.pll_lock_tx) {
            txPll.textContent = 'Locked';
            txPll.className = 'hw-value status-locked';
        } else {
            txPll.textContent = 'Unlocked';
            txPll.className = 'hw-value status-unlocked';
        }

        // XOSC
        const xosc = document.getElementById('hw-xosc');
        if (status.status.xosc_ready) {
            xosc.textContent = 'Ready';
            xosc.className = 'hw-value status-ok';
        } else {
            xosc.textContent = 'Not Ready';
            xosc.className = 'hw-value status-error';
        }
    }
}

// Clear hardware status
function clearHardwareStatus() {
    document.getElementById('hw-conn-status').textContent = 'Not initialized';
    document.getElementById('hw-conn-status').className = 'hw-value';
    document.getElementById('hw-version').textContent = '--';
    document.getElementById('hw-rx-freq').textContent = '-- MHz';
    document.getElementById('hw-tx-freq').textContent = '-- MHz';
    document.getElementById('hw-rx-pll').textContent = '--';
    document.getElementById('hw-rx-pll').className = 'hw-value';
    document.getElementById('hw-tx-pll').textContent = '--';
    document.getElementById('hw-tx-pll').className = 'hw-value';
    document.getElementById('hw-xosc').textContent = '--';
    document.getElementById('hw-xosc').className = 'hw-value';
    document.getElementById('hw-mode').textContent = '--';
}

// Set operating mode
async function setMode() {
    if (!hardwareInitialized) {
        showToast('Hardware not initialized', 'error');
        return;
    }

    const mode = document.getElementById('hw-mode-select').value;
    
    try {
        showLoading('Setting mode...');
        const response = await fetch('/api/hardware/mode', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({mode: mode})
        });
        const data = await response.json();
        hideLoading();

        if (data.success) {
            showToast('Mode set to ' + mode.toUpperCase(), 'success');
            await refreshHardwareStatus();
        } else {
            showToast('Failed to set mode: ' + data.message, 'error');
        }
    } catch (error) {
        hideLoading();
        showToast('Error setting mode: ' + error.message, 'error');
    }
}

// Set RX frequency
async function setRxFrequency() {
    if (!hardwareInitialized) {
        showToast('Hardware not initialized', 'error');
        return;
    }

    const freqMHz = parseFloat(document.getElementById('hw-rx-freq-input').value);
    if (isNaN(freqMHz) || freqMHz < 400 || freqMHz > 510) {
        showToast('Invalid frequency. Must be between 400-510 MHz', 'error');
        return;
    }

    const freqHz = Math.round(freqMHz * 1000000);
    
    try {
        showLoading('Setting RX frequency...');
        const response = await fetch('/api/hardware/frequency/rx', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({frequency: freqHz})
        });
        const data = await response.json();
        hideLoading();

        if (data.success) {
            showToast('RX frequency set to ' + freqMHz.toFixed(3) + ' MHz', 'success');
            await refreshHardwareStatus();
        } else {
            showToast('Failed to set RX frequency: ' + data.message, 'error');
        }
    } catch (error) {
        hideLoading();
        showToast('Error setting RX frequency: ' + error.message, 'error');
    }
}

// Set TX frequency
async function setTxFrequency() {
    if (!hardwareInitialized) {
        showToast('Hardware not initialized', 'error');
        return;
    }

    const freqMHz = parseFloat(document.getElementById('hw-tx-freq-input').value);
    if (isNaN(freqMHz) || freqMHz < 400 || freqMHz > 510) {
        showToast('Invalid frequency. Must be between 400-510 MHz', 'error');
        return;
    }

    const freqHz = Math.round(freqMHz * 1000000);
    
    try {
        showLoading('Setting TX frequency...');
        const response = await fetch('/api/hardware/frequency/tx', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({frequency: freqHz})
        });
        const data = await response.json();
        hideLoading();

        if (data.success) {
            showToast('TX frequency set to ' + freqMHz.toFixed(3) + ' MHz', 'success');
            await refreshHardwareStatus();
        } else {
            showToast('Failed to set TX frequency: ' + data.message, 'error');
        }
    } catch (error) {
        hideLoading();
        showToast('Error setting TX frequency: ' + error.message, 'error');
    }
}

// Set LNA gain
async function setLNAGain() {
    if (!hardwareInitialized) {
        showToast('Hardware not initialized', 'error');
        return;
    }

    const gain = parseInt(document.getElementById('hw-lna-gain-input').value);
    if (isNaN(gain) || gain < 0 || gain > 48) {
        showToast('Invalid LNA gain. Must be between 0-48 dB', 'error');
        return;
    }
    
    try {
        showLoading('Setting LNA gain...');
        const response = await fetch('/api/hardware/gain/lna', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({gain: gain})
        });
        const data = await response.json();
        hideLoading();

        if (data.success) {
            showToast('LNA gain set to ' + gain + ' dB', 'success');
        } else {
            showToast('Failed to set LNA gain: ' + data.message, 'error');
        }
    } catch (error) {
        hideLoading();
        showToast('Error setting LNA gain: ' + error.message, 'error');
    }
}

// Set PGA gain
async function setPGAGain() {
    if (!hardwareInitialized) {
        showToast('Hardware not initialized', 'error');
        return;
    }

    const gain = parseInt(document.getElementById('hw-pga-gain-input').value);
    if (isNaN(gain) || gain < 0 || gain > 30) {
        showToast('Invalid PGA gain. Must be between 0-30 dB', 'error');
        return;
    }
    
    try {
        showLoading('Setting PGA gain...');
        const response = await fetch('/api/hardware/gain/pga', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({gain: gain})
        });
        const data = await response.json();
        hideLoading();

        if (data.success) {
            showToast('PGA gain set to ' + gain + ' dB', 'success');
        } else {
            showToast('Failed to set PGA gain: ' + data.message, 'error');
        }
    } catch (error) {
        hideLoading();
        showToast('Error setting PGA gain: ' + error.message, 'error');
    }
}

// Read all registers
async function readAllRegisters() {
    if (!hardwareInitialized) {
        showToast('Hardware not initialized', 'error');
        return;
    }

    try {
        showLoading('Reading registers...');
        const response = await fetch('/api/hardware/registers');
        const data = await response.json();
        hideLoading();

        if (data.success) {
            displayRegisters(data.data.registers);
            showToast('Read ' + data.data.count + ' registers', 'success');
        } else {
            showToast('Failed to read registers: ' + data.message, 'error');
        }
    } catch (error) {
        hideLoading();
        showToast('Error reading registers: ' + error.message, 'error');
    }
}

// Display registers in table
function displayRegisters(registers) {
    const tbody = document.getElementById('hw-register-list');
    tbody.innerHTML = '';

    registers.forEach(reg => {
        const addrNum = parseInt(reg.address, 16);
        
        // Convert decimal to binary
        const binary = reg.value_dec.toString(2).padStart(8, '0');
        const binaryFormatted = binary.substring(0, 4) + binary.substring(4);
        
        const row = document.createElement('tr');
        row.innerHTML = `
            <td class="hw-register-addr">${reg.address}</td>
            <td>
                <div>${reg.description.split(' - ')[0]}</div>
                <div class="hw-register-name">${reg.description.split(' - ')[1] || ''}</div>
            </td>
            <td>
                <input type="text" id="reg-hex-${addrNum}" class="hw-input hw-reg-input"
                       value="${reg.value_dec.toString(16).toUpperCase().padStart(2, '0')}"
                       maxlength="2"
                       oninput="syncHexToBinary(${addrNum})" />
            </td>
            <td>
                <input type="text" id="reg-bin-${addrNum}" class="hw-input hw-reg-input-bin"
                       value="${binaryFormatted}"
                       maxlength="9"
                       oninput="syncBinaryToHex(${addrNum})" />
            </td>
            <td>
                <button class="btn btn-sm" onclick="readRegister(${addrNum})">Read</button>
                <button class="btn btn-sm btn-primary" onclick="writeRegisterFromInput(${addrNum})">Write</button>
            </td>
        `;
        tbody.appendChild(row);
    });
}

// Read single register
async function readRegister(addrNum) {
    if (!hardwareInitialized) {
        showToast('Hardware not initialized', 'error');
        return;
    }
    
    try {
        const response = await fetch(`/api/hardware/register/${addrNum}`);
        const data = await response.json();

        if (data.success) {
            const value = data.data.value_dec;
            
            // Update hex input
            const hexInput = document.getElementById(`reg-hex-${addrNum}`);
            if (hexInput) {
                hexInput.value = value.toString(16).toUpperCase().padStart(2, '0');
            }
            
            // Update binary input
            const binary = value.toString(2).padStart(8, '0');
            const binInput = document.getElementById(`reg-bin-${addrNum}`);
            if (binInput) {
                binInput.value = binary.substring(0, 4) + binary.substring(4);
            }
            
            showToast('Register 0x' + addrNum.toString(16).toUpperCase().padStart(2, '0') + ' = 0x' + value.toString(16).toUpperCase().padStart(2, '0'), 'success');
        } else {
            showToast('Failed to read register: ' + data.message, 'error');
        }
    } catch (error) {
        showToast('Error reading register: ' + error.message, 'error');
    }
}

// Sync hex input to binary
function syncHexToBinary(addrNum) {
    const hexInput = document.getElementById(`reg-hex-${addrNum}`);
    const binInput = document.getElementById(`reg-bin-${addrNum}`);
    
    if (!hexInput || !binInput) return;
    
    const hexValue = hexInput.value.replace(/[^0-9A-Fa-f]/g, '').substring(0, 2);
    hexInput.value = hexValue.toUpperCase();
    
    if (hexValue.length === 2) {
        const decimal = parseInt(hexValue, 16);
        const binary = decimal.toString(2).padStart(8, '0');
        binInput.value = binary.substring(0, 4) + binary.substring(4);
    }
}

// Sync binary input to hex
function syncBinaryToHex(addrNum) {
    const binInput = document.getElementById(`reg-bin-${addrNum}`);
    const hexInput = document.getElementById(`reg-hex-${addrNum}`);
    
    if (!hexInput || !binInput) return;
    
    // Remove non-binary characters and space
    let binValue = binInput.value.replace(/[^01]/g, '').substring(0, 8);
    
    // Add space after 4th bit for readability
    if (binValue.length > 4) {
        binInput.value = binValue.substring(0, 4) + binValue.substring(4);
    } else {
        binInput.value = binValue;
    }
    
    if (binValue.length === 8) {
        const decimal = parseInt(binValue, 2);
        hexInput.value = decimal.toString(16).toUpperCase().padStart(2, '0');
    }
}

// Write register from input fields
async function writeRegisterFromInput(addrNum) {
    if (!hardwareInitialized) {
        showToast('Hardware not initialized', 'error');
        return;
    }

    const hexInput = document.getElementById(`reg-hex-${addrNum}`);
    if (!hexInput) return;

    const hexValue = hexInput.value;
    if (!/^[0-9A-Fa-f]{2}$/.test(hexValue)) {
        showToast('Invalid hex value. Must be 2 hex digits (00-FF)', 'error');
        return;
    }

    const value = parseInt(hexValue, 16);
    
    try {
        showLoading('Writing register...');
        const response = await fetch(`/api/hardware/register/${addrNum}`, {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({value: value})
        });
        const data = await response.json();
        hideLoading();

        if (data.success) {
            showToast('Register 0x' + addrNum.toString(16).toUpperCase().padStart(2, '0') + ' written', 'success');
            await readRegister(addrNum);
        } else {
            showToast('Failed to write register: ' + data.message, 'error');
        }
    } catch (error) {
        hideLoading();
        showToast('Error writing register: ' + error.message, 'error');
    }
}

// Set TX/RX switch
async function setTxRxSwitch(tx) {
    if (!hardwareInitialized) {
        showToast('Hardware not initialized', 'error');
        return;
    }

    const mode = tx ? 'TX' : 'RX';
    
    try {
        showLoading('Setting TX/RX switch...');
        const response = await fetch('/api/hardware/txrx-switch', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({tx: tx})
        });
        const data = await response.json();
        hideLoading();

        if (data.success) {
            showToast('TX/RX switch set to ' + mode, 'success');
            updateTxRxStatus(tx);
        } else {
            showToast('Failed to set TX/RX switch: ' + data.message, 'error');
        }
    } catch (error) {
        hideLoading();
        showToast('Error setting TX/RX switch: ' + error.message, 'error');
    }
}

// Update TX/RX switch status display
function updateTxRxStatus(tx) {
    const statusEl = document.getElementById('hw-txrx-status');
    if (tx) {
        statusEl.textContent = 'TX';
        statusEl.className = 'hw-value status-error'; // Red for TX
    } else {
        statusEl.textContent = 'RX';
        statusEl.className = 'hw-value status-ok'; // Green for RX
    }
}

// Get TX/RX switch status
async function getTxRxSwitchStatus() {
    if (!hardwareInitialized) {
        return;
    }

    try {
        const response = await fetch('/api/hardware/txrx-switch');
        const data = await response.json();

        if (data.success) {
            updateTxRxStatus(data.data.tx);
        }
    } catch (error) {
        console.error('Error getting TX/RX status:', error);
    }
}