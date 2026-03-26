(function() {
    const API_STATE_URL = '/api/state';
    const SSE_STREAM_URL = '/stream';
    const POLL_INTERVAL = 5000;

    let serverStartTime = null;
    let eventSource = null;
    let lastUpdateTime = null;

    function init() {
        startPolling();
        connectSSE();
        updateLastUpdateTime();
    }

    function startPolling() {
        fetchState();
        setInterval(fetchState, POLL_INTERVAL);
    }

    function fetchState() {
        fetch(API_STATE_URL)
            .then(response => {
                if (!response.ok) {
                    throw new Error('Network response was not ok');
                }
                return response.json();
            })
            .then(data => {
                updateDashboard(data);
                updateLastUpdateTime();
            })
            .catch(error => {
                console.error('Failed to fetch state:', error);
            });
    }

    function connectSSE() {
        if (eventSource) {
            eventSource.close();
        }

        eventSource = new EventSource(SSE_STREAM_URL);

        eventSource.onopen = function() {
            console.log('SSE connection established');
        };

        eventSource.onmessage = function(event) {
            try {
                const data = JSON.parse(event.data);
                handleSSEEvent(data);
            } catch (error) {
                console.error('Failed to parse SSE event:', error);
            }
        };

        eventSource.onerror = function(error) {
            console.error('SSE connection error:', error);
            if (eventSource.readyState === EventSource.CLOSED) {
                setTimeout(connectSSE, 3000);
            }
        };
    }

    function handleSSEEvent(event) {
        const eventType = event.type;
        const project = event.project;
        const agentId = event.agent_id;
        const payload = event.payload;

        switch (eventType) {
            case 'agent_online':
            case 'agent_offline':
                fetchState();
                break;
            case 'task_status_changed':
            case 'task_completed':
            case 'task_blocked':
            case 'project_registered':
                fetchState();
                prependEvent(event);
                break;
        }
    }

    function updateDashboard(data) {
        if (data.start_time) {
            serverStartTime = new Date(data.start_time);
            updateUptime();
        }

        updateAgents(data.agents || []);
        updateProjects(data.projects || [], data.tasks || {});
    }

    function updateAgents(agents) {
        const agentList = document.getElementById('agent-list');
        const onlineCount = document.getElementById('agent-count');
        const offlineCount = document.getElementById('agent-offline-count');

        const onlineAgents = agents.filter(a => a.online);
        const offlineAgents = agents.filter(a => !a.online);

        onlineCount.textContent = onlineAgents.length;
        offlineCount.textContent = offlineAgents.length;

        if (agents.length === 0) {
            agentList.innerHTML = '<div class="empty-state">No agents registered</div>';
            return;
        }

        agentList.innerHTML = '';

        const sortedAgents = [...agents].sort((a, b) => {
            if (a.online !== b.online) {
                return b.online ? 1 : -1;
            }
            return new Date(b.last_heartbeat) - new Date(a.last_heartbeat);
        });

        sortedAgents.forEach(agent => {
            const agentEl = createAgentElement(agent);
            agentList.appendChild(agentEl);
        });
    }

    function createAgentElement(agent) {
        const div = document.createElement('div');
        div.className = 'agent-item';

        const statusDot = document.createElement('span');
        statusDot.className = `agent-status ${agent.online ? 'online' : 'offline'}`;
        div.appendChild(statusDot);

        const idSpan = document.createElement('span');
        idSpan.className = 'agent-id';
        idSpan.textContent = agent.id;
        div.appendChild(idSpan);

        const roleSpan = document.createElement('span');
        roleSpan.className = 'agent-role';
        roleSpan.textContent = agent.role;
        div.appendChild(roleSpan);

        const uptimeSpan = document.createElement('span');
        uptimeSpan.className = 'agent-uptime';
        if (agent.last_heartbeat) {
            const lastSeen = formatTimeAgo(new Date(agent.last_heartbeat));
            uptimeSpan.textContent = agent.online ? `seen ${lastSeen}` : `offline ${lastSeen}`;
        }
        div.appendChild(uptimeSpan);

        if (agent.capabilities && agent.capabilities.length > 0) {
            const capsDiv = document.createElement('div');
            capsDiv.className = 'agent-capabilities';
            agent.capabilities.forEach(cap => {
                const capSpan = document.createElement('span');
                capSpan.className = 'capability';
                capSpan.textContent = cap;
                capsDiv.appendChild(capSpan);
            });
            div.appendChild(capsDiv);
        }

        return div;
    }

    function updateProjects(projects, tasksByProject) {
        const projectList = document.getElementById('project-list');

        if (projects.length === 0) {
            projectList.innerHTML = '<div class="empty-state">No projects registered</div>';
            return;
        }

        projectList.innerHTML = '';

        projects.forEach(project => {
            const tasks = tasksByProject[project.name] || [];
            const projectEl = createProjectElement(project, tasks);
            projectList.appendChild(projectEl);
        });
    }

    function createProjectElement(project, tasks) {
        const div = document.createElement('div');
        div.className = 'project-item';

        const nameSpan = document.createElement('span');
        nameSpan.className = 'project-name';
        nameSpan.textContent = project.name;
        div.appendChild(nameSpan);

        const statusSpan = document.createElement('span');
        statusSpan.className = 'project-status';
        
        const currentTask = tasks.find(t => t.status === 'executing') || 
                           tasks.find(t => t.status === 'pending');
        
        if (currentTask) {
            statusSpan.textContent = currentTask.status;
            statusSpan.classList.add(currentTask.status);
        } else {
            statusSpan.textContent = 'idle';
            statusSpan.classList.add('pending');
        }
        div.appendChild(statusSpan);

        const taskSpan = document.createElement('span');
        taskSpan.className = 'project-task';
        
        const completedCount = tasks.filter(t => t.status === 'completed' || t.status === 'archived').length;
        const totalCount = tasks.length;
        
        if (currentTask) {
            taskSpan.textContent = `task ${currentTask.num}/${totalCount}`;
        } else {
            taskSpan.textContent = `${completedCount}/${totalCount} completed`;
        }
        div.appendChild(taskSpan);

        const executorSpan = document.createElement('span');
        executorSpan.className = 'project-executor';
        executorSpan.textContent = project.executor || '-';
        div.appendChild(executorSpan);

        return div;
    }

    function prependEvent(event) {
        const eventList = document.getElementById('event-list');
        const eventEl = createEventElement(event);
        
        if (eventList.firstChild) {
            eventList.insertBefore(eventEl, eventList.firstChild);
        } else {
            eventList.appendChild(eventEl);
        }

        const maxEvents = 50;
        while (eventList.children.length > maxEvents) {
            eventList.removeChild(eventList.lastChild);
        }
    }

    function createEventElement(event) {
        const div = document.createElement('div');
        div.className = 'event-item';

        const timeSpan = document.createElement('span');
        timeSpan.className = 'event-time';
        timeSpan.textContent = formatTime(new Date(event.timestamp));
        div.appendChild(timeSpan);

        const projectSpan = document.createElement('span');
        projectSpan.className = 'event-project';
        projectSpan.textContent = event.project || event.agent_id || '';
        div.appendChild(projectSpan);

        const taskSpan = document.createElement('span');
        taskSpan.className = 'event-task';
        taskSpan.innerHTML = formatEventDescription(event);
        div.appendChild(taskSpan);

        return div;
    }

    function formatEventDescription(event) {
        const type = event.type;
        const payload = event.payload;

        switch (type) {
            case 'task_completed':
                return `<span class="event-icon">✅</span> ${payload.num || ''} commit ${payload.commit || 'unknown'}`;
            case 'task_status_changed':
                const statusIcons = {
                    'executing': '🔵',
                    'pending': '🟢',
                    'blocked': '🔴',
                    'completed': '✅',
                    'archived': '📦'
                };
                const icon = statusIcons[payload.status] || '📝';
                return `${payload.num || ''} ${icon} ${payload.status}`;
            case 'agent_online':
                return `<span class="event-icon">🟢</span> connected`;
            case 'agent_offline':
                return `<span class="event-icon">🔴</span> disconnected`;
            case 'project_registered':
                return `<span class="event-icon">📁</span> registered`;
            default:
                return `${type}`;
        }
    }

    function updateUptime() {
        if (!serverStartTime) return;

        const now = new Date();
        const diff = now - serverStartTime;
        
        const hours = Math.floor(diff / (1000 * 60 * 60));
        const minutes = Math.floor((diff % (1000 * 60 * 60)) / (1000 * 60));
        
        const uptimeEl = document.getElementById('uptime');
        uptimeEl.textContent = `uptime: ${hours}h ${minutes}m`;
    }

    function updateLastUpdateTime() {
        lastUpdateTime = new Date();
        const el = document.getElementById('last-update');
        el.textContent = formatTime(lastUpdateTime);
    }

    function formatTime(date) {
        return date.toLocaleTimeString('en-US', { 
            hour: '2-digit', 
            minute: '2-digit',
            hour12: false 
        });
    }

    function formatTimeAgo(date) {
        const seconds = Math.floor((new Date() - date) / 1000);
        
        if (seconds < 60) return 'just now';
        if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
        if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
        return `${Math.floor(seconds / 86400)}d ago`;
    }

    setInterval(updateUptime, 60000);

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
