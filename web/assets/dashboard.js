// Store previous data to detect changes
const previousData = {
    inputs: {},
    outputs: {},
    stats: {},
};

// WebSocket connection
let ws = null;
let reconnectTimeout = null;
let reconnectDelay = 1000;
const maxReconnectDelay = 30000;

// HTML Generation Helpers
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function createLabeledField(
    label,
    value,
    labelClass = 'text-muted-dark',
    valueClass = 'font-semibold',
) {
    return `<div class="mb-2"><span class="${labelClass}">${label}:</span> <span class="${valueClass}">${escapeHtml(value)}</span></div>`;
}

function createBadge(text, classes) {
    return `<span class="badge ${classes}">${escapeHtml(text)}</span>`;
}

function createMetadataCard(name, type, headerClass, content, hasChanged) {
    return `<div class="card" data-changed="${hasChanged}"><div class="card-header ${headerClass}"><div class="card-title-row"><h3>${escapeHtml(name)}</h3>${createBadge(type, 'badge-type')}</div></div><div class="card-body">${content}</div></div>`;
}

function createStatusBadge(status, statusConfig) {
    const config = statusConfig[status] || statusConfig.default;
    return `<div class="flex items-center mb-3"><span class="status-dot ${config.dot}"></span><span class="font-semibold ${config.text}">${config.label}</span></div>`;
}

// Configuration Constants
const STATUS_CONFIG = {
    available: { dot: 'bg-success', text: 'text-success', label: 'Available' },
    expired: { dot: 'bg-warning', text: 'text-warning', label: 'Expired' },
    unavailable: {
        dot: 'bg-danger',
        text: 'text-danger',
        label: 'Unavailable',
    },
    default: { dot: 'bg-danger', text: 'text-danger', label: 'Unavailable' },
};

const TAG_CLASSES = {
    input: 'badge-input',
    formatter: 'badge-brand',
    filter: 'badge-brand',
    type: 'badge-type',
};

const CARD_CLASSES = {
    container: 'card',
    inputHeader: 'card-header-brand',
    outputHeader: 'card-header-slate',
};

// Data Management Helpers
function updateContainerWithCards(container, html) {
    container.innerHTML = html;
    for (const card of container.querySelectorAll('[data-changed="true"]')) {
        animateCardChange(card);
    }
}

function hasDataChanged(current, previous, compareKeys) {
    if (!previous) {
        return true;
    }
    return compareKeys.some((key) => {
        if (typeof current[key] === 'object') {
            return (
                JSON.stringify(current[key]) !== JSON.stringify(previous[key])
            );
        }
        return current[key] !== previous[key];
    });
}

