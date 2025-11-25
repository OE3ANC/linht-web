// CPS
// Dynamic settings editor for /usr/share/linht/settings.yaml

const CPS = {
    settings: null,
    initialized: false,

    init() {
        if (!this.initialized) {
            this.setupEventListeners();
            this.initialized = true;
        }
    },

    setupEventListeners() {
        document.getElementById('cps-load-btn').addEventListener('click', () => this.loadSettings());
        document.getElementById('cps-save-btn').addEventListener('click', () => this.saveSettings());
    },

    async loadSettings() {
        const container = document.getElementById('cps-form-container');
        container.innerHTML = '<div class="loading">Loading settings...</div>';

        showLoading('Loading settings...');
        try {
            const response = await api('/api/cps/load');
            const data = await response.json();

            if (data.success) {
                this.settings = data.data;
                this.renderForm();
                showToast('Settings loaded successfully', 'success');
            } else {
                container.innerHTML = `<div class="empty">Failed to load settings: ${data.error}</div>`;
                showToast(data.error || 'Failed to load settings', 'error');
            }
        } catch (error) {
            container.innerHTML = '<div class="empty">Failed to load settings</div>';
            showToast('Failed to load settings', 'error');
        } finally {
            hideLoading();
        }
    },

    async saveSettings() {
        if (!this.settings) {
            showToast('No settings loaded', 'error');
            return;
        }

        // Collect values from form
        this.collectFormValues();

        showLoading('Saving settings...');
        try {
            const response = await api('/api/cps/save', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(this.settings)
            });

            const data = await response.json();

            if (data.success) {
                showToast('Settings saved successfully', 'success');
            } else {
                showToast(data.error || 'Failed to save settings', 'error');
            }
        } catch (error) {
            showToast('Failed to save settings', 'error');
        } finally {
            hideLoading();
        }
    },

    renderForm() {
        const container = document.getElementById('cps-form-container');
        container.innerHTML = '';

        if (!this.settings) {
            container.innerHTML = '<div class="empty">No settings loaded</div>';
            return;
        }

        // Render each top-level section
        for (const [key, value] of Object.entries(this.settings)) {
            const section = this.createSection(key, value, key);
            container.appendChild(section);
        }
    },

    createSection(title, data, path) {
        const section = document.createElement('div');
        section.className = 'cps-section';

        const header = document.createElement('div');
        header.className = 'cps-section-header';
        header.innerHTML = `
            <span class="cps-section-toggle">▼</span>
            <h3 class="cps-section-title">&gt; ${this.formatTitle(title)}</h3>
        `;
        header.addEventListener('click', () => this.toggleSection(section));

        const content = document.createElement('div');
        content.className = 'cps-section-content';

        if (typeof data === 'object' && data !== null && !Array.isArray(data)) {
            // Render nested object
            for (const [key, value] of Object.entries(data)) {
                const fieldPath = `${path}.${key}`;
                
                if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
                    // Nested section
                    const nestedSection = this.createSection(key, value, fieldPath);
                    nestedSection.classList.add('cps-nested-section');
                    content.appendChild(nestedSection);
                } else {
                    // Field
                    const field = this.createField(key, value, fieldPath);
                    content.appendChild(field);
                }
            }
        } else {
            // Single value at top level (unlikely but handle it)
            const field = this.createField(title, data, path);
            content.appendChild(field);
        }

        section.appendChild(header);
        section.appendChild(content);

        return section;
    },

    createField(key, value, path) {
        const field = document.createElement('div');
        field.className = 'cps-field';

        const label = document.createElement('label');
        label.className = 'cps-field-label';
        label.textContent = this.formatTitle(key) + ':';

        const inputContainer = document.createElement('div');
        inputContainer.className = 'cps-field-input';

        let input;

        if (typeof value === 'boolean') {
            // Checkbox for boolean
            input = document.createElement('input');
            input.type = 'checkbox';
            input.checked = value;
            input.className = 'cps-checkbox';
        } else if (typeof value === 'number') {
            // Number input
            input = document.createElement('input');
            input.type = 'number';
            input.value = value;
            input.step = Number.isInteger(value) ? '1' : '0.001';
        } else if (value === null) {
            // Text input for null values
            input = document.createElement('input');
            input.type = 'text';
            input.value = '';
            input.placeholder = 'null';
        } else {
            // Text input for strings
            input = document.createElement('input');
            input.type = 'text';
            input.value = value;
        }

        input.dataset.path = path;
        input.dataset.type = value === null ? 'null' : typeof value;

        inputContainer.appendChild(input);

        field.appendChild(label);
        field.appendChild(inputContainer);

        return field;
    },

    toggleSection(section) {
        const content = section.querySelector('.cps-section-content');
        const toggle = section.querySelector('.cps-section-toggle');
        
        if (content.classList.contains('collapsed')) {
            content.classList.remove('collapsed');
            toggle.textContent = '▼';
        } else {
            content.classList.add('collapsed');
            toggle.textContent = '▶';
        }
    },

    formatTitle(key) {
        // Convert snake_case or camelCase to Title Case
        return key
            .replace(/_/g, ' ')
            .replace(/([A-Z])/g, ' $1')
            .replace(/^./, str => str.toUpperCase())
            .trim();
    },

    collectFormValues() {
        const inputs = document.querySelectorAll('#cps-form-container input');
        
        inputs.forEach(input => {
            const path = input.dataset.path;
            const type = input.dataset.type;
            
            let value;
            if (input.type === 'checkbox') {
                value = input.checked;
            } else if (type === 'number') {
                value = input.value === '' ? 0 : parseFloat(input.value);
            } else if (type === 'null' && input.value === '') {
                value = null;
            } else {
                value = input.value;
            }

            this.setValueByPath(path, value);
        });
    },

    setValueByPath(path, value) {
        const keys = path.split('.');
        let obj = this.settings;
        
        for (let i = 0; i < keys.length - 1; i++) {
            obj = obj[keys[i]];
        }
        
        obj[keys[keys.length - 1]] = value;
    }
};