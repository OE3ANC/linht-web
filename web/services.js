// Services Module
// Manages systemd services with the "linht-" prefix

const Services = {
    services: [],
    initialized: false,
    logsEventSource: null,

    init() {
        if (!this.initialized) {
            this.setupEventListeners();
            this.initialized = true;
        }
        // Don't auto-load services on tab change - user must click refresh button
    },

    setupEventListeners() {
        document.getElementById('services-refresh-btn').addEventListener('click', () => this.loadServices());
    },

    async loadServices() {
        const container = document.getElementById('services-list');
        container.innerHTML = '<tr><td colspan="5" class="loading">Loading services...</td></tr>';

        showLoading('Loading services...');
        try {
            const response = await api('/api/services');
            const data = await response.json();

            if (data.success) {
                this.services = data.data || [];
                this.renderServices();
                if (this.services.length === 0) {
                    showToast('No linht-* services found', 'info');
                }
            } else {
                container.innerHTML = `<tr><td colspan="5" class="empty">Failed to load services: ${data.error}</td></tr>`;
                showToast(data.error || 'Failed to load services', 'error');
            }
        } catch (error) {
            container.innerHTML = '<tr><td colspan="5" class="empty">Failed to load services</td></tr>';
            showToast('Failed to load services', 'error');
        } finally {
            hideLoading();
        }
    },

    renderServices() {
        const container = document.getElementById('services-list');

        if (!this.services || this.services.length === 0) {
            container.innerHTML = '<tr><td colspan="5" class="empty">No linht-* services found</td></tr>';
            return;
        }

        container.innerHTML = this.services.map(service => this.renderServiceRow(service)).join('');
    },

    renderServiceRow(service) {
        const statusClass = service.is_active ? 'status-running' : 'status-exited';
        const statusText = service.active_state || 'unknown';
        const enabledClass = service.is_enabled ? 'status-running' : 'status-exited';
        const enabledText = service.is_enabled ? 'enabled' : 'disabled';

        const startStopBtn = service.is_active
            ? `<button class="btn btn-danger btn-sm" onclick="Services.stopService('${service.name}')">Stop</button>`
            : `<button class="btn btn-success btn-sm" onclick="Services.startService('${service.name}')">Start</button>`;

        const enableDisableBtn = service.is_enabled
            ? `<button class="btn btn-sm" onclick="Services.disableService('${service.name}')">Disable</button>`
            : `<button class="btn btn-sm" onclick="Services.enableService('${service.name}')">Enable</button>`;

        return `
            <tr>
                <td class="service-name">${service.name}</td>
                <td class="service-description">${service.description || '-'}</td>
                <td><span class="status ${statusClass}">${statusText}</span></td>
                <td><span class="status ${enabledClass}">${enabledText}</span></td>
                <td class="service-actions">
                    ${startStopBtn}
                    ${enableDisableBtn}
                    <button class="btn btn-sm" onclick="Services.viewLogs('${service.name}')">Logs</button>
                </td>
            </tr>
        `;
    },

    async startService(name) {
        await this.serviceAction(name, 'start', 'Starting');
    },

    async stopService(name) {
        await this.serviceAction(name, 'stop', 'Stopping');
    },

    async enableService(name) {
        await this.serviceAction(name, 'enable', 'Enabling');
    },

    async disableService(name) {
        await this.serviceAction(name, 'disable', 'Disabling');
    },

    async serviceAction(name, action, actionText) {
        showLoading(`${actionText} service ${name}...`);
        try {
            const response = await api(`/api/services/${name}/${action}`, {
                method: 'POST'
            });
            const data = await response.json();

            if (data.success) {
                showToast(data.message || `Service ${action}ed successfully`, 'success');
                await this.loadServices();
            } else {
                showToast(data.error || `Failed to ${action} service`, 'error');
            }
        } catch (error) {
            showToast(`Failed to ${action} service: ${error.message}`, 'error');
        } finally {
            hideLoading();
        }
    },

    viewLogs(name) {
        const modal = document.getElementById('services-logs-modal');
        const title = document.getElementById('services-logs-title');
        const content = document.getElementById('services-logs-content');

        title.textContent = `Logs: ${name}`;
        modal.classList.remove('hidden');
        content.innerHTML = '<div class="loading">Connecting to log stream...</div>';

        // Close any existing connection
        if (this.logsEventSource) {
            this.logsEventSource.close();
            this.logsEventSource = null;
        }

        // Start SSE connection for logs
        this.logsEventSource = new EventSource(`/api/services/${name}/logs`);

        this.logsEventSource.onopen = () => {
            content.innerHTML = '';
        };

        this.logsEventSource.onmessage = (event) => {
            const line = document.createElement('div');
            line.className = 'log-line';
            line.textContent = event.data;
            content.appendChild(line);
            content.scrollTop = content.scrollHeight;
        };

        this.logsEventSource.onerror = () => {
            const line = document.createElement('div');
            line.className = 'log-line log-error';
            line.textContent = '--- Connection closed ---';
            content.appendChild(line);
            this.logsEventSource.close();
            this.logsEventSource = null;
        };
    },

    closeLogsModal() {
        const modal = document.getElementById('services-logs-modal');
        modal.classList.add('hidden');

        if (this.logsEventSource) {
            this.logsEventSource.close();
            this.logsEventSource = null;
        }
    }
};

// Close modal when clicking the close button
document.addEventListener('DOMContentLoaded', () => {
    const closeBtn = document.querySelector('#services-logs-modal .modal-close');
    if (closeBtn) {
        closeBtn.addEventListener('click', () => Services.closeLogsModal());
    }
});