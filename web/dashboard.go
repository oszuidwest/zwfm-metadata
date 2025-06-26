package web

// dashboardHTML returns the HTML for the dashboard
func dashboardHTML(stationName, brandColor string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>` + stationName + ` Metadata</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script>
        tailwind.config = {
            theme: {
                extend: {
                    colors: {
                        'brand': '` + brandColor + `',
                        'success': '#10b981',
                        'danger': '#ef4444',
                        'warning': '#f59e0b',
                        'muted': '#6b7280'
                    },
                    animation: {
                        'flash': 'flash 0.5s ease-in-out'
                    },
                    keyframes: {
                        flash: {
                            '0%, 100%': { opacity: '1' },
                            '50%': { opacity: '0.6', transform: 'scale(0.98)' }
                        }
                    }
                }
            }
        }
    </script>
</head>
<body class="bg-gradient-to-br from-gray-100 to-gray-200 min-h-screen text-gray-900 font-sans">
    <div class="max-w-7xl mx-auto p-5">
        <!-- Header -->
        <div class="mb-8">
            <h1 class="text-4xl font-bold text-brand mb-2">` + stationName + ` Metadata</h1>
            <p class="text-muted text-lg">Real-time metadata routing and synchronization</p>
        </div>
        
        <!-- Statistics -->
        <div id="stats" class="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-5 mb-10">
            <div class="bg-white rounded-xl shadow-md p-4 sm:p-6 text-center hover:shadow-xl transform hover:-translate-y-1 transition-all">
                <div class="text-3xl sm:text-5xl font-bold text-brand mb-2" id="total-inputs">-</div>
                <div class="text-gray-700 text-sm sm:text-lg font-medium">Total Inputs</div>
            </div>
            <div class="bg-white rounded-xl shadow-md p-4 sm:p-6 text-center hover:shadow-xl transform hover:-translate-y-1 transition-all">
                <div class="text-3xl sm:text-5xl font-bold text-success mb-2" id="available-inputs">-</div>
                <div class="text-gray-700 text-sm sm:text-lg font-medium">Available Inputs</div>
            </div>
            <div class="bg-white rounded-xl shadow-md p-4 sm:p-6 text-center hover:shadow-xl transform hover:-translate-y-1 transition-all">
                <div class="text-3xl sm:text-5xl font-bold text-brand mb-2" id="total-outputs">-</div>
                <div class="text-gray-700 text-sm sm:text-lg font-medium">Total Outputs</div>
            </div>
            <div class="bg-white rounded-xl shadow-md p-4 sm:p-6 text-center hover:shadow-xl transform hover:-translate-y-1 transition-all">
                <div class="text-3xl sm:text-5xl font-bold text-brand mb-2" id="active-flows">-</div>
                <div class="text-gray-700 text-sm sm:text-lg font-medium">Active Flows</div>
            </div>
        </div>
        
        <!-- Inputs Section -->
        <div class="mb-10">
            <h2 class="text-2xl font-semibold mb-5 text-gray-800">Inputs</h2>
            <div id="inputs-grid" class="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-5">
                <div class="text-center py-12 text-muted">Loading inputs...</div>
            </div>
        </div>
        
        <!-- Outputs Section -->
        <div class="mb-10">
            <h2 class="text-2xl font-semibold mb-5 text-gray-800">Outputs</h2>
            <div id="outputs-grid" class="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-5">
                <div class="text-center py-12 text-muted">Loading outputs...</div>
            </div>
        </div>
    </div>
    
    <script>` + dashboardJS() + `</script>
</body>
</html>`
}

// dashboardJS returns the JavaScript for the dashboard
func dashboardJS() string {
	return `
        // Store previous data to detect changes
        let previousData = {
            inputs: {},
            outputs: {},
            stats: {}
        };

        // Fetch dashboard data from API
        async function fetchDashboardData() {
            try {
                const response = await fetch('/status');
                if (!response.ok) throw new Error('Failed to fetch dashboard data');
                return await response.json();
            } catch (error) {
                console.error('Error fetching dashboard data:', error);
                return null;
            }
        }

        // Format timestamp with optional relative time
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

        // Flash element animation for changes
        function flashElement(element) {
            element.classList.add('animate-flash');
            setTimeout(() => {
                element.classList.remove('animate-flash');
            }, 500);
        }

        // Render statistics cards
        function renderStats(data) {
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

        // Render input cards
        function renderInputs(inputs) {
            const container = document.getElementById('inputs-grid');
            
            const html = inputs.map(input => {
                // Build metadata display
                let metadataHtml = '';
                if (input.metadata) {
                    const fields = [];
                    if (input.metadata.artist) {
                        fields.push('<div class="mb-2"><span class="text-gray-600">Artist:</span> <span class="text-gray-900 font-semibold">' + input.metadata.artist + '</span></div>');
                    }
                    if (input.metadata.title) {
                        fields.push('<div class="mb-2"><span class="text-gray-600">Title:</span> <span class="text-gray-900 font-semibold">' + input.metadata.title + '</span></div>');
                    }
                    if (input.metadata.songID) {
                        fields.push('<div class="mb-2"><span class="text-gray-600">Song ID:</span> <span class="text-gray-900 font-semibold">' + input.metadata.songID + '</span></div>');
                    }
                    if (input.metadata.duration) {
                        fields.push('<div class="mb-2"><span class="text-gray-600">Duration:</span> <span class="text-gray-900 font-semibold">' + input.metadata.duration + '</span></div>');
                    }
                    
                    if (fields.length > 0) {
                        metadataHtml = '<div class="bg-gradient-to-r from-gray-50 to-gray-100 p-4 rounded-lg mt-4 font-mono text-sm break-all border border-gray-200">' + 
                                      fields.join('') + 
                                      '</div>';
                    }
                }
                
                // Build prefix/suffix display
                let prefixSuffixHtml = '';
                if (input.prefix || input.suffix) {
                    const parts = [];
                    if (input.prefix) {
                        parts.push('<div class="text-sm"><span class="text-gray-500">Prefix:</span> <span class="font-mono text-gray-700">' + input.prefix + '</span></div>');
                    }
                    if (input.suffix) {
                        parts.push('<div class="text-sm"><span class="text-gray-500">Suffix:</span> <span class="font-mono text-gray-700">' + input.suffix + '</span></div>');
                    }
                    prefixSuffixHtml = '<div class="mt-3 space-y-1">' + parts.join('') + '</div>';
                }
                
                // Check for changes
                const prevInput = previousData.inputs[input.name] || {};
                const hasChanged = prevInput.status !== input.status || 
                                 JSON.stringify(prevInput.metadata) !== JSON.stringify(input.metadata);
                
                // Status colors
                const statusClass = {
                    available: { dot: 'bg-success', text: 'text-success' },
                    expired: { dot: 'bg-warning', text: 'text-warning' },
                    unavailable: { dot: 'bg-danger', text: 'text-danger' }
                }[input.status] || { dot: 'bg-danger', text: 'text-danger' };
                
                const statusText = {
                    available: 'Available',
                    expired: 'Expired',
                    unavailable: 'Unavailable'
                }[input.status] || 'Unavailable';
                
                // Build card HTML
                return '<div class="bg-white rounded-xl shadow-md hover:shadow-xl transition-all duration-200 overflow-hidden" data-input-name="' + input.name + '" data-changed="' + hasChanged + '">' +
                    '<div class="bg-brand p-6 text-white">' +
                        '<div class="flex justify-between items-center">' +
                            '<h3 class="text-xl font-bold">' + input.name + '</h3>' +
                            '<span class="backdrop-blur-sm bg-white/20 px-4 py-1.5 rounded-full text-sm font-medium text-white ring-1 ring-white/40 shadow-sm">' + input.type + '</span>' +
                        '</div>' +
                    '</div>' +
                    '<div class="p-6">' +
                        '<div class="flex items-center mb-3">' +
                            '<span class="inline-block w-3 h-3 rounded-full mr-2 ' + statusClass.dot + '"></span>' +
                            '<span class="font-semibold ' + statusClass.text + '">' + statusText + '</span>' +
                        '</div>' +
                        prefixSuffixHtml +
                        metadataHtml +
                        '<div class="text-gray-500 text-sm mt-4 pt-4 border-t border-gray-200">' +
                            '<div>Updated: <span class="' + (input.status === 'available' ? 'text-gray-700 font-medium' : '') + '">' + 
                            formatTimestamp(input.updatedAt, input.status === 'available') + '</span></div>' +
                            (input.expiresAt ? '<div>Expires: ' + formatTimestamp(input.expiresAt) + '</div>' : '') +
                        '</div>' +
                    '</div>' +
                '</div>';
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

        // Render output cards
        function renderOutputs(outputs) {
            const container = document.getElementById('outputs-grid');
            
            const html = outputs.map(output => {
                // Build tags
                const inputsHtml = output.inputs
                    .map(input => '<span class="bg-gray-100 px-3.5 py-1.5 rounded-full text-sm text-gray-800 font-medium ring-1 ring-gray-300 hover:bg-gray-200 transition-colors">' + input + '</span>')
                    .join(' ');
                
                const formattersHtml = output.formatters
                    .map(formatter => '<span class="bg-brand/15 px-3.5 py-1.5 rounded-full text-sm text-brand font-semibold ring-1 ring-brand/30 hover:bg-brand/20 transition-colors">' + formatter + '</span>')
                    .join(' ');
                
                // Check for changes
                const prevOutput = previousData.outputs[output.name] || {};
                const hasChanged = prevOutput.currentInput !== output.currentInput;
                
                // Build card HTML
                return '<div class="bg-white rounded-xl shadow-md hover:shadow-xl transition-all duration-200 overflow-hidden" data-output-name="' + output.name + '" data-changed="' + hasChanged + '">' +
                    '<div class="bg-gradient-to-r from-gray-700 to-gray-900 p-6 text-white">' +
                        '<div class="flex justify-between items-center">' +
                            '<h3 class="text-xl font-bold">' + output.name + '</h3>' +
                            '<span class="backdrop-blur-sm bg-white/20 px-4 py-1.5 rounded-full text-sm font-medium text-white ring-1 ring-white/40 shadow-sm">' + output.type + '</span>' +
                        '</div>' +
                    '</div>' +
                    '<div class="p-6">' +
                        '<div class="grid grid-cols-2 gap-4 mb-4 p-4 bg-gray-50 rounded-lg">' +
                            '<div>' +
                                '<div class="text-gray-600 text-sm">Delay</div>' +
                                '<div class="font-bold text-lg">' + output.delay + 's</div>' +
                            '</div>' +
                            '<div>' +
                                '<div class="text-gray-600 text-sm">Current Input</div>' +
                                '<div class="font-bold text-lg ' + (output.currentInput ? 'text-success' : 'text-gray-400') + '">' + 
                                (output.currentInput || 'None') + '</div>' +
                            '</div>' +
                        '</div>' +
                        '<div class="space-y-4">' +
                            '<div>' +
                                '<div class="text-gray-700 text-sm mb-2 font-semibold">Inputs (priority order)</div>' +
                                '<div class="flex flex-wrap gap-2">' + inputsHtml + '</div>' +
                            '</div>' +
                            (formattersHtml ? '<div><div class="text-gray-700 text-sm mb-2 font-semibold">Formatters</div><div class="flex flex-wrap gap-2">' + formattersHtml + '</div></div>' : '') +
                        '</div>' +
                    '</div>' +
                '</div>';
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

        // Main update function
        async function updateDashboard() {
            const data = await fetchDashboardData();
            if (!data) return;
            
            renderStats(data);
            renderInputs(data.inputs);
            renderOutputs(data.outputs);
        }

        // Initialize on page load
        updateDashboard();
        
        // Auto-refresh every second
        setInterval(updateDashboard, 1000);
    `
}
