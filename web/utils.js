// Shared utility functions

// Escape HTML to prevent XSS
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML.replace(/'/g, '&#39;');
}

// Format bytes to human-readable size
function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
}

// Async operation wrapper with loading indicator and error handling
// Usage: await withLoading('Loading...', async () => { ... })
async function withLoading(message, asyncFn) {
    showLoading(message);
    try {
        return await asyncFn();
    } finally {
        hideLoading();
    }
}

// Async API call wrapper with loading, error handling, and toast notifications
// Usage: await apiCall('Loading...', '/api/endpoint', { method: 'POST' }, 'Success!', onSuccess)
async function apiCall(loadingMsg, url, options, successMsg, onSuccess) {
    return withLoading(loadingMsg, async () => {
        const response = await fetch(url, options);
        const data = await response.json();
        
        if (data.success) {
            if (successMsg) showToast(successMsg, 'success');
            if (onSuccess) await onSuccess(data);
            return data;
        } else {
            showToast(data.message || data.error || 'Operation failed', 'error');
            return null;
        }
    });
}