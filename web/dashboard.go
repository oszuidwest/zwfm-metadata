package web

import (
	_ "embed"
)

//go:embed assets/dashboard.css
var cssContent string

//go:embed assets/dashboard.js
var jsContent string

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
    <style>:root { --brand: ` + brandColor + `; }</style>
    <style>
` + cssContent + `
    </style>
</head>
<body>
    <div class="container">
        <header class="site-header mb-10" role="banner">
            <div class="header-content">
                <div class="header-brand">
                    <picture>
                        <source srcset="/icon-dark.svg" media="(prefers-color-scheme: dark)">
                        <img src="/icon.svg" alt="` + stationName + ` brand icon" class="header-logo" loading="lazy">
                    </picture>
                    <div>
                        <h1 class="header-title">` + stationName + ` Metadata</h1>
                        <p class="header-subtitle">Real-time metadata routing and synchronization</p>
                    </div>
                </div>
                <nav class="nav">
                    <a href="#overview" class="nav-link">Overview</a>
                    <a href="#inputs-section" class="nav-link">Inputs</a>
                    <a href="#outputs-section" class="nav-link">Outputs</a>
                </nav>
            </div>
        </header>

        <main role="main">
            <section id="overview" class="mb-10">
                <h2 class="section-title">Overview</h2>
                <div id="stats" class="grid-stats">
                    <div class="stat-card">
                        <div class="stat-value text-brand" id="total-inputs">-</div>
                        <div class="stat-label">Total Inputs</div>
                    </div>
                    <div class="stat-card">
                        <div class="stat-value text-success" id="available-inputs">-</div>
                        <div class="stat-label">Available Inputs</div>
                    </div>
                    <div class="stat-card">
                        <div class="stat-value text-brand" id="total-outputs">-</div>
                        <div class="stat-label">Total Outputs</div>
                    </div>
                    <div class="stat-card">
                        <div class="stat-value text-brand" id="active-flows">-</div>
                        <div class="stat-label">Active Flows</div>
                    </div>
                </div>
            </section>

            <section id="inputs-section" class="mb-10">
                <h2 class="section-title">Inputs</h2>
                <div id="inputs-grid" class="grid-cards">
                    <div class="text-center py-12 text-muted">Loading inputs...</div>
                </div>
            </section>

            <section id="outputs-section" class="mb-10">
                <h2 class="section-title">Outputs</h2>
                <div id="outputs-grid" class="grid-cards">
                    <div class="text-center py-12 text-muted">Loading outputs...</div>
                </div>
            </section>
        </main>

        <footer class="site-footer" role="contentinfo">
            <div class="footer-content">
                <div class="grid-footer">
                    <div class="footer-brand">
                        <h3>` + stationName + ` Metadata</h3>
                        <div class="connection-indicator" id="connection-indicator">
                            <svg id="plug-icon" class="plug-icon" fill="currentColor" viewBox="0 0 100 100">
                                <path d="M30,5 L30,25 L40,25 L40,5 L30,5 z M60,5 L60,25 L70,25 L70,5 L60,5 z M25,20 C22.239,20 20,22.239,20,25 L20,70 C20,72.761 22.239,75 25,75 L40,75 L40,95 L60,95 L60,75 L75,75 C77.761,75 80,72.761 80,70 L80,25 C80,22.239 77.761,20 75,20 L70,20 L70,25 L60,25 L60,20 L40,20 L40,25 L30,25 L30,20 L25,20 z"/>
                            </svg>
                            <span id="connection-status" class="connection-status">Connecting</span>
                        </div>
                    </div>

                    <div class="hidden-mobile"></div>

                    <div class="footer-links">
                        <div class="footer-links-row">
                            <a href="https://github.com/oszuidwest/zwfm-metadata" target="_blank">
                                <svg fill="currentColor" viewBox="0 0 24 24">
                                    <path d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"></path>
                                </svg>
                                GitHub
                            </a>
                            <span class="footer-divider">|</span>
                            <span class="text-muted">Version <span id="app-version" class="font-medium">` + version + `</span></span>
                        </div>
                        <div class="footer-copyright">
                            © ` + buildYear + ` Streekomroep ZuidWest • MIT License
                        </div>
                    </div>
                </div>
            </div>
        </footer>
    </div>

    <script>` + jsContent + `</script>
</body>
</html>`
}
