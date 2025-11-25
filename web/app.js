// Toast duration constants
const TOAST_DURATION = 3000;           // milliseconds
const ERROR_TOAST_DURATION = 10000;    // milliseconds

// DOM Elements
const app = document.getElementById('app');

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
    loadInitialData();
});

function setupEventListeners() {
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

function loadInitialData() {
    loadImages();
    loadContainers();
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
        files: () => FileManager.init(),
        cps: () => CPS.init(),
        services: () => Services.init()
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
            ...options.headers
        }
    });
    
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

// Images
async function loadImages() {
    const container = document.getElementById('images-list');
    container.innerHTML = '<div class="loading">Loading images...</div>';
    
    await withLoading('Fetching Docker images...', async () => {
        try {
            const response = await api('/api/images');
            const data = await response.json();
            
            if (data.success && data.data.length > 0) {
                container.innerHTML = data.data.map(renderImage).join('');
            } else {
                container.innerHTML = '<div class="empty">No images found</div>';
            }
        } catch (error) {
            container.innerHTML = '<div class="empty">Failed to load images</div>';
            showToast('Failed to load images', 'error');
        }
    });
}

function renderImage(image) {
    const tags = Array.isArray(image.tags) ? image.tags.join(', ') : image.tags;
    const size = formatBytes(image.size);
    const created = new Date(image.created).toLocaleString();
    
    return `
        <div class="card">
            <div class="card-info">
                <div class="card-title">${tags}</div>
                <div class="card-meta">Size: ${size} • Created: ${created}</div>
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
    await withLoading('Importing Docker image...', async () => {
        try {
            const response = await api('/api/images/import', { method: 'POST', body: formData });
            
            if (response.ok) {
                const data = await response.json();
                if (data.success) {
                    showToast('Image imported successfully', 'success');
                    loadImages();
                } else {
                    showToast(data.error || 'Failed to import image', 'error');
                }
            } else {
                let errorMessage = 'Failed to import image';
                if (response.status === 413) errorMessage = 'Image file is too large. Maximum size is 10GB.';
                else {
                    try {
                        const data = await response.json();
                        errorMessage = data.error || errorMessage;
                    } catch (e) { /* ignore */ }
                }
                showToast(errorMessage, 'error');
            }
        } catch (error) {
            showToast(`Failed to import image: ${error.message}`, 'error');
        }
    });
    
    e.target.value = '';
}

async function exportImage(imageId) {
    await withLoading('Exporting Docker image...', async () => {
        try {
            showToast('Exporting image...');
            const response = await api(`/api/images/${imageId}/export`);
            
            if (!response.ok) throw new Error(`Export failed: ${response.status}`);
            
            const blob = await response.blob();
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
        }
    });
}

async function deleteImage(imageId) {
    if (!confirm('Are you sure you want to delete this image?')) return;
    
    await apiCall('Deleting Docker image...', `/api/images/${imageId}`, { method: 'DELETE' },
        'Image deleted successfully', loadImages);
}

// Containers
async function loadContainers() {
    const container = document.getElementById('containers-list');
    container.innerHTML = '<div class="loading">Loading containers...</div>';
    
    await withLoading('Fetching Docker containers...', async () => {
        try {
            const response = await api('/api/containers');
            const data = await response.json();
            
            if (data.success && data.data.length > 0) {
                container.innerHTML = data.data.map(renderContainer).join('');
            } else {
                container.innerHTML = '<div class="empty">No containers found</div>';
            }
        } catch (error) {
            container.innerHTML = '<div class="empty">Failed to load containers</div>';
            showToast('Failed to load containers', 'error');
        }
    });
}

function renderContainer(container) {
    const name = Array.isArray(container.names) && container.names.length > 0
        ? container.names[0].replace(/^\//, '') : 'unnamed';
    const state = container.state.toLowerCase();
    const created = new Date(container.created).toLocaleString();
    
    const actions = state === 'running'
        ? `<button class="btn" onclick="viewLogs('${container.id}')">Logs</button>
           <button class="btn btn-danger" onclick="stopContainer('${container.id}')">Stop</button>`
        : `<button class="btn btn-success" onclick="startContainer('${container.id}')">Start</button>
           <button class="btn btn-danger" onclick="deleteContainer('${container.id}')">Delete</button>`;
    
    return `
        <div class="card">
            <div class="card-info">
                <div class="card-title">${name} <span class="status status-${state}">${state}</span></div>
                <div class="card-meta">Image: ${container.image} • ${container.status} • Created: ${created}</div>
            </div>
            <div class="card-actions">${actions}</div>
        </div>
    `;
}

async function openCreateModal() {
    document.getElementById('create-container-modal').classList.remove('hidden');
    await populateImageDropdown();
}

async function populateImageDropdown() {
    const select = document.getElementById('container-image');
    select.innerHTML = '<option value="">Select an image...</option>';
    
    try {
        const response = await api('/api/images');
        const data = await response.json();
        
        if (data.success && data.data.length > 0) {
            data.data.forEach(image => {
                const tags = Array.isArray(image.tags) ? image.tags : [image.tags];
                tags.filter(tag => tag !== '<none>').forEach(tag => {
                    const option = document.createElement('option');
                    option.value = tag;
                    option.textContent = tag;
                    select.appendChild(option);
                });
            });
        }
    } catch (error) {
        showToast('Failed to load images', 'error');
    }
}

function closeCreateModal() {
    document.getElementById('create-container-modal').classList.add('hidden');
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
    
    await apiCall('Creating Docker container...', '/api/containers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ image, name, env, cmd })
    }, 'Container created successfully', () => {
        closeCreateModal();
        loadContainers();
    });
}

async function startContainer(containerId) {
    await apiCall('Starting Docker container...', `/api/containers/${containerId}/start`,
        { method: 'POST' }, 'Container started', loadContainers);
}

async function stopContainer(containerId) {
    await apiCall('Stopping Docker container...', `/api/containers/${containerId}/stop`,
        { method: 'POST' }, 'Container stopped', loadContainers);
}

async function deleteContainer(containerId) {
    if (!confirm('Are you sure you want to delete this container?')) return;
    
    await apiCall('Deleting Docker container...', `/api/containers/${containerId}`,
        { method: 'DELETE' }, 'Container deleted', loadContainers);
}

// Logs
let logsEventSource = null;

function viewLogs(containerId) {
    const modal = document.getElementById('logs-modal');
    modal.classList.remove('hidden');
    
    const content = document.getElementById('logs-content');
    content.innerHTML = '';
    
    if (logsEventSource) logsEventSource.close();
    
    logsEventSource = new EventSource(`/api/containers/${containerId}/logs`);
    
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
    
    setTimeout(() => {
        toast.classList.add('hidden');
    }, ERROR_TOAST_DURATION);
}