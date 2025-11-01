package web

// dashboardHTML returns the HTML for the dashboard.
func dashboardHTML(stationName, brandColor, version, buildYear string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover">
    <meta name="theme-color" content="` + brandColor + `">
    <meta name="apple-mobile-web-app-capable" content="yes">
    <meta name="apple-mobile-web-app-status-bar-style" content="black-translucent">
    <meta name="apple-mobile-web-app-title" content="` + stationName + ` Metadata">
    <link rel="icon" href="/favicon.ico" sizes="32x32">
    <link rel="icon" href="/favicon.ico" sizes="32x32" media="(prefers-color-scheme: light)">
    <link rel="icon" href="/favicon-dark.ico" sizes="32x32" media="(prefers-color-scheme: dark)">
    <link rel="icon" href="/icon.svg" type="image/svg+xml" media="(prefers-color-scheme: light)">
    <link rel="icon" href="/icon-dark.svg" type="image/svg+xml" media="(prefers-color-scheme: dark)">
    <link rel="apple-touch-icon" href="/apple-touch-icon.png" media="(prefers-color-scheme: light)">
    <link rel="apple-touch-icon" href="/apple-touch-icon-dark.png" media="(prefers-color-scheme: dark)">

    <title>` + stationName + ` Metadata</title>
    <script src="https://cdn.jsdelivr.net/npm/@tailwindcss/browser@4"></script>
    <style type="text/tailwindcss">
        @theme {
            --color-brand: ` + brandColor + `;
            --color-success: #10b981;
            --color-danger: #ef4444;
            --color-warning: #f59e0b;
            --color-muted: #6b7280;
        }

        @keyframes flash {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.6; transform: scale(0.98); }
        }

        .animate-flash {
            animation: flash 0.5s ease-in-out;
        }

    </style>
</head>
<body class="bg-gray-100 dark:bg-slate-900 min-h-screen text-gray-900 dark:text-gray-100 font-sans transition-colors duration-200">
    <div class="max-w-7xl mx-auto p-5">
        <header class="mb-10" role="banner">
            <div class="rounded-2xl border border-gray-200/70 dark:border-gray-700 bg-white dark:bg-gray-800 p-6 sm:p-7 shadow-sm dark:shadow-gray-900/50">
                <div class="flex flex-col gap-5 sm:flex-row sm:items-center sm:justify-between">
                    <div class="flex flex-col items-center gap-3 text-center sm:flex-row sm:items-center sm:gap-5 sm:text-left">
                        <picture class="sm:flex-shrink-0">
                            <source srcset="/icon-dark.svg" media="(prefers-color-scheme: dark)">
                            <img src="/icon.svg" alt="` + stationName + ` brand icon" class="h-12 w-12 sm:h-11 sm:w-11 rounded-lg shadow-sm" loading="lazy">
                        </picture>
                        <div>
                            <h1 class="text-3xl sm:text-[2.35rem] font-semibold tracking-tight text-brand">` + stationName + ` Metadata</h1>
                            <p class="text-muted dark:text-gray-400 text-sm sm:text-base">Real-time metadata routing and synchronization</p>
                        </div>
                    </div>
                    <div class="flex flex-col items-center gap-2 text-center sm:items-end sm:text-right">
                        <nav class="flex flex-wrap items-center justify-center gap-3 text-sm font-medium text-gray-500 dark:text-gray-300 sm:justify-end">
                            <a href="#overview" class="inline-flex items-center gap-2 rounded-lg border border-gray-200/80 dark:border-gray-600 bg-white dark:bg-gray-800/80 px-3.5 py-1.75 transition hover:border-brand hover:text-brand hover:shadow-sm">Overview</a>
                            <a href="#inputs-section" class="inline-flex items-center gap-2 rounded-lg border border-gray-200/80 dark:border-gray-600 bg-white dark:bg-gray-800/80 px-3.5 py-1.75 transition hover:border-brand hover:text-brand hover:shadow-sm">Inputs</a>
                            <a href="#outputs-section" class="inline-flex items-center gap-2 rounded-lg border border-gray-200/80 dark:border-gray-600 bg-white dark:bg-gray-800/80 px-3.5 py-1.75 transition hover:border-brand hover:text-brand hover:shadow-sm">Outputs</a>
                        </nav>
                    </div>
                </div>
            </div>
        </header>

        <main role="main">
        <section id="overview" class="mb-10">
            <h2 class="text-2xl font-semibold mb-5 text-gray-800 dark:text-gray-200">Overview</h2>
            <div id="stats" class="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-5">
            <div class="bg-white dark:bg-gray-800 rounded-xl shadow-md dark:shadow-gray-900/50 p-4 sm:p-6 text-center hover:shadow-xl dark:hover:shadow-gray-900/70 transform hover:-translate-y-1 transition-all">
                <div class="text-3xl sm:text-5xl font-bold text-brand mb-2" id="total-inputs">-</div>
                <div class="text-gray-700 dark:text-gray-300 text-sm sm:text-lg font-medium">Total Inputs</div>
            </div>
            <div class="bg-white dark:bg-gray-800 rounded-xl shadow-md dark:shadow-gray-900/50 p-4 sm:p-6 text-center hover:shadow-xl dark:hover:shadow-gray-900/70 transform hover:-translate-y-1 transition-all">
                <div class="text-3xl sm:text-5xl font-bold text-success mb-2" id="available-inputs">-</div>
                <div class="text-gray-700 dark:text-gray-300 text-sm sm:text-lg font-medium">Available Inputs</div>
            </div>
            <div class="bg-white dark:bg-gray-800 rounded-xl shadow-md dark:shadow-gray-900/50 p-4 sm:p-6 text-center hover:shadow-xl dark:hover:shadow-gray-900/70 transform hover:-translate-y-1 transition-all">
                <div class="text-3xl sm:text-5xl font-bold text-brand mb-2" id="total-outputs">-</div>
                <div class="text-gray-700 dark:text-gray-300 text-sm sm:text-lg font-medium">Total Outputs</div>
            </div>
            <div class="bg-white dark:bg-gray-800 rounded-xl shadow-md dark:shadow-gray-900/50 p-4 sm:p-6 text-center hover:shadow-xl dark:hover:shadow-gray-900/70 transform hover:-translate-y-1 transition-all">
                <div class="text-3xl sm:text-5xl font-bold text-brand mb-2" id="active-flows">-</div>
                <div class="text-gray-700 dark:text-gray-300 text-sm sm:text-lg font-medium">Active Flows</div>
            </div>
        </div>
        </section>
        
        <div class="mb-10" id="inputs-section">
            <h2 class="text-2xl font-semibold mb-5 text-gray-800 dark:text-gray-200">Inputs</h2>
            <div id="inputs-grid" class="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-5">
                <div class="text-center py-12 text-muted dark:text-gray-400">Loading inputs...</div>
            </div>
        </div>

        <div class="mb-10" id="outputs-section">
            <h2 class="text-2xl font-semibold mb-5 text-gray-800 dark:text-gray-200">Outputs</h2>
            <div id="outputs-grid" class="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-5">
                <div class="text-center py-12 text-muted dark:text-gray-400">Loading outputs...</div>
            </div>
        </div>
        
        </main>
        <footer class="mt-16 border-t border-gray-200 dark:border-gray-700" role="contentinfo">
            <div class="max-w-7xl mx-auto px-4 py-8">
                <div class="grid grid-cols-1 md:grid-cols-3 gap-8 items-center">
                    <div>
                        <h3 class="font-semibold text-gray-900 dark:text-gray-100 mb-1 text-center md:text-left">` + stationName + ` Metadata</h3>
                        <div id="connection-indicator" class="flex items-center gap-1.5 justify-center md:justify-start">
                            <svg id="plug-icon" class="w-3 h-3 transition-all duration-300" fill="currentColor" viewBox="0 0 100 100">
                                <path d="M30,5 L30,25 L40,25 L40,5 L30,5 z M60,5 L60,25 L70,25 L70,5 L60,5 z M25,20 C22.239,20 20,22.239,20,25 L20,70 C20,72.761 22.239,75 25,75 L40,75 L40,95 L60,95 L60,75 L75,75 C77.761,75 80,72.761 80,70 L80,25 C80,22.239 77.761,20 75,20 L70,20 L70,25 L60,25 L60,20 L40,20 L40,25 L30,25 L30,20 L25,20 z"/>
                            </svg>
                            <span id="connection-status" class="text-xs text-gray-500 dark:text-gray-400">Connecting</span>
                        </div>
                    </div>

                    <div class="hidden md:block"></div>

                    <div class="text-center md:text-right space-y-2">
                        <div class="flex items-center justify-center md:justify-end gap-3 text-sm">
                            <a href="https://github.com/oszuidwest/zwfm-metadata" target="_blank" class="inline-flex items-center gap-1.5 text-gray-600 dark:text-gray-400 hover:text-brand transition-colors">
                                <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                                    <path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"></path>
                                </svg>
                                GitHub
                            </a>
                            <span class="text-gray-300 dark:text-gray-600">|</span>
                            <span class="text-gray-500 dark:text-gray-400">Version <span id="app-version" class="font-medium">` + version + `</span></span>
                        </div>
                        <div class="text-xs text-gray-400 dark:text-gray-500">
                            © ` + buildYear + ` Streekomroep ZuidWest • MIT License
                        </div>
                    </div>
                </div>
            </div>
        </footer>
    </div>
    
    <script>` + dashboardJS() + `</script>
</body>
</html>`
}

// dashboardJS returns the JavaScript for the dashboard.
func dashboardJS() string {
	return `
        // Store previous data to detect changes
        let previousData = {
            inputs: {},
            outputs: {},
            stats: {}
        };

        // WebSocket connection
        let ws = null;
        let reconnectTimeout = null;
        let reconnectDelay = 1000; // Start with 1 second
        const maxReconnectDelay = 30000; // Max 30 seconds

        // HTML Generation Helpers
        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        function createLabeledField(label, value, labelClass = 'text-gray-600 dark:text-gray-400', valueClass = 'text-gray-900 dark:text-gray-100 font-semibold') {
            return '<div class="mb-2"><span class="' + labelClass + '">' + label + ':</span> <span class="' + valueClass + '">' + escapeHtml(value) + '</span></div>';
        }

        function createBadge(text, classes) {
            return '<span class="' + classes + '">' + escapeHtml(text) + '</span>';
        }

        function createMetadataCard(name, type, headerClass, content, hasChanged) {
            return '<div class="' + CARD_CLASSES.container + '" data-changed="' + hasChanged + '">' +
                '<div class="' + headerClass + '">' +
                    '<div class="flex justify-between items-center">' +
                        '<h3 class="text-xl font-bold">' + escapeHtml(name) + '</h3>' +
                        createBadge(type, TAG_CLASSES.type) +
                    '</div>' +
                '</div>' +
                '<div class="p-6">' + content + '</div>' +
            '</div>';
        }

        function createStatusBadge(status, statusConfig) {
            const config = statusConfig[status] || statusConfig.default;
            return '<div class="flex items-center mb-3">' +
                '<span class="inline-block w-3 h-3 rounded-full mr-2 ' + config.dot + '"></span>' +
                '<span class="font-semibold ' + config.text + '">' + config.label + '</span>' +
            '</div>';
        }

        // Configuration Constants
        const STATUS_CONFIG = {
            available: { dot: 'bg-success', text: 'text-success', label: 'Available' },
            expired: { dot: 'bg-warning', text: 'text-warning', label: 'Expired' },
            unavailable: { dot: 'bg-danger', text: 'text-danger', label: 'Unavailable' },
            default: { dot: 'bg-danger', text: 'text-danger', label: 'Unavailable' }
        };

        const TAG_CLASSES = {
            input: 'bg-gray-100 dark:bg-gray-700 px-3.5 py-1.5 rounded-full text-sm text-gray-800 dark:text-gray-200 font-medium ring-1 ring-gray-300 dark:ring-gray-600 hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors',
            formatter: 'bg-brand/15 dark:bg-brand/25 px-3.5 py-1.5 rounded-full text-sm text-brand dark:text-brand font-semibold ring-1 ring-brand/30 dark:ring-brand/50 hover:bg-brand/20 dark:hover:bg-brand/35 transition-colors',
            type: 'backdrop-blur-sm bg-white/20 dark:bg-white/10 px-4 py-1.5 rounded-full text-sm font-medium text-white ring-1 ring-white/40 dark:ring-white/30 shadow-sm'
        };

        const CARD_CLASSES = {
            container: 'bg-white dark:bg-gray-800 rounded-xl shadow-md dark:shadow-gray-900/50 hover:shadow-xl dark:hover:shadow-gray-900/70 transition-all duration-200 overflow-hidden',
            inputHeader: 'bg-brand p-6 text-white',
            outputHeader: 'bg-slate-700 dark:bg-slate-600 p-6 text-white'
        };

        // Data Management Helpers
        function updateContainerWithCards(container, html) {
            container.innerHTML = html;
            
            // Flash changed cards
            container.querySelectorAll('[data-changed="true"]').forEach(card => {
                animateCardChange(card);
            });
        }

        function hasDataChanged(current, previous, compareKeys) {
            if (!previous) return true;
            return compareKeys.some(key => {
                if (typeof current[key] === 'object') {
                    return JSON.stringify(current[key]) !== JSON.stringify(previous[key]);
                }
                return current[key] !== previous[key];
            });
        }

        // WebSocket Management
        function establishWebSocketConnection() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = protocol + '//' + window.location.host + '/ws/dashboard';
            
            console.log('Connecting to WebSocket:', wsUrl);
            ws = new WebSocket(wsUrl);

            ws.onopen = function() {
                console.log('WebSocket connected');
                reconnectDelay = 1000; // Reset delay on successful connection
                
                // Update connection indicator
                updateConnectionStatus('connected');
            };

            ws.onmessage = function(event) {
                try {
                    const data = JSON.parse(event.data);
                    processDashboardUpdate(data);
                } catch (error) {
                    console.error('Error parsing WebSocket message:', error);
                }
            };

            ws.onerror = function(error) {
                console.error('WebSocket error:', error);
            };

            ws.onclose = function(event) {
                console.log('WebSocket disconnected:', event.code, event.reason);
                
                // Update connection indicator
                updateConnectionStatus('disconnected');
                
                // Schedule reconnection with exponential backoff
                if (reconnectTimeout) clearTimeout(reconnectTimeout);
                reconnectTimeout = setTimeout(() => {
                    updateConnectionStatus('connecting');
                    establishWebSocketConnection();
                    // Increase delay for next attempt (exponential backoff)
                    reconnectDelay = Math.min(reconnectDelay * 2, maxReconnectDelay);
                }, reconnectDelay);
            };
        }

        // Update connection status indicator
        function updateConnectionStatus(status) {
            const plugIcon = document.getElementById('plug-icon');
            const statusText = document.getElementById('connection-status');
            const indicator = document.getElementById('connection-indicator');
            
            switch(status) {
                case 'connected':
                    plugIcon.className = 'w-3 h-3 transition-all duration-300 text-gray-400 dark:text-gray-500';
                    statusText.textContent = 'Connected';
                    statusText.className = 'text-xs text-gray-500 dark:text-gray-400';
                    break;

                case 'disconnected':
                    plugIcon.className = 'w-3 h-3 transition-all duration-300 text-gray-300 dark:text-gray-600';
                    statusText.textContent = 'Disconnected';
                    statusText.className = 'text-xs text-gray-400 dark:text-gray-500';
                    break;

                case 'connecting':
                    plugIcon.className = 'w-3 h-3 transition-all duration-300 text-gray-400 dark:text-gray-500 animate-pulse';
                    statusText.textContent = 'Connecting';
                    statusText.className = 'text-xs text-gray-500 dark:text-gray-400 animate-pulse';
                    break;
            }
        }

        // Format timestamp for display
        function formatDisplayTime(timestamp, useRelative = false) {
            if (!timestamp) return 'N/A';
            
            const date = new Date(timestamp);
            const now = new Date();
            const diffSeconds = Math.floor((now - date) / 1000);
            
            if (useRelative && diffSeconds < 60) {
                if (diffSeconds < 5) return 'just now';
                return diffSeconds + 's ago';
            }
            
            return date.toLocaleTimeString();
        }

        // Visual feedback for data changes
        function animateCardChange(element) {
            element.classList.add('animate-flash');
            setTimeout(() => {
                element.classList.remove('animate-flash');
            }, 500);
        }

        // Update dashboard statistics
        function updateStatistics(data) {
            const stats = {
                'total-inputs': data.inputs.length,
                'available-inputs': data.inputs.filter(i => i.available).length,
                'total-outputs': data.outputs.length,
                'active-flows': data.activeFlows
            };
            
            Object.entries(stats).forEach(([id, newValue]) => {
                const element = document.getElementById(id);
                const oldValue = previousData.stats[id];
                
                element.textContent = newValue;
                
                if (oldValue !== undefined && oldValue !== newValue) {
                    animateCardChange(element.parentElement);
                }
                
                previousData.stats[id] = newValue;
            });
        }

        // Update input cards display
        function updateInputCards(inputs) {
            const container = document.getElementById('inputs-grid');
            
            const html = inputs.map(input => {
                // Build metadata display
                let metadataHtml = '';
                if (input.metadata) {
                    const fields = [];
                    const metadataFields = [
                        { key: 'artist', label: 'Artist' },
                        { key: 'title', label: 'Title' },
                        { key: 'songID', label: 'Song ID' },
                        { key: 'duration', label: 'Duration' }
                    ];
                    
                    metadataFields.forEach(field => {
                        if (input.metadata[field.key]) {
                            fields.push(createLabeledField(field.label, input.metadata[field.key]));
                        }
                    });
                    
                    if (fields.length > 0) {
                        metadataHtml = '<div class="bg-gray-50 dark:bg-gray-900/50 p-4 rounded-lg mt-4 font-mono text-sm break-all border border-gray-200 dark:border-gray-700">' +
                                      fields.join('') +
                                      '</div>';
                    }
                }
                
                // Build prefix/suffix display
                let prefixSuffixHtml = '';
                const parts = [];
                if (input.prefix && input.prefix !== 'undefined') {
                    parts.push(createLabeledField('Prefix', input.prefix, 'text-gray-500 dark:text-gray-400', 'font-mono text-gray-700 dark:text-gray-300'));
                }
                if (input.suffix && input.suffix !== 'undefined') {
                    parts.push(createLabeledField('Suffix', input.suffix, 'text-gray-500 dark:text-gray-400', 'font-mono text-gray-700 dark:text-gray-300'));
                }
                if (parts.length > 0) {
                    prefixSuffixHtml = '<div class="mt-3 space-y-1">' + 
                        parts.map(p => '<div class="text-sm">' + p + '</div>').join('') + 
                        '</div>';
                }
                
                // Check for changes
                const prevInput = previousData.inputs[input.name];
                const hasChanged = hasDataChanged(input, prevInput, ['status', 'metadata']);
                
                // Build content
                const content = createStatusBadge(input.status, STATUS_CONFIG) +
                    prefixSuffixHtml +
                    metadataHtml +
                    '<div class="text-gray-500 dark:text-gray-400 text-sm mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">' +
                        '<div>Updated: <span class="' + (input.status === 'available' ? 'text-gray-700 dark:text-gray-300 font-medium' : '') + '">' +
                        formatDisplayTime(input.updatedAt, input.status === 'available') + '</span></div>' +
                        (input.expiresAt ? '<div>Expires: ' + formatDisplayTime(input.expiresAt) + '</div>' : '') +
                    '</div>';
                
                const card = createMetadataCard(input.name, input.type, CARD_CLASSES.inputHeader, content, hasChanged);
                return card.replace('data-changed=', 'data-input-name="' + input.name + '" data-changed=');
            }).join('');
            
            updateContainerWithCards(container, html);
            
            // Store current data
            inputs.forEach(input => {
                previousData.inputs[input.name] = {
                    status: input.status,
                    metadata: input.metadata
                };
            });
        }

        // Update output cards display
        function updateOutputCards(outputs) {
            const container = document.getElementById('outputs-grid');
            
            const html = outputs.map(output => {
                // Build tags
                const inputTags = (output.inputs || [])
                    .map(input => createBadge(input, TAG_CLASSES.input))
                    .join(' ');
                
                const formatterTags = (output.formatters || [])
                    .map(formatter => createBadge(formatter, TAG_CLASSES.formatter))
                    .join(' ');
                
                // Check for changes
                const prevOutput = previousData.outputs[output.name];
                const hasChanged = hasDataChanged(output, prevOutput, ['currentInput']);
                
                // Build content
                const content =
                    '<div class="grid grid-cols-2 gap-4 mb-4 p-4 bg-gray-50 dark:bg-gray-900/50 rounded-lg">' +
                        '<div>' +
                            '<div class="text-gray-600 dark:text-gray-400 text-sm">Delay</div>' +
                            '<div class="font-bold text-lg dark:text-gray-100">' + output.delay + 's</div>' +
                        '</div>' +
                        '<div>' +
                            '<div class="text-gray-600 dark:text-gray-400 text-sm">Current Input</div>' +
                            '<div class="font-bold text-lg ' + (output.currentInput ? 'text-success' : 'text-gray-400 dark:text-gray-500') + '">' +
                            escapeHtml(output.currentInput || 'None') + '</div>' +
                        '</div>' +
                    '</div>' +
                    '<div class="space-y-4">' +
                        '<div>' +
                            '<div class="text-gray-700 dark:text-gray-300 text-sm mb-2 font-semibold">Inputs (priority order)</div>' +
                            '<div class="flex flex-wrap gap-2">' + inputTags + '</div>' +
                        '</div>' +
                        (formatterTags ? '<div><div class="text-gray-700 dark:text-gray-300 text-sm mb-2 font-semibold">Formatters</div><div class="flex flex-wrap gap-2">' + formatterTags + '</div></div>' : '') +
                    '</div>';
                
                const card = createMetadataCard(output.name, output.type, CARD_CLASSES.outputHeader, content, hasChanged);
                return card.replace('data-changed=', 'data-output-name="' + output.name + '" data-changed=');
            }).join('');
            
            updateContainerWithCards(container, html);
            
            // Store current data
            outputs.forEach(output => {
                previousData.outputs[output.name] = {
                    currentInput: output.currentInput
                };
            });
        }

        // Process and display dashboard data
        function processDashboardUpdate(data) {
            if (!data) return;
            
            updateStatistics(data);
            updateInputCards(data.inputs);
            updateOutputCards(data.outputs);
        }

        // Initialize on page load
        updateConnectionStatus('connecting');
        establishWebSocketConnection();
        
        // Cleanup on page unload
        window.addEventListener('beforeunload', () => {
            if (reconnectTimeout) clearTimeout(reconnectTimeout);
            if (ws) ws.close();
        });
    `
}
