package web

// dashboardHTML returns the HTML for the dashboard
func dashboardHTML(stationName, brandColor, version, buildYear string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + stationName + ` Metadata</title>
    <script src="https://cdn.tailwindcss.com/4.1.0" type="module"></script>
</head>
<body class="bg-gray-100 min-h-screen text-gray-900">
    <div class="max-w-7xl mx-auto p-5 pb-24">
        <!-- Header -->
        <div class="mb-8">
            <h1 class="text-4xl font-bold mb-2" style="color: ` + brandColor + `;">` + stationName + ` Metadata</h1>
            <p class="text-gray-700 text-lg flex items-center gap-3">
                <span>Real-time metadata routing system</span>
                <span id="connection-status" class="text-xs font-medium flex items-center gap-1.5 text-gray-500">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>
                    </svg>
                    <span id="status-text">Connecting...</span>
                </span>
            </p>
        </div>
        
        <!-- Statistics -->
        <div id="stats" class="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-5 mb-10">
            <div class="bg-white rounded-xl shadow-sm p-4 sm:p-6 text-center">
                <div class="text-3xl sm:text-5xl font-bold mb-2" style="color: ` + brandColor + `;" id="total-inputs">-</div>
                <div class="text-gray-700 text-sm sm:text-lg font-medium">Total Inputs</div>
            </div>
            <div class="bg-white rounded-xl shadow-sm p-4 sm:p-6 text-center">
                <div class="text-3xl sm:text-5xl font-bold text-green-600 mb-2" id="available-inputs">-</div>
                <div class="text-gray-700 text-sm sm:text-lg font-medium">Available Inputs</div>
            </div>
            <div class="bg-white rounded-xl shadow-sm p-4 sm:p-6 text-center">
                <div class="text-3xl sm:text-5xl font-bold mb-2" style="color: ` + brandColor + `;" id="total-outputs">-</div>
                <div class="text-gray-700 text-sm sm:text-lg font-medium">Total Outputs</div>
            </div>
            <div class="bg-white rounded-xl shadow-sm p-4 sm:p-6 text-center">
                <div class="text-3xl sm:text-5xl font-bold mb-2" style="color: ` + brandColor + `;" id="active-flows">-</div>
                <div class="text-gray-700 text-sm sm:text-lg font-medium">Active Flows</div>
            </div>
        </div>
        
        <!-- Inputs Section -->
        <div class="mb-10">
            <h2 class="text-2xl font-semibold mb-5 text-gray-800">Inputs</h2>
            <div id="inputs-grid" class="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-5">
                <div class="text-center py-12 text-gray-500">Loading inputs...</div>
            </div>
        </div>
        
        <!-- Outputs Section -->
        <div class="mb-10">
            <h2 class="text-2xl font-semibold mb-5 text-gray-800">Outputs</h2>
            <div id="outputs-grid" class="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-5">
                <div class="text-center py-12 text-gray-500">Loading outputs...</div>
            </div>
        </div>
    </div>
    
    <!-- Footer -->
    <footer class="bg-white border-t border-gray-200 mt-auto">
        <div class="max-w-7xl mx-auto px-5 py-6">
            <div class="flex flex-col sm:flex-row justify-between items-center gap-4">
                <div class="text-gray-600 text-sm flex items-center gap-1.5">
                    <svg id="connection-icon" class="w-4 h-4 text-green-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M11 4a2 2 0 114 0v1a1 1 0 001 1h3a1 1 0 011 1v3a1 1 0 01-1 1h-1a2 2 0 100 4h1a1 1 0 011 1v3a1 1 0 01-1 1h-3a1 1 0 01-1-1v-1a2 2 0 10-4 0v1a1 1 0 01-1 1H7a1 1 0 01-1-1v-3a1 1 0 00-1-1H4a2 2 0 110-4h1a1 1 0 001-1V7a1 1 0 011-1h3a1 1 0 001-1V4z" />
                    </svg>
                    <span id="connection-text" class="text-green-600">Connected</span>
                </div>
                <div class="text-gray-600 text-sm text-center">
                    © ` + buildYear + ` Streekomroep ZuidWest • MIT License • v` + version + `
                </div>
                <div>
                    <a href="https://github.com/oszuidwest/zwfm-metadata" target="_blank" class="text-gray-600 hover:text-gray-900 transition-colors">
                        <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
                            <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
                        </svg>
                    </a>
                </div>
            </div>
        </div>
    </footer>
    
    <script>` + dashboardJS(brandColor) + `</script>
</body>
</html>`
}

// dashboardJS returns the JavaScript for the dashboard
func dashboardJS(brandColor string) string {
	return `
        // Constants for styling
        const BRAND_COLOR = '` + brandColor + `';
        
        const CARD_CLASSES = {
            container: 'bg-white rounded-xl shadow-sm hover:shadow-md transition-all duration-200 overflow-hidden',
            statusAvailable: 'bg-green-600',
            statusExpired: 'bg-orange-600',
            statusUnavailable: 'bg-red-600',
            headerInput: 'p-6 text-white',
            headerOutput: 'bg-gray-800 p-6 text-white'
        };
        
        const TAG_CLASSES = {
            type: 'backdrop-blur-sm bg-white/20 px-4 py-1.5 rounded-full text-sm font-medium text-white ring-1 ring-white/40 shadow-sm',
            input: 'bg-gray-100 px-3.5 py-1.5 rounded-full text-sm text-gray-800 font-medium ring-1 ring-gray-300',
            formatter: 'bg-gray-100 px-3.5 py-1.5 rounded-full text-sm text-gray-800 font-medium ring-1 ring-gray-300'
        };
        
        const STATUS_COLORS = {
            available: { dot: 'bg-green-600', text: 'text-green-600' },
            expired: { dot: 'bg-orange-600', text: 'text-orange-600' },
            unavailable: { dot: 'bg-red-600', text: 'text-red-600' }
        };
        
        const STATUS_TEXT = {
            available: 'Available',
            expired: 'Expired',
            unavailable: 'Unavailable'
        };

        // Store previous data to detect changes
        let previousData = {
            inputs: {},
            outputs: {},
            stats: {}
        };

        // WebSocket connection
        let ws = null;
        let reconnectTimer = null;
        let reconnectAttempts = 0;

        // Helper functions
        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        function createBadge(text, className) {
            return '<span class="' + className + '">' + escapeHtml(text) + '</span>';
        }

        function createStatusIndicator(status) {
            const config = STATUS_COLORS[status] || STATUS_COLORS.unavailable;
            const text = STATUS_TEXT[status] || 'Unavailable';
            
            return '<div class="flex items-center mb-3">' +
                '<span class="inline-block w-3 h-3 rounded-full mr-2 ' + config.dot + '"></span>' +
                '<span class="font-semibold ' + config.text + '">' + text + '</span>' +
            '</div>';
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

        function formatTimestamp(timestamp, useRelative = false) {
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

        function flashElement(element) {
            element.style.transform = 'scale(0.98)';
            element.style.opacity = '0.7';
            setTimeout(() => {
                element.style.transform = '';
                element.style.opacity = '';
            }, 300);
        }

        // Update functions
        function updateStats(data) {
            const stats = {
                'total-inputs': data.inputs.length,
                'available-inputs': data.inputs.filter(i => i.available).length,
                'total-outputs': data.outputs.length,
                'active-flows': data.activeFlows
            };
            
            Object.keys(stats).forEach(id => {
                const element = document.getElementById(id);
                const newValue = stats[id];
                const oldValue = previousData.stats[id];
                
                element.textContent = newValue;
                
                if (oldValue !== undefined && oldValue !== newValue) {
                    flashElement(element.parentElement);
                }
                
                previousData.stats[id] = newValue;
            });
        }

        function updateInputCards(inputs) {
            const container = document.getElementById('inputs-grid');
            
            const html = inputs.map(input => {
                // Build metadata display
                let metadataHtml = '';
                if (input.metadata) {
                    const fields = [];
                    if (input.metadata.artist) {
                        fields.push('<div class="mb-2"><span class="text-gray-600">Artist:</span> <span class="text-gray-900 font-semibold">' + escapeHtml(input.metadata.artist) + '</span></div>');
                    }
                    if (input.metadata.title) {
                        fields.push('<div class="mb-2"><span class="text-gray-600">Title:</span> <span class="text-gray-900 font-semibold">' + escapeHtml(input.metadata.title) + '</span></div>');
                    }
                    if (input.metadata.songID) {
                        fields.push('<div class="mb-2"><span class="text-gray-600">Song ID:</span> <span class="text-gray-900 font-semibold">' + escapeHtml(input.metadata.songID) + '</span></div>');
                    }
                    if (input.metadata.duration) {
                        fields.push('<div class="mb-2"><span class="text-gray-600">Duration:</span> <span class="text-gray-900 font-semibold">' + escapeHtml(input.metadata.duration) + '</span></div>');
                    }
                    
                    if (fields.length > 0) {
                        metadataHtml = '<div class="bg-gray-50 p-4 rounded-lg mt-4 font-mono text-sm break-all border border-gray-200">' + 
                                      fields.join('') + 
                                      '</div>';
                    }
                }
                
                // Build prefix/suffix display
                let prefixSuffixHtml = '';
                if ((input.prefix && input.prefix !== 'undefined') || (input.suffix && input.suffix !== 'undefined')) {
                    const parts = [];
                    if (input.prefix && input.prefix !== 'undefined') {
                        parts.push('<div class="text-sm"><span class="text-gray-500">Prefix:</span> <span class="font-mono text-gray-700">' + escapeHtml(input.prefix) + '</span></div>');
                    }
                    if (input.suffix && input.suffix !== 'undefined') {
                        parts.push('<div class="text-sm"><span class="text-gray-500">Suffix:</span> <span class="font-mono text-gray-700">' + escapeHtml(input.suffix) + '</span></div>');
                    }
                    if (parts.length > 0) {
                        prefixSuffixHtml = '<div class="mt-3 space-y-1">' + parts.join('') + '</div>';
                    }
                }
                
                // Check for changes
                const prevInput = previousData.inputs[input.name] || {};
                const hasChanged = prevInput.status !== input.status || 
                                 JSON.stringify(prevInput.metadata) !== JSON.stringify(input.metadata);
                
                const headerClass = CARD_CLASSES.headerInput + ' ' + (CARD_CLASSES['status' + input.status.charAt(0).toUpperCase() + input.status.slice(1)] || CARD_CLASSES.statusUnavailable);
                headerClass.replace('undefined', CARD_CLASSES.statusUnavailable);
                
                const content = createStatusIndicator(input.status) +
                    prefixSuffixHtml +
                    metadataHtml +
                    '<div class="text-gray-500 text-sm mt-4 pt-4 border-t border-gray-200">' +
                        '<div>Updated: <span class="' + (input.status === 'available' ? 'text-gray-700 font-medium' : '') + '">' + 
                        formatTimestamp(input.updatedAt, input.status === 'available') + '</span></div>' +
                        (input.expiresAt ? '<div>Expires: ' + formatTimestamp(input.expiresAt) + '</div>' : '') +
                    '</div>';
                
                return createMetadataCard(input.name, input.type, headerClass, content, hasChanged);
            }).join('');
            
            container.innerHTML = html;
            
            // Flash changed cards
            container.querySelectorAll('[data-changed="true"]').forEach(card => {
                flashElement(card);
            });
            
            // Store current data
            inputs.forEach(input => {
                previousData.inputs[input.name] = {
                    status: input.status,
                    metadata: input.metadata
                };
            });
        }

        function updateOutputCards(outputs) {
            const container = document.getElementById('outputs-grid');
            
            const html = outputs.map(output => {
                // Build tags
                const inputsHtml = (output.inputs || [])
                    .map(input => createBadge(input, TAG_CLASSES.input))
                    .join(' ');
                
                const formattersHtml = (output.formatters || [])
                    .map(formatter => createBadge(formatter, TAG_CLASSES.formatter))
                    .join(' ');
                
                // Check for changes
                const prevOutput = previousData.outputs[output.name] || {};
                const hasChanged = prevOutput.currentInput !== output.currentInput;
                
                const content = '<div class="grid grid-cols-2 gap-4 mb-4 p-4 bg-gray-50 rounded-lg">' +
                    '<div>' +
                        '<div class="text-gray-600 text-sm">Delay</div>' +
                        '<div class="font-bold text-lg">' + output.delay + 's</div>' +
                    '</div>' +
                    '<div>' +
                        '<div class="text-gray-600 text-sm">Current Input</div>' +
                        '<div class="font-bold text-lg ' + (output.currentInput ? 'text-green-600' : 'text-gray-400') + '">' + 
                        escapeHtml(output.currentInput || 'None') + '</div>' +
                    '</div>' +
                '</div>' +
                '<div class="space-y-4">' +
                    '<div>' +
                        '<div class="text-gray-700 text-sm mb-2 font-semibold">Inputs (priority order)</div>' +
                        '<div class="flex flex-wrap gap-2">' + inputsHtml + '</div>' +
                    '</div>' +
                    (formattersHtml ? '<div><div class="text-gray-700 text-sm mb-2 font-semibold">Formatters</div><div class="flex flex-wrap gap-2">' + formattersHtml + '</div></div>' : '') +
                '</div>';
                
                return createMetadataCard(output.name, output.type, CARD_CLASSES.headerOutput, content, hasChanged);
            }).join('');
            
            container.innerHTML = html;
            
            // Flash changed cards
            container.querySelectorAll('[data-changed="true"]').forEach(card => {
                flashElement(card);
            });
            
            // Store current data
            outputs.forEach(output => {
                previousData.outputs[output.name] = {
                    currentInput: output.currentInput
                };
            });
        }

        function updateConnectionStatus(connected) {
            const statusText = document.getElementById('status-text');
            const connectionText = document.getElementById('connection-text');
            const connectionIcon = document.getElementById('connection-icon');
            const connectionStatus = document.getElementById('connection-status');
            
            if (connected) {
                statusText.textContent = 'Connected';
                connectionText.textContent = 'Connected';
                connectionStatus.classList.remove('text-red-600');
                connectionStatus.classList.add('text-gray-500');
                connectionIcon.classList.remove('text-red-600');
                connectionIcon.classList.add('text-green-600');
                connectionText.classList.remove('text-red-600');
                connectionText.classList.add('text-green-600');
            } else {
                statusText.textContent = 'Disconnected';
                connectionText.textContent = 'Disconnected';
                connectionStatus.classList.remove('text-gray-500');
                connectionStatus.classList.add('text-red-600');
                connectionIcon.classList.remove('text-green-600');
                connectionIcon.classList.add('text-red-600');
                connectionText.classList.remove('text-green-600');
                connectionText.classList.add('text-red-600');
            }
        }

        // WebSocket functions
        function connectWebSocket() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = protocol + '//' + window.location.host + '/ws/dashboard';
            
            try {
                ws = new WebSocket(wsUrl);
                
                ws.onopen = function() {
                    console.log('WebSocket connected');
                    updateConnectionStatus(true);
                    reconnectAttempts = 0;
                };
                
                ws.onmessage = function(event) {
                    try {
                        const data = JSON.parse(event.data);
                        updateStats(data);
                        updateInputCards(data.inputs || []);
                        updateOutputCards(data.outputs || []);
                    } catch (err) {
                        console.error('Error processing WebSocket message:', err);
                    }
                };
                
                ws.onclose = function() {
                    console.log('WebSocket disconnected');
                    updateConnectionStatus(false);
                    scheduleReconnect();
                };
                
                ws.onerror = function(error) {
                    console.error('WebSocket error:', error);
                    updateConnectionStatus(false);
                };
                
            } catch (err) {
                console.error('Failed to create WebSocket:', err);
                updateConnectionStatus(false);
                scheduleReconnect();
            }
        }

        function scheduleReconnect() {
            if (reconnectTimer) return;
            
            reconnectAttempts++;
            const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 30000);
            
            reconnectTimer = setTimeout(() => {
                reconnectTimer = null;
                connectWebSocket();
            }, delay);
        }

        // Initialize on page load
        connectWebSocket();
    `
}