// Claude Watch Status - Web UI

class ClaudeWatchStatus {
    constructor() {
        this.projects = new Map();
        this.eventSource = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 10;
        this.reconnectDelay = 1000;

        this.init();
    }

    init() {
        this.connectSSE();
    }

    connectSSE() {
        this.updateConnectionStatus('connecting');

        this.eventSource = new EventSource('/api/status/stream');

        this.eventSource.addEventListener('init', (event) => {
            const data = JSON.parse(event.data);
            this.handleInit(data);
            this.reconnectAttempts = 0;
            this.updateConnectionStatus('connected');
        });

        this.eventSource.addEventListener('update', (event) => {
            const project = JSON.parse(event.data);
            this.handleUpdate(project);
        });

        this.eventSource.onerror = () => {
            this.eventSource.close();
            this.updateConnectionStatus('disconnected');
            this.scheduleReconnect();
        };
    }

    scheduleReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('Max reconnection attempts reached');
            return;
        }

        const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts);
        this.reconnectAttempts++;

        setTimeout(() => {
            this.connectSSE();
        }, delay);
    }

    updateConnectionStatus(status) {
        const statusEl = document.getElementById('connectionStatus');
        const textEl = statusEl.querySelector('.status-text');

        statusEl.className = 'connection-status ' + status;

        switch (status) {
            case 'connecting':
                textEl.textContent = 'Connecting...';
                break;
            case 'connected':
                textEl.textContent = 'Connected';
                break;
            case 'disconnected':
                textEl.textContent = 'Disconnected - Reconnecting...';
                break;
        }
    }

    handleInit(data) {
        this.projects.clear();

        if (data.projects && data.projects.length > 0) {
            data.projects.forEach(project => {
                this.projects.set(project.name, project);
            });
        }

        this.render();
    }

    handleUpdate(project) {
        this.projects.set(project.name, project);
        this.render();
    }

    render() {
        const container = document.getElementById('projects');

        if (this.projects.size === 0) {
            container.innerHTML = `
                <div class="empty-state">
                    <p>No active projects</p>
                    <p class="hint">Start a Claude Code session to see status updates</p>
                </div>
            `;
            return;
        }

        // Sort projects by update time (most recent first)
        const sortedProjects = Array.from(this.projects.values())
            .sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at));

        container.innerHTML = sortedProjects
            .map(project => this.renderProjectCard(project))
            .join('');
    }

    renderProjectCard(project) {
        const time = this.formatTime(project.updated_at);
        const stateClass = this.getStateClass(project.state);
        const isProcessing = this.isProcessingState(project.state);

        return `
            <div class="project-card ${isProcessing ? 'processing' : ''} ${stateClass}" data-state="${stateClass}">
                <div class="project-icon">${project.icon}</div>
                <div class="project-info">
                    <div class="project-name">${this.escapeHtml(project.name)}</div>
                    <div class="project-state">${this.escapeHtml(project.state)}</div>
                </div>
                <div class="project-meta">
                    <div class="project-time">${time}</div>
                    <div class="project-source ${project.source}">${project.source}</div>
                </div>
            </div>
        `;
    }

    formatTime(timestamp) {
        const date = new Date(timestamp);
        return date.toLocaleTimeString('en-US', {
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit',
            hour12: false
        });
    }

    getStateClass(state) {
        if (state.includes('completed')) return 'completed';
        if (state.includes('waiting') || state.includes('approval')) return 'waiting';
        if (state.includes('error') || state.includes('max tokens')) return 'error';
        return '';
    }

    isProcessingState(state) {
        return state.includes('processing') ||
               state.includes('thinking') ||
               state.includes('running') ||
               state.includes('calling');
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Initialize when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    new ClaudeWatchStatus();
});
