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

// DOM Element Factory Helpers
function el(tag, className, textContent) {
    const element = document.createElement(tag);
    if (className) {
        element.className = className;
    }
    if (textContent !== undefined) {
        element.textContent = textContent;
    }
    return element;
}

function createLabeledField(
    label,
    value,
    labelClass = 'text-muted-dark',
    valueClass = 'font-semibold',
) {
    const container = el('div', 'mb-2');
    const labelSpan = el('span', labelClass, `${label}: `);
    const valueSpan = el('span', valueClass, value);
    container.append(labelSpan, valueSpan);
    return container;
}

function createBadge(text, classes) {
    return el('span', `badge ${classes}`, text);
}

function createStatusBadge(status, statusConfig) {
    const config = statusConfig[status] || statusConfig.default;
    const container = el('div', 'flex items-center mb-3');
    const dot = el('span', `status-dot ${config.dot}`);
    const label = el('span', `font-semibold ${config.text}`, config.label);
    container.append(dot, label);
    return container;
}

function createMetadataCard(name, type, headerClass, hasChanged) {
    const card = el('div', 'card');
    card.dataset.changed = hasChanged;

    const header = el('div', `card-header ${headerClass}`);
    const titleRow = el('div', 'card-title-row');
    const h3 = el('h3', null, name);
    const typeBadge = createBadge(type, TAG_CLASSES.type);
    titleRow.append(h3, typeBadge);
    header.appendChild(titleRow);

    const body = el('div', 'card-body');

    card.append(header, body);
    return card;
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

const CARD_HEADER_CLASSES = {
    input: 'card-header-brand',
    output: 'card-header-slate',
};

// Data Management Helpers
function updateContainerWithCards(container, cards) {
    container.replaceChildren(...cards);
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

    const statusLabels = {
        connected: 'Connected',
        disconnected: 'Disconnected',
        connecting: 'Connecting',
    };

    statusText.textContent = statusLabels[status] || 'Unknown';

    if (status === 'connecting') {
        plugIcon.classList.add('animate-pulse');
        statusText.classList.add('animate-pulse');
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

// Input card DOM builders
function buildMetadataBox(metadata) {
    if (!metadata) {
        return null;
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
        return null;
    }

    const container = el(
        'div',
        'content-box-bordered mt-4 font-mono text-sm break-all',
    );
    container.append(...fields);
    return container;
}

function buildPrefixSuffixBox(input) {
    const fields = [
        { label: 'Prefix', value: input.prefix },
        { label: 'Suffix', value: input.suffix },
    ].filter((f) => f.value && f.value !== 'undefined');

    if (fields.length === 0) {
        return null;
    }

    const container = el('div', 'mt-3 space-y-1');
    for (const field of fields) {
        const wrapper = el('div', 'text-sm');
        wrapper.appendChild(
            createLabeledField(
                field.label,
                field.value,
                'text-muted-light',
                'font-mono',
            ),
        );
        container.appendChild(wrapper);
    }
    return container;
}

function buildBadgeSection(label, items, badgeClass) {
    if (!items || items.length === 0) {
        return null;
    }

    const container = el('div');
    container.appendChild(el('div', 'section-label', label));

    const badgeContainer = el('div', 'flex flex-wrap gap-2');
    for (const item of items) {
        badgeContainer.appendChild(createBadge(item, badgeClass));
    }
    container.appendChild(badgeContainer);

    return container;
}

function buildInputTimestampBox(input) {
    const container = el('div', 'text-muted-light text-sm mt-4 pt-4 border-t');
    const isAvailable = input.status === 'available';
    const updatedTime = formatDisplayTime(input.updatedAt, isAvailable);

    const updatedDiv = el('div');
    const labelSpan = el('span', null, 'Updated: ');
    const timeSpan = el('span', isAvailable ? 'font-medium' : '', updatedTime);
    updatedDiv.append(labelSpan, timeSpan);
    container.appendChild(updatedDiv);

    if (input.expiresAt) {
        container.appendChild(
            el('div', null, `Expires: ${formatDisplayTime(input.expiresAt)}`),
        );
    }

    return container;
}

function appendIfPresent(parent, element) {
    if (element) {
        parent.appendChild(element);
    }
}

function buildInputCard(input) {
    const prevInput = previousData.inputs[input.name];
    const hasChanged = hasDataChanged(input, prevInput, ['status', 'metadata']);

    const card = createMetadataCard(
        input.name,
        input.type,
        CARD_HEADER_CLASSES.input,
        hasChanged,
    );
    card.dataset.inputName = input.name;

    const body = card.querySelector('.card-body');
    if (!body) {
        return card;
    }

    body.appendChild(createStatusBadge(input.status, STATUS_CONFIG));
    appendIfPresent(body, buildPrefixSuffixBox(input));

    const filterSection = buildBadgeSection(
        'Filters',
        input.filters,
        TAG_CLASSES.filter,
    );
    if (filterSection) {
        filterSection.classList.add('mt-3');
        body.appendChild(filterSection);
    }

    appendIfPresent(body, buildMetadataBox(input.metadata));
    body.appendChild(buildInputTimestampBox(input));

    return card;
}

function updateInputCards(inputs) {
    const container = document.getElementById('inputs-grid');
    const cards = inputs.map((input) => buildInputCard(input));

    updateContainerWithCards(container, cards);

    for (const input of inputs) {
        previousData.inputs[input.name] = {
            status: input.status,
            metadata: input.metadata,
        };
    }
}

function buildStatColumn(label, value, valueClass) {
    const col = el('div');
    col.appendChild(el('div', 'text-muted-dark text-sm', label));
    const className = valueClass
        ? `font-bold text-lg ${valueClass}`
        : 'font-bold text-lg';
    col.appendChild(el('div', className, value));
    return col;
}

function buildOutputCard(output) {
    const prevOutput = previousData.outputs[output.name];
    const hasChanged = hasDataChanged(output, prevOutput, ['currentInput']);

    const card = createMetadataCard(
        output.name,
        output.type,
        CARD_HEADER_CLASSES.output,
        hasChanged,
    );
    card.dataset.outputName = output.name;

    const body = card.querySelector('.card-body');
    if (!body) {
        return card;
    }

    // Stats box with delay and current input
    const statsBox = el('div', 'content-box grid-2-col mb-4');
    const inputValueClass = output.currentInput ? 'text-success' : 'text-faint';
    statsBox.append(
        buildStatColumn('Delay', `${output.delay}s`),
        buildStatColumn(
            'Current Input',
            output.currentInput || 'None',
            inputValueClass,
        ),
    );
    body.appendChild(statsBox);

    // Tags container
    const tagsContainer = el('div', 'space-y-4');

    const inputsSection = buildBadgeSection(
        'Inputs (priority order)',
        output.inputs || [],
        TAG_CLASSES.input,
    );
    if (inputsSection) {
        tagsContainer.appendChild(inputsSection);
    }

    const formattersSection = buildBadgeSection(
        'Formatters',
        output.formatters,
        TAG_CLASSES.formatter,
    );
    appendIfPresent(tagsContainer, formattersSection);

    body.appendChild(tagsContainer);

    return card;
}

function updateOutputCards(outputs) {
    const container = document.getElementById('outputs-grid');
    const cards = outputs.map((output) => buildOutputCard(output));

    updateContainerWithCards(container, cards);

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
