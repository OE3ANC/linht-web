// File Manager Module
const FileManager = {
    currentPath: '/',
    
    // Initialize file manager
    init() {
        this.setupEventListeners();
        this.loadDirectory('/');
    },
    
    // Setup event listeners
    setupEventListeners() {
        // Parent button
        document.getElementById('fm-parent-btn').addEventListener('click', () => {
            this.goToParent();
        });
        
        // New folder button
        document.getElementById('fm-mkdir-btn').addEventListener('click', () => {
            this.showCreateFolderDialog();
        });
        
        // Upload button
        document.getElementById('fm-upload-btn').addEventListener('click', () => {
            document.getElementById('fm-upload-input').click();
        });
        
        // Upload file input
        document.getElementById('fm-upload-input').addEventListener('change', (e) => {
            if (e.target.files.length > 0) {
                this.uploadFile(e.target.files[0]);
            }
        });
        
        // Refresh button
        document.getElementById('fm-refresh-btn').addEventListener('click', () => {
            this.loadDirectory(this.currentPath);
        });
        
        // Close modal buttons
        document.querySelectorAll('.fm-modal-close').forEach(btn => {
            btn.addEventListener('click', () => {
                this.closeAllModals();
            });
        });
        
        // Create folder form
        document.getElementById('fm-mkdir-form').addEventListener('submit', (e) => {
            e.preventDefault();
            this.createFolder();
        });
    },
    
    // Load directory contents
    async loadDirectory(path) {
        this.currentPath = path;
        const tbody = document.getElementById('fm-file-list');
        tbody.innerHTML = '<tr><td colspan="3" class="loading">Loading...</td></tr>';
        
        showLoading('Loading directory...');
        try {
            const response = await api(`/api/filemanager/list?path=${encodeURIComponent(path)}`);
            const data = await response.json();
            
            if (data.success) {
                this.renderDirectory(data.data);
            } else {
                showToast(data.error || 'Failed to load directory', 'error');
                tbody.innerHTML = '<tr><td colspan="3" class="empty">Failed to load directory</td></tr>';
            }
        } catch (error) {
            showToast('Failed to load directory', 'error');
            tbody.innerHTML = '<tr><td colspan="3" class="empty">Failed to load directory</td></tr>';
        } finally {
            hideLoading();
        }
    },
    
    // Render directory contents
    renderDirectory(data) {
        this.currentPath = data.path;
        this.updateBreadcrumb(data.path);
        
        const tbody = document.getElementById('fm-file-list');
        
        if (data.items.length === 0) {
            tbody.innerHTML = '<tr><td colspan="3" class="empty">Empty directory</td></tr>';
            return;
        }
        
        // Sort: directories first, then files, alphabetically
        const items = data.items.sort((a, b) => {
            if (a.isDir && !b.isDir) return -1;
            if (!a.isDir && b.isDir) return 1;
            return a.name.localeCompare(b.name);
        });
        
        tbody.innerHTML = items.map(item => this.renderFileRow(item)).join('');
    },
    
    // Render a single file row
    renderFileRow(item) {
        const icon = item.isDir ? '📁' : '📄';
        const size = item.isDir ? '-' : formatBytes(item.size);
        const modified = new Date(item.modified).toLocaleString();
        const nameClass = item.isDir ? 'fm-folder-name' : 'fm-file-name';
        const onclick = item.isDir 
            ? `FileManager.loadDirectory('${escapeHtml(item.path)}')`
            : `FileManager.downloadFile('${escapeHtml(item.path)}')`;
        
        return `
            <tr>
                <td>
                    <span class="${nameClass}" onclick="${onclick}">
                        ${icon} ${escapeHtml(item.name)}
                    </span>
                </td>
                <td>${size}</td>
                <td>${modified}</td>
                <td>
                    <button class="btn btn-sm btn-danger" onclick="FileManager.deleteItem('${escapeHtml(item.path)}', '${escapeHtml(item.name)}')">
                        Delete
                    </button>
                </td>
            </tr>
        `;
    },
    
    // Update breadcrumb navigation
    updateBreadcrumb(path) {
        const breadcrumb = document.getElementById('fm-breadcrumb');
        
        if (path === '/') {
            breadcrumb.innerHTML = '<span class="fm-breadcrumb-item" onclick="FileManager.loadDirectory(\'/\')">/ (root)</span>';
            return;
        }
        
        const parts = path.split('/').filter(p => p);
        let html = '<span class="fm-breadcrumb-item" onclick="FileManager.loadDirectory(\'/\')">/ </span>';
        
        let currentPath = '';
        parts.forEach((part, index) => {
            currentPath += '/' + part;
            const isLast = index === parts.length - 1;
            const encodedPath = escapeHtml(currentPath);
            
            if (isLast) {
                html += `<span class="fm-breadcrumb-item fm-breadcrumb-current">${escapeHtml(part)}</span>`;
            } else {
                html += `<span class="fm-breadcrumb-item" onclick="FileManager.loadDirectory('${encodedPath}')">${escapeHtml(part)} / </span>`;
            }
        });
        
        breadcrumb.innerHTML = html;
    },
    
    // Go to parent directory
    goToParent() {
        if (this.currentPath === '/') {
            showToast('Already at root directory', 'info');
            return;
        }
        
        const parent = this.currentPath.substring(0, this.currentPath.lastIndexOf('/')) || '/';
        this.loadDirectory(parent);
    },
    
    // Upload file
    async uploadFile(file) {
        showLoading('Uploading file...');
        
        const formData = new FormData();
        formData.append('file', file);
        formData.append('path', this.currentPath);
        
        try {
            const response = await api('/api/filemanager/upload', {
                method: 'POST',
                body: formData
            });
            
            const data = await response.json();
            
            if (data.success) {
                showToast('File uploaded successfully', 'success');
                this.loadDirectory(this.currentPath);
            } else {
                showToast(data.error || 'Failed to upload file', 'error');
            }
        } catch (error) {
            showToast('Failed to upload file', 'error');
        } finally {
            hideLoading();
            // Reset file input
            document.getElementById('fm-upload-input').value = '';
        }
    },
    
    // Download file
    async downloadFile(path) {
        showLoading('Downloading file...');
        
        try {
            const response = await api(`/api/filemanager/download?path=${encodeURIComponent(path)}`);
            
            if (response.ok) {
                const blob = await response.blob();
                const filename = path.split('/').pop();
                
                // Create download link
                const url = window.URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                a.download = filename;
                document.body.appendChild(a);
                a.click();
                document.body.removeChild(a);
                window.URL.revokeObjectURL(url);
                
                showToast('File downloaded', 'success');
            } else {
                const data = await response.json();
                showToast(data.error || 'Failed to download file', 'error');
            }
        } catch (error) {
            showToast('Failed to download file', 'error');
        } finally {
            hideLoading();
        }
    },
    
    // Delete item
    async deleteItem(path, name) {
        if (!confirm(`Are you sure you want to delete "${name}"?`)) {
            return;
        }
        
        showLoading('Deleting...');
        
        try {
            const response = await api('/api/filemanager/delete', {
                method: 'DELETE',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path })
            });
            
            const data = await response.json();
            
            if (data.success) {
                showToast('Deleted successfully', 'success');
                this.loadDirectory(this.currentPath);
            } else {
                showToast(data.error || 'Failed to delete', 'error');
            }
        } catch (error) {
            showToast('Failed to delete', 'error');
        } finally {
            hideLoading();
        }
    },
    
    // Show create folder dialog
    showCreateFolderDialog() {
        document.getElementById('fm-mkdir-modal').classList.remove('hidden');
        document.getElementById('fm-folder-name').value = '';
        document.getElementById('fm-folder-name').focus();
    },
    
    // Create folder
    async createFolder() {
        const name = document.getElementById('fm-folder-name').value.trim();
        
        if (!name) {
            showToast('Folder name is required', 'error');
            return;
        }
        
        // Basic validation
        if (name.includes('/') || name.includes('\\')) {
            showToast('Folder name cannot contain / or \\', 'error');
            return;
        }
        
        const path = this.currentPath === '/' 
            ? `/${name}` 
            : `${this.currentPath}/${name}`;
        
        this.closeAllModals();
        showLoading('Creating folder...');
        
        try {
            const response = await api('/api/filemanager/mkdir', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path })
            });
            
            const data = await response.json();
            
            if (data.success) {
                showToast('Folder created successfully', 'success');
                this.loadDirectory(this.currentPath);
            } else {
                showToast(data.error || 'Failed to create folder', 'error');
            }
        } catch (error) {
            showToast('Failed to create folder', 'error');
        } finally {
            hideLoading();
        }
    },
    
    // Close all modals
    closeAllModals() {
        document.querySelectorAll('.modal').forEach(modal => {
            modal.classList.add('hidden');
        });
    }
};