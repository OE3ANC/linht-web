// Terminal component using xterm.js
class WebShellTerminal {
    constructor() {
        this.term = null;
        this.socket = null;
        this.fitAddon = null;
        this.sessionType = null;
    }
    
    // Initialize xterm.js terminal
    init(containerId) {
        if (this.term) {
            this.destroy();
        }

        const container = document.getElementById(containerId);
        if (!container) {
            console.error('Terminal container not found:', containerId);
            return;
        }

        this.term = new Terminal({
            cursorBlink: true,
            fontSize: 14,
            fontFamily: 'Menlo, Monaco, "Courier New", monospace',
            theme: {
                background: '#000000',
                foreground: '#d4d4d4',
                cursor: '#aeafad',
                black: '#000000',
                red: '#cd3131',
                green: '#0dbc79',
                yellow: '#e5e510',
                blue: '#2472c8',
                magenta: '#bc3fbc',
                cyan: '#11a8cd',
                white: '#e5e5e5',
                brightBlack: '#666666',
                brightRed: '#f14c4c',
                brightGreen: '#23d18b',
                brightYellow: '#f5f543',
                brightBlue: '#3b8eea',
                brightMagenta: '#d670d6',
                brightCyan: '#29b8db',
                brightWhite: '#ffffff',
            },
            cols: 80,
            rows: 24,
        });
        
        // Load fit addon
        this.fitAddon = new FitAddon.FitAddon();
        this.term.loadAddon(this.fitAddon);
        
        // Open terminal in container
        this.term.open(container);
        this.fitAddon.fit();
        
        // Handle window resize
        this.resizeHandler = () => {
            if (this.term && this.fitAddon) {
                this.fitAddon.fit();
                this.sendResize();
            }
        };
        window.addEventListener('resize', this.resizeHandler);
        
        return this;
    }
    
    // Connect to host shell
    connectHost() {
        this.sessionType = 'host';
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${location.host}/api/webshell/ws?type=host`;
        this.connect(wsUrl);
    }
    
    // Connect to container shell
    connectContainer(containerId) {
        this.sessionType = 'container';
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${location.host}/api/webshell/ws?type=container&container=${encodeURIComponent(containerId)}`;
        this.connect(wsUrl);
    }
    
    // Connect to WebSocket
    connect(url) {
        if (this.socket) {
            this.socket.close();
        }

        this.socket = new WebSocket(url);
        
        this.socket.onopen = () => {
            this.term.write('\r\n\x1b[32m*** Connected ***\x1b[0m\r\n\r\n');
            
            // Send initial resize
            this.sendResize();
            
            // Handle terminal input
            this.term.onData(data => {
                if (this.socket && this.socket.readyState === WebSocket.OPEN) {
                    this.socket.send(data);
                }
            });
        };
        
        this.socket.onmessage = (event) => {
            if (this.term) {
                this.term.write(event.data);
            }
        };
        
        this.socket.onerror = (error) => {
            console.error('WebSocket error:', error);
            if (this.term) {
                this.term.write('\r\n\x1b[31m*** Connection Error ***\x1b[0m\r\n');
            }
        };
        
        this.socket.onclose = () => {
            if (this.term) {
                this.term.write('\r\n\x1b[33m*** Connection Closed ***\x1b[0m\r\n');
            }
        };
    }
    
    // Send terminal resize to backend
    sendResize() {
        if (this.socket && this.socket.readyState === WebSocket.OPEN && this.term) {
            const resizeMsg = JSON.stringify({
                type: 'resize',
                rows: this.term.rows,
                cols: this.term.cols
            });
            this.socket.send(resizeMsg);
        }
    }
    
    // Disconnect and cleanup
    disconnect() {
        if (this.socket) {
            this.socket.close();
            this.socket = null;
        }
    }
    
    // Destroy terminal instance
    destroy() {
        this.disconnect();
        
        if (this.resizeHandler) {
            window.removeEventListener('resize', this.resizeHandler);
            this.resizeHandler = null;
        }
        
        if (this.term) {
            this.term.dispose();
            this.term = null;
        }
        
        this.fitAddon = null;
    }
}

// Global terminal instance
let currentTerminal = null;

// Open host shell
function openHostShell() {
    if (currentTerminal) {
        currentTerminal.destroy();
    }
    
    currentTerminal = new WebShellTerminal();
    currentTerminal.init('xterm-container');
    currentTerminal.connectHost();
}

// Open container shell selection modal
async function openContainerShell() {
    try {
        const response = await api('/api/webshell/containers');
        const data = await response.json();
        
        if (!data.success || !data.data || data.data.length === 0) {
            showToast('No running containers found', 'error');
            return;
        }
        
        // Show container selection modal
        showContainerSelectModal(data.data);
    } catch (error) {
        showToast('Failed to load containers', 'error');
    }
}

// Show container selection modal
function showContainerSelectModal(containers) {
    const modal = document.getElementById('container-select-modal');
    const list = document.getElementById('container-select-list');
    
    list.innerHTML = containers.map(container => `
        <div class="card" style="cursor: pointer; margin-bottom: 10px;" onclick="selectContainer('${container.id}')">
            <div class="card-info">
                <div class="card-title">${container.name}</div>
                <div class="card-meta">ID: ${container.id.substring(0, 12)} • Image: ${container.image}</div>
            </div>
        </div>
    `).join('');
    
    modal.classList.remove('hidden');
}

// Select container and open shell
function selectContainer(containerId) {
    // Close modal
    document.getElementById('container-select-modal').classList.add('hidden');
    
    // Open terminal
    if (currentTerminal) {
        currentTerminal.destroy();
    }
    
    currentTerminal = new WebShellTerminal();
    currentTerminal.init('xterm-container');
    currentTerminal.connectContainer(containerId);
}

// Close terminal
function closeTerminal() {
    if (currentTerminal) {
        currentTerminal.destroy();
        currentTerminal = null;
    }
    
    // Clear terminal container
    const container = document.getElementById('xterm-container');
    if (container) {
        container.innerHTML = '';
    }
}