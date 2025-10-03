// Toast duration constants
const TOAST_DURATION = 3000;           // milliseconds
const ERROR_TOAST_DURATION = 10000;    // milliseconds

// Authentication
let authToken = localStorage.getItem('authToken') || '';

// DOM Elements
const loginScreen = document.getElementById('login-screen');
const app = document.getElementById('app');
const loginForm = document.getElementById('login-form');
const loginError = document.getElementById('login-error');
const passwordInput = document.getElementById('password');

// Loading Overlay
function showLoading(message = 'Communicating with daemon...') {
    const overlay = document.getElementById('loading-overlay');
    const status = document.getElementById('loading-status');
    status.textContent = message;
    overlay.classList.remove('hidden');
}

function hideLoading() {
    const overlay = document.getElementById('loading-overlay');
    overlay.classList.add('hidden');
}

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    setupEventListeners();
    
    // Restore session if token exists
    if (authToken) {
        restoreSession();
    }
});

function setupEventListeners() {
    // Login
    loginForm.addEventListener('submit', handleLogin);
    
    // Logout
    document.getElementById('logout-btn').addEventListener('click', handleLogout);
    
    // Tab switching
    document.querySelectorAll('.nav-tab').forEach(tab => {
        tab.addEventListener('click', () => switchTab(tab.dataset.tab));
    });
    
    // Images
    document.getElementById('refresh-images').addEventListener('click', loadImages);
    document.getElementById('import-file').addEventListener('change', handleImageImport);
    
    // Containers
    document.getElementById('refresh-containers').addEventListener('click', loadContainers);
    document.getElementById('create-container-btn').addEventListener('click', openCreateModal);
    document.getElementById('create-container-form').addEventListener('submit', handleCreateContainer);
    
    // Modal close buttons
    document.querySelectorAll('.modal-close').forEach(btn => {
        btn.addEventListener('click', () => {
            btn.closest('.modal').classList.add('hidden');
        });
    });
}

// Authentication
async function handleLogin(e) {
    e.preventDefault();
    const password = passwordInput.value;
    
    try {
        const response = await fetch('/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ password })
        });
        
        const data = await response.json();
        
        if (response.ok && data.success) {
            authToken = data.token;
            localStorage.setItem('authToken', authToken);
            loginScreen.classList.add('hidden');
            app.classList.remove('hidden');
            loginError.classList.add('hidden');
            passwordInput.value = '';
            
            // Load initial data
            loadImages();
            loadContainers();
        } else {
            showLoginError('Invalid password');
        }
    } catch (error) {
        showLoginError('Connection error');
    }
}

async function handleLogout() {
    try {
        await api('/logout', { method: 'POST' });
    } catch (error) {
        // Logout locally even if server call fails
    }
    
    authToken = '';
    localStorage.removeItem('authToken');
    app.classList.add('hidden');
    loginScreen.classList.remove('hidden');
    passwordInput.focus();
}

// Session restoration
async function restoreSession() {
    try {
        const response = await api('/api/images');
        
        if (response.ok) {
            showApp();
            loadInitialData();
        } else if (response.status === 401) {
            handleLogout();
        } else {
            // Other error, but auth might be OK
            showApp();
            loadInitialData();
        }
    } catch (error) {
        // Network error - assume session is valid
        if (error.message === 'Unauthorized') {
            handleLogout();
        } else {
            showApp();
        }
    }
}

function showApp() {
    loginScreen.classList.add('hidden');
    app.classList.remove('hidden');
}

function loadInitialData() {
    loadImages();
    loadContainers();
}

function showLoginError(message) {
    loginError.textContent = message;
    loginError.classList.remove('hidden');
}

// Tab Management
function switchTab(tabName) {
    // Update nav tabs
    document.querySelectorAll('.nav-tab').forEach(tab => {
        tab.classList.toggle('active', tab.dataset.tab === tabName);
    });
    
    // Update tab content
    document.querySelectorAll('.tab-content').forEach(content => {
        const isActive = content.id === `${tabName}-tab`;
        content.classList.toggle('hidden', !isActive);
        content.classList.toggle('active', isActive);
    });
    
    // Load data for active tab
    const dataLoaders = {
        images: loadImages,
        containers: loadContainers,
        files: () => FileManager.init()
    };
    
    const loader = dataLoaders[tabName];
    if (loader) {
        loader();
    }
}

// API Helper
async function api(url, options = {}) {
    const response = await fetch(url, {
        ...options,
        headers: {
            'X-Auth-Token': authToken,
            ...options.headers
        }
    });
    
    if (response.status === 401) {
        handleLogout();
        throw new Error('Unauthorized');
    }
    
    // Handle 500 errors with detailed error display
    if (response.status === 500) {
        try {
            const errorData = await response.clone().json();
            const errorMessage = errorData.error || 'Internal Server Error';
            showDetailedError(errorMessage, response.url);
        } catch (e) {
            const errorText = await response.clone().text();
            showDetailedError(errorText || 'Internal Server Error', response.url);
        }
    }
    
    return response;
}

// API response handler
async function handleApiResponse(response, successMessage) {
    if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        const errorMessage = errorData.error || `HTTP ${response.status} Error`;
        throw new Error(errorMessage);
    }
    
    const data = await response.json();
    
    if (!data.success) {
        throw new Error(data.error || 'Operation failed');
    }
    
    if (successMessage) {
        showToast(successMessage, 'success');
    }
    
    return data;
}

// Images
async function loadImages() {
    const container = document.getElementById('images-list');
    container.innerHTML = '<div class="loading">Loading images...</div>';
    
    showLoading('Fetching Docker images...');
    try {
        const response = await api('/api/images');
        const data = await response.json();
        
        if (data.success && data.data.length > 0) {
            container.innerHTML = data.data.map(image => renderImage(image)).join('');
        } else {
            container.innerHTML = '<div class="empty">No images found</div>';
        }
    } catch (error) {
        container.innerHTML = '<div class="empty">Failed to load images</div>';
        showToast('Failed to load images', 'error');
    } finally {
        hideLoading();
    }
}

function renderImage(image) {
    const tags = Array.isArray(image.tags) ? image.tags.join(', ') : image.tags;
    const size = formatBytes(image.size);
    const created = new Date(image.created).toLocaleString();
    
    return `
        <div class="card">
            <div class="card-info">
                <div class="card-title">${tags}</div>
                <div class="card-meta">
                    Size: ${size} • Created: ${created}
                </div>
            </div>
            <div class="card-actions">
                <button class="btn" onclick="exportImage('${image.id}')">Export</button>
                <button class="btn btn-danger" onclick="deleteImage('${image.id}')">Delete</button>
            </div>
        </div>
    `;
}

async function handleImageImport(e) {
    const file = e.target.files[0];
    if (!file) return;
    
    const formData = new FormData();
    formData.append('file', file);
    
    showToast('Importing image...');
    showLoading('Importing Docker image...');
    
    try {
        const response = await api('/api/images/import', {
            method: 'POST',
            body: formData
        });
        
        // Handle successful response
        if (response.ok) {
            const data = await response.json();
            if (data.success) {
                showToast('Image imported successfully', 'success');
                loadImages();
            } else {
                showToast(data.error || 'Failed to import image', 'error');
            }
        } else {
            // Handle error responses with detailed messages
            let errorMessage = 'Failed to import image';
            
            // Check for specific status codes
            if (response.status === 413) {
                errorMessage = 'Image file is too large. Maximum size is 10GB.';
            } else if (response.status === 400) {
                const data = await response.json();
                errorMessage = data.error || 'Invalid request';
            } else if (response.status === 500) {
                const data = await response.json();
                errorMessage = data.error || 'Server error during import';
            } else {
                // Try to get error message from response
                try {
                    const data = await response.json();
                    errorMessage = data.error || errorMessage;
                } catch (e) {
                    const text = await response.text();
                    if (text) errorMessage = text;
                }
            }
            
            showToast(errorMessage, 'error');
        }
    } catch (error) {
        showToast(`Failed to import image: ${error.message}`, 'error');
    } finally {
        hideLoading();
    }
    
    // Reset file input
    e.target.value = '';
}

async function exportImage(imageId) {
    showLoading('Exporting Docker image...');
    
    try {
        showToast('Exporting image...');
        
        const response = await api(`/api/images/${imageId}/export`);
        
        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`Export failed: ${response.status} ${errorText}`);
        }
        
        const blob = await response.blob();
        
        // Download file
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `image-${imageId.substring(0, 12)}.tar`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(url);
        
        showToast(`Image exported successfully (${formatBytes(blob.size)})`, 'success');
    } catch (error) {
        showToast(`Failed to export image: ${error.message}`, 'error');
    } finally {
        hideLoading();
    }
}

async function deleteImage(imageId) {
    if (!confirm('Are you sure you want to delete this image?')) return;
    
    showLoading('Deleting Docker image...');
    try {
        const response = await api(`/api/images/${imageId}`, {
            method: 'DELETE'
        });
        
        await handleApiResponse(response, 'Image deleted successfully');
        loadImages();
    } catch (error) {
        showToast(`Failed to delete image: ${error.message}`, 'error');
    } finally {
        hideLoading();
    }
}

// Containers
async function loadContainers() {
    const container = document.getElementById('containers-list');
    container.innerHTML = '<div class="loading">Loading containers...</div>';
    
    showLoading('Fetching Docker containers...');
    try {
        const response = await api('/api/containers');
        const data = await response.json();
        
        if (data.success && data.data.length > 0) {
            container.innerHTML = data.data.map(cont => renderContainer(cont)).join('');
        } else {
            container.innerHTML = '<div class="empty">No containers found</div>';
        }
    } catch (error) {
        container.innerHTML = '<div class="empty">Failed to load containers</div>';
        showToast('Failed to load containers', 'error');
    } finally {
        hideLoading();
    }
}

function renderContainer(container) {
    const name = Array.isArray(container.names) && container.names.length > 0
        ? container.names[0].replace(/^\//, '')
        : 'unnamed';
    const state = container.state.toLowerCase();
    const created = new Date(container.created).toLocaleString();
    
    const actions = state === 'running'
        ? `
            <button class="btn" onclick="viewLogs('${container.id}')">Logs</button>
            <button class="btn btn-danger" onclick="stopContainer('${container.id}')">Stop</button>
        `
        : `
            <button class="btn btn-success" onclick="startContainer('${container.id}')">Start</button>
            <button class="btn btn-danger" onclick="deleteContainer('${container.id}')">Delete</button>
        `;
    
    return `
        <div class="card">
            <div class="card-info">
                <div class="card-title">
                    ${name}
                    <span class="status status-${state}">${state}</span>
                </div>
                <div class="card-meta">
                    Image: ${container.image} • ${container.status} • Created: ${created}
                </div>
            </div>
            <div class="card-actions">
                ${actions}
            </div>
        </div>
    `;
}

async function openCreateModal() {
    const modal = document.getElementById('create-container-modal');
    modal.classList.remove('hidden');
    populateImageDropdown();
}

async function populateImageDropdown() {
    const select = document.getElementById('container-image');
    
    // Clear existing options except the first one
    select.innerHTML = '<option value="">Select an image...</option>';
    
    try {
        const response = await api('/api/images');
        const data = await response.json();
        
        if (data.success && data.data.length > 0) {
            data.data.forEach(image => {
                const tags = Array.isArray(image.tags) ? image.tags : [image.tags];
                tags.forEach(tag => {
                    if (tag !== '<none>') {
                        const option = document.createElement('option');
                        option.value = tag;
                        option.textContent = tag;
                        select.appendChild(option);
                    }
                });
            });
        } else {
            const option = document.createElement('option');
            option.value = '';
            option.textContent = 'No images available';
            option.disabled = true;
            select.appendChild(option);
        }
    } catch (error) {
        showToast('Failed to load images', 'error');
    }
}

function closeCreateModal() {
    const modal = document.getElementById('create-container-modal');
    modal.classList.add('hidden');
    document.getElementById('create-container-form').reset();
}

async function handleCreateContainer(e) {
    e.preventDefault();
    
    const image = document.getElementById('container-image').value;
    const name = document.getElementById('container-name').value;
    const envText = document.getElementById('container-env').value;
    const cmdText = document.getElementById('container-cmd').value;
    
    const env = envText.trim() ? envText.split('\n').filter(line => line.trim()) : [];
    const cmd = cmdText.trim() ? cmdText.split(' ').filter(part => part.trim()) : [];
    
    showLoading('Creating Docker container...');
    try {
        const response = await api('/api/containers', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ image, name, env, cmd })
        });
        
        const data = await response.json();
        
        if (data.success) {
            showToast('Container created successfully', 'success');
            closeCreateModal();
            loadContainers();
        } else {
            showToast(data.error || 'Failed to create container', 'error');
        }
    } catch (error) {
        showToast('Failed to create container', 'error');
    } finally {
        hideLoading();
    }
}

async function startContainer(containerId) {
    showLoading('Starting Docker container...');
    try {
        const response = await api(`/api/containers/${containerId}/start`, {
            method: 'POST'
        });
        
        await handleApiResponse(response, 'Container started');
        loadContainers();
    } catch (error) {
        showToast(`Failed to start container: ${error.message}`, 'error');
    } finally {
        hideLoading();
    }
}

async function stopContainer(containerId) {
    showLoading('Stopping Docker container...');
    try {
        const response = await api(`/api/containers/${containerId}/stop`, {
            method: 'POST'
        });
        
        await handleApiResponse(response, 'Container stopped');
        loadContainers();
    } catch (error) {
        showToast(`Failed to stop container: ${error.message}`, 'error');
    } finally {
        hideLoading();
    }
}

async function deleteContainer(containerId) {
    if (!confirm('Are you sure you want to delete this container?')) return;
    
    showLoading('Deleting Docker container...');
    try {
        const response = await api(`/api/containers/${containerId}`, {
            method: 'DELETE'
        });
        
        await handleApiResponse(response, 'Container deleted');
        loadContainers();
    } catch (error) {
        showToast(`Failed to delete container: ${error.message}`, 'error');
    } finally {
        hideLoading();
    }
}

// Logs
let logsEventSource = null;

function viewLogs(containerId) {
    const modal = document.getElementById('logs-modal');
    modal.classList.remove('hidden');
    
    const content = document.getElementById('logs-content');
    content.innerHTML = '';
    
    // Close previous connection
    if (logsEventSource) {
        logsEventSource.close();
    }
    
    // Pass token in URL since EventSource doesn't support headers
    const url = `/api/containers/${containerId}/logs?token=${encodeURIComponent(authToken)}`;
    logsEventSource = new EventSource(url);
    
    logsEventSource.onmessage = (event) => {
        const line = document.createElement('div');
        line.className = 'log-line';
        line.textContent = event.data;
        content.appendChild(line);
        content.scrollTop = content.scrollHeight;
    };
    
    logsEventSource.onerror = () => {
        const line = document.createElement('div');
        line.className = 'log-line';
        line.textContent = '--- Connection closed ---';
        content.appendChild(line);
        logsEventSource.close();
        logsEventSource = null;
    };
    
    // Setup close handler
    modal.querySelector('.modal-close').onclick = () => {
        if (logsEventSource) {
            logsEventSource.close();
            logsEventSource = null;
        }
        modal.classList.add('hidden');
    };
}

// Toast notifications
function showToast(message, type = 'info') {
    const toast = document.getElementById('toast');
    toast.textContent = message;
    toast.className = `toast ${type}`;
    
    setTimeout(() => {
        toast.classList.add('hidden');
    }, TOAST_DURATION);
}

function showDetailedError(errorMessage, url) {
    const toast = document.getElementById('toast');
    const timestamp = new Date().toLocaleTimeString();
    
    toast.innerHTML = `
        <strong>HTTP 500 Error</strong><br>
        <small>${timestamp} - ${url}</small><br>
        <div style="margin-top: 8px; padding: 8px; background: rgba(0,0,0,0.2); border-radius: 4px; font-family: monospace; font-size: 12px; max-height: 200px; overflow-y: auto; text-align: left; color: white;">
            ${escapeHtml(errorMessage)}
        </div>
    `;
    toast.className = 'toast error';
    
    // Keep error visible longer
    setTimeout(() => {
        toast.classList.add('hidden');
    }, ERROR_TOAST_DURATION);
}