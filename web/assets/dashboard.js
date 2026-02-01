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
    const typeBadge = createBadge(type, 'badge-type');
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

const CARD_CLASSES = {
    container: 'card',
    inputHeader: 'card-header-brand',
    outputHeader: 'card-header-slate',
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
        return null;
    }

    const container = el('div', 'mt-3 space-y-1');
    for (const part of parts) {
        const wrapper = el('div', 'text-sm');
        wrapper.appendChild(part);
        container.appendChild(wrapper);
    }
    return container;
}

function buildFilterBox(filters) {
    if (!filters || filters.length === 0) {
        return null;
    }

    const container = el('div', 'mt-3');
    const label = el('div', 'section-label', 'Filters');
    const badgeContainer = el('div', 'flex flex-wrap gap-2');

    for (const filter of filters) {
        badgeContainer.appendChild(createBadge(filter, TAG_CLASSES.filter));
    }

    container.append(label, badgeContainer);
    return container;
}

function buildInputTimestampBox(input) {
    const container = el('div', 'text-muted-light text-sm mt-4 pt-4 border-t');

    const updatedDiv = el('div');
    updatedDiv.appendChild(document.createTextNode('Updated: '));
    const updatedTime = formatDisplayTime(
        input.updatedAt,
        input.status === 'available',
    );
    const timeSpan = el(
        'span',
        input.status === 'available' ? 'font-medium' : '',
        updatedTime,
    );
    updatedDiv.appendChild(timeSpan);
    container.appendChild(updatedDiv);

    if (input.expiresAt) {
        const expiresDiv = el(
            'div',
            null,
            `Expires: ${formatDisplayTime(input.expiresAt)}`,
        );
        container.appendChild(expiresDiv);
    }

    return container;
}

function buildInputCard(input) {
    const prevInput = previousData.inputs[input.name];
    const hasChanged = hasDataChanged(input, prevInput, ['status', 'metadata']);

    const card = createMetadataCard(
        input.name,
        input.type,
        CARD_CLASSES.inputHeader,
        hasChanged,
    );
    card.dataset.inputName = input.name;

    const body = card.querySelector('.card-body');

    body.appendChild(createStatusBadge(input.status, STATUS_CONFIG));

    const prefixSuffixBox = buildPrefixSuffixBox(input);
    if (prefixSuffixBox) {
        body.appendChild(prefixSuffixBox);
    }

    const filterBox = buildFilterBox(input.filters);
    if (filterBox) {
        body.appendChild(filterBox);
    }

    const metadataBox = buildMetadataBox(input.metadata);
    if (metadataBox) {
        body.appendChild(metadataBox);
    }

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

function buildOutputCard(output) {
    const prevOutput = previousData.outputs[output.name];
    const hasChanged = hasDataChanged(output, prevOutput, ['currentInput']);

    const card = createMetadataCard(
        output.name,
        output.type,
        CARD_CLASSES.outputHeader,
        hasChanged,
    );
    card.dataset.outputName = output.name;

    const body = card.querySelector('.card-body');

    // Stats box with delay and current input
    const statsBox = el('div', 'content-box grid-2-col mb-4');

    const delayCol = el('div');
    delayCol.appendChild(el('div', 'text-muted-dark text-sm', 'Delay'));
    delayCol.appendChild(el('div', 'font-bold text-lg', `${output.delay}s`));

    const inputCol = el('div');
    inputCol.appendChild(el('div', 'text-muted-dark text-sm', 'Current Input'));
    const inputValue = el(
        'div',
        `font-bold text-lg ${output.currentInput ? 'text-success' : 'text-faint'}`,
        output.currentInput || 'None',
    );
    inputCol.appendChild(inputValue);

    statsBox.append(delayCol, inputCol);
    body.appendChild(statsBox);

    // Tags container
    const tagsContainer = el('div', 'space-y-4');

    // Input tags
    const inputsSection = el('div');
    inputsSection.appendChild(
        el('div', 'section-label', 'Inputs (priority order)'),
    );
    const inputBadges = el('div', 'flex flex-wrap gap-2');
    for (const inputName of output.inputs || []) {
        inputBadges.appendChild(createBadge(inputName, TAG_CLASSES.input));
    }
    inputsSection.appendChild(inputBadges);
    tagsContainer.appendChild(inputsSection);

    // Formatter tags
    if (output.formatters && output.formatters.length > 0) {
        const formattersSection = el('div');
        formattersSection.appendChild(el('div', 'section-label', 'Formatters'));
        const formatterBadges = el('div', 'flex flex-wrap gap-2');
        for (const formatter of output.formatters) {
            formatterBadges.appendChild(
                createBadge(formatter, TAG_CLASSES.formatter),
            );
        }
        formattersSection.appendChild(formatterBadges);
        tagsContainer.appendChild(formattersSection);
    }

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