// WebSocket Management
function establishWebSocketConnection() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/dashboard`;

    ws = new WebSocket(wsUrl);

    ws.onopen = () => {
        reconnectDelay = 1000;
        updateConnectionStatus('connected');
    };

    ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        processDashboardUpdate(data);
    };

    ws.onerror = () => {
        // Error handling - connection will be retried on close
    };

    ws.onclose = () => {
        updateConnectionStatus('disconnected');

        if (reconnectTimeout) {
            clearTimeout(reconnectTimeout);
        }
        reconnectTimeout = setTimeout(() => {
            updateConnectionStatus('connecting');
            establishWebSocketConnection();
            reconnectDelay = Math.min(reconnectDelay * 2, maxReconnectDelay);
        }, reconnectDelay);
    };
}

function updateConnectionStatus(status) {
    const plugIcon = document.getElementById('plug-icon');
    const statusText = document.getElementById('connection-status');

    plugIcon.classList.remove('animate-pulse');
    statusText.classList.remove('animate-pulse');

    switch (status) {
        case 'connected':
            statusText.textContent = 'Connected';
            break;
        case 'disconnected':
            statusText.textContent = 'Disconnected';
            break;
        case 'connecting': {
            statusText.textContent = 'Connecting';
            plugIcon.classList.add('animate-pulse');
            statusText.classList.add('animate-pulse');
            break;
        }
        default:
            statusText.textContent = 'Unknown';
            break;
    }
}

function formatDisplayTime(timestamp, useRelative) {
    if (!timestamp) {
        return 'N/A';
    }

    const date = new Date(timestamp);
    const now = new Date();
    const diffSeconds = Math.floor((now - date) / 1000);

    if (useRelative && diffSeconds < 60) {
        if (diffSeconds < 5) {
            return 'just now';
        }
        return `${diffSeconds}s ago`;
    }

    return date.toLocaleTimeString();
}

function animateCardChange(element) {
    element.classList.add('animate-flash');
    setTimeout(() => {
        element.classList.remove('animate-flash');
    }, 500);
}

function updateStatistics(data) {
    const stats = {
        'total-inputs': data.inputs.length,
        'available-inputs': data.inputs.filter((i) => i.available).length,
        'total-outputs': data.outputs.length,
        'active-flows': data.activeFlows,
    };

    for (const [id, newValue] of Object.entries(stats)) {
        const element = document.getElementById(id);
        const oldValue = previousData.stats[id];

        element.textContent = newValue;

        if (oldValue !== undefined && oldValue !== newValue) {
            animateCardChange(element.parentElement);
        }

        previousData.stats[id] = newValue;
    }
}

// Input card HTML builders (extracted to reduce complexity)
function buildMetadataHtml(metadata) {
    if (!metadata) {
        return '';
    }

    const metadataFields = [
        { key: 'artist', label: 'Artist' },
        { key: 'title', label: 'Title' },
        { key: 'songID', label: 'Song ID' },
        { key: 'duration', label: 'Duration' },
    ];

    const fields = metadataFields
        .filter((field) => metadata[field.key])
        .map((field) => createLabeledField(field.label, metadata[field.key]));

    if (fields.length === 0) {
        return '';
    }

    return `<div class="content-box-bordered mt-4 font-mono text-sm break-all">${fields.join('')}</div>`;
}

function buildPrefixSuffixHtml(input) {
    const parts = [];

    if (input.prefix && input.prefix !== 'undefined') {
        parts.push(
            createLabeledField(
                'Prefix',
                input.prefix,
                'text-muted-light',
                'font-mono',
            ),
        );
    }
    if (input.suffix && input.suffix !== 'undefined') {
        parts.push(
            createLabeledField(
                'Suffix',
                input.suffix,
                'text-muted-light',
                'font-mono',
            ),
        );
    }

    if (parts.length === 0) {
        return '';
    }

    return `<div class="mt-3 space-y-1">${parts.map((p) => `<div class="text-sm">${p}</div>`).join('')}</div>`;
}

function buildFilterHtml(filters) {
    if (!filters || filters.length === 0) {
        return '';
    }

    const filterTags = filters
        .map((filter) => createBadge(filter, TAG_CLASSES.filter))
        .join(' ');

    return `<div class="mt-3"><div class="section-label">Filters</div><div class="flex flex-wrap gap-2">${filterTags}</div></div>`;
}

function buildInputTimestampHtml(input) {
    const statusClass = input.status === 'available' ? 'font-medium' : '';
    const updatedTime = formatDisplayTime(
        input.updatedAt,
        input.status === 'available',
    );
    const expiresHtml = input.expiresAt
        ? `<div>Expires: ${formatDisplayTime(input.expiresAt)}</div>`
        : '';

    return `<div class="text-muted-light text-sm mt-4 pt-4 border-t"><div>Updated: <span class="${statusClass}">${updatedTime}</span></div>${expiresHtml}</div>`;
}

function buildInputCardHtml(input) {
    const metadataHtml = buildMetadataHtml(input.metadata);
    const prefixSuffixHtml = buildPrefixSuffixHtml(input);
    const filterHtml = buildFilterHtml(input.filters);
    const timestampHtml = buildInputTimestampHtml(input);

    const prevInput = previousData.inputs[input.name];
    const hasChanged = hasDataChanged(input, prevInput, ['status', 'metadata']);

    const content =
        createStatusBadge(input.status, STATUS_CONFIG) +
        prefixSuffixHtml +
        filterHtml +
        metadataHtml +
        timestampHtml;

    const card = createMetadataCard(
        input.name,
        input.type,
        CARD_CLASSES.inputHeader,
        content,
        hasChanged,
    );

    return card.replace(
        'data-changed=',
        `data-input-name="${input.name}" data-changed=`,
    );
}

function updateInputCards(inputs) {
    const container = document.getElementById('inputs-grid');
    const html = inputs.map((input) => buildInputCardHtml(input)).join('');

    updateContainerWithCards(container, html);

    for (const input of inputs) {
        previousData.inputs[input.name] = {
            status: input.status,
            metadata: input.metadata,
        };
    }
}

function updateOutputCards(outputs) {
    const container = document.getElementById('outputs-grid');

    const html = outputs
        .map((output) => {
            const inputTags = (output.inputs || [])
                .map((input) => createBadge(input, TAG_CLASSES.input))
                .join(' ');

            const formatterTags = (output.formatters || [])
                .map((formatter) =>
                    createBadge(formatter, TAG_CLASSES.formatter),
                )
                .join(' ');

            const prevOutput = previousData.outputs[output.name];
            const hasChanged = hasDataChanged(output, prevOutput, [
                'currentInput',
            ]);

            const content = `<div class="content-box grid-2-col mb-4"><div><div class="text-muted-dark text-sm">Delay</div><div class="font-bold text-lg">${output.delay}s</div></div><div><div class="text-muted-dark text-sm">Current Input</div><div class="font-bold text-lg ${output.currentInput ? 'text-success' : 'text-faint'}">${escapeHtml(output.currentInput || 'None')}</div></div></div><div class="space-y-4"><div><div class="section-label">Inputs (priority order)</div><div class="flex flex-wrap gap-2">${inputTags}</div></div>${
                formatterTags
                    ? `<div><div class="section-label">Formatters</div><div class="flex flex-wrap gap-2">${formatterTags}</div></div>`
                    : ''
            }</div>`;

            const card = createMetadataCard(
                output.name,
                output.type,
                CARD_CLASSES.outputHeader,
                content,
                hasChanged,
            );
            return card.replace(
                'data-changed=',
                `data-output-name="${output.name}" data-changed=`,
            );
        })
        .join('');

    updateContainerWithCards(container, html);

    for (const output of outputs) {
        previousData.outputs[output.name] = {
            currentInput: output.currentInput,
        };
    }
}

function processDashboardUpdate(data) {
    if (!data) {
        return;
    }

    updateStatistics(data);
    updateInputCards(data.inputs);
    updateOutputCards(data.outputs);
}

// Initialize
updateConnectionStatus('connecting');
establishWebSocketConnection();

window.addEventListener('beforeunload', () => {
    if (reconnectTimeout) {
        clearTimeout(reconnectTimeout);
    }
    if (ws) {
        ws.close();
    }
});
