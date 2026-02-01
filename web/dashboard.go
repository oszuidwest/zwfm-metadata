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
    <style>
/* ==========================================================================
   CSS Custom Properties - Design Tokens
   ========================================================================== */

:root {
    /* Brand colors */
    --color-brand: ` + brandColor + `;
    --color-brand-15: color-mix(in srgb, var(--color-brand) 15%, transparent);
    --color-brand-20: color-mix(in srgb, var(--color-brand) 20%, transparent);
    --color-brand-30: color-mix(in srgb, var(--color-brand) 30%, transparent);

    /* Semantic colors */
    --color-success: #10b981;
    --color-danger: #ef4444;
    --color-warning: #f59e0b;

    /* Gray palette - Light mode */
    --color-bg-page: #f3f4f6;
    --color-bg-card: #ffffff;
    --color-bg-subtle: #f9fafb;
    --color-bg-muted: rgba(0, 0, 0, 0.03);

    --color-border: rgba(229, 231, 235, 0.8);
    --color-border-muted: rgba(229, 231, 235, 0.7);
    --color-ring: #d1d5db;

    --color-text-primary: #111827;
    --color-text-secondary: #374151;
    --color-text-tertiary: #4b5563;
    --color-text-muted: #6b7280;
    --color-text-faint: #9ca3af;

    /* Card shadows - Light mode */
    --shadow-card: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -2px rgba(0, 0, 0, 0.1);
    --shadow-card-hover: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 8px 10px -6px rgba(0, 0, 0, 0.1);
    --shadow-sm: 0 1px 2px 0 rgba(0, 0, 0, 0.05);

    /* Header backgrounds */
    --color-header-slate: #334155;

    /* Spacing scale */
    --space-1: 0.25rem;
    --space-2: 0.5rem;
    --space-3: 0.75rem;
    --space-4: 1rem;
    --space-5: 1.25rem;
    --space-6: 1.5rem;
    --space-8: 2rem;
    --space-10: 2.5rem;
    --space-12: 3rem;
    --space-16: 4rem;

    /* Typography */
    --font-sans: ui-sans-serif, system-ui, sans-serif, "Apple Color Emoji", "Segoe UI Emoji";
    --font-mono: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;

    --text-xs: 0.75rem;
    --text-sm: 0.875rem;
    --text-base: 1rem;
    --text-lg: 1.125rem;
    --text-xl: 1.25rem;
    --text-2xl: 1.5rem;
    --text-3xl: 1.875rem;
    --text-5xl: 3rem;

    /* Border radius */
    --radius-sm: 0.25rem;
    --radius-md: 0.375rem;
    --radius-lg: 0.5rem;
    --radius-xl: 0.75rem;
    --radius-2xl: 1rem;
    --radius-full: 9999px;

    /* Transitions */
    --transition-fast: 150ms ease;
    --transition-base: 200ms ease;
    --transition-slow: 300ms ease;
}

/* Dark mode overrides */
@media (prefers-color-scheme: dark) {
    :root {
        --color-brand-15: color-mix(in srgb, var(--color-brand) 25%, transparent);
        --color-brand-20: color-mix(in srgb, var(--color-brand) 35%, transparent);
        --color-brand-30: color-mix(in srgb, var(--color-brand) 50%, transparent);

        --color-bg-page: #0f172a;
        --color-bg-card: #1e293b;
        --color-bg-subtle: rgba(15, 23, 42, 0.5);
        --color-bg-muted: rgba(255, 255, 255, 0.03);

        --color-border: #374151;
        --color-border-muted: #374151;
        --color-ring: #4b5563;

        --color-text-primary: #f3f4f6;
        --color-text-secondary: #e5e7eb;
        --color-text-tertiary: #d1d5db;
        --color-text-muted: #9ca3af;
        --color-text-faint: #6b7280;

        --shadow-card: 0 4px 6px -1px rgba(0, 0, 0, 0.3), 0 2px 4px -2px rgba(0, 0, 0, 0.3);
        --shadow-card-hover: 0 20px 25px -5px rgba(0, 0, 0, 0.4), 0 8px 10px -6px rgba(0, 0, 0, 0.4);

        --color-header-slate: #475569;
    }
}

/* ==========================================================================
   Base & Reset
   ========================================================================== */

*, *::before, *::after {
    box-sizing: border-box;
}

body {
    margin: 0;
    font-family: var(--font-sans);
    font-size: var(--text-base);
    line-height: 1.5;
    color: var(--color-text-primary);
    background-color: var(--color-bg-page);
    min-height: 100vh;
    transition: background-color var(--transition-base), color var(--transition-base);
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
}

/* ==========================================================================
   Layout
   ========================================================================== */

.container {
    max-width: 80rem;
    margin: 0 auto;
    padding: var(--space-5);
}

.grid-stats {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: var(--space-3);
}

.grid-cards {
    display: grid;
    grid-template-columns: 1fr;
    gap: var(--space-5);
}

.grid-2-col {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: var(--space-4);
}

.grid-footer {
    display: grid;
    grid-template-columns: 1fr;
    gap: var(--space-8);
    align-items: center;
}

@media (min-width: 640px) {
    .grid-stats { gap: var(--space-5); }
}

@media (min-width: 768px) {
    .grid-footer { grid-template-columns: 1fr 1fr 1fr; }
}

@media (min-width: 1024px) {
    .grid-stats { grid-template-columns: repeat(4, 1fr); }
    .grid-cards { grid-template-columns: repeat(2, 1fr); }
}

@media (min-width: 1280px) {
    .grid-cards { grid-template-columns: repeat(3, 1fr); }
}

/* Flexbox utilities */
.flex { display: flex; }
.flex-col { flex-direction: column; }
.flex-wrap { flex-wrap: wrap; }
.items-center { align-items: center; }
.justify-center { justify-content: center; }
.justify-between { justify-content: space-between; }
.gap-1 { gap: var(--space-1); }
.gap-2 { gap: var(--space-2); }
.gap-3 { gap: var(--space-3); }
.gap-4 { gap: var(--space-4); }
.gap-5 { gap: var(--space-5); }

/* ==========================================================================
   Typography
   ========================================================================== */

.text-xs { font-size: var(--text-xs); }
.text-sm { font-size: var(--text-sm); }
.text-lg { font-size: var(--text-lg); }
.text-xl { font-size: var(--text-xl); }
.text-2xl { font-size: var(--text-2xl); }
.text-3xl { font-size: var(--text-3xl); }

.font-medium { font-weight: 500; }
.font-semibold { font-weight: 600; }
.font-bold { font-weight: 700; }
.font-mono { font-family: var(--font-mono); }

.text-center { text-align: center; }

.tracking-tight { letter-spacing: -0.025em; }
.break-all { word-break: break-all; }

/* Text colors */
.text-brand { color: var(--color-brand); }
.text-success { color: var(--color-success); }
.text-warning { color: var(--color-warning); }
.text-danger { color: var(--color-danger); }
.text-muted { color: var(--color-text-muted); }
.text-muted-light { color: var(--color-text-muted); }
.text-muted-dark { color: var(--color-text-tertiary); }
.text-faint { color: var(--color-text-faint); }
.text-white { color: #ffffff; }

/* ==========================================================================
   Spacing
   ========================================================================== */

.mb-1 { margin-bottom: var(--space-1); }
.mb-2 { margin-bottom: var(--space-2); }
.mb-3 { margin-bottom: var(--space-3); }
.mb-4 { margin-bottom: var(--space-4); }
.mb-5 { margin-bottom: var(--space-5); }
.mb-10 { margin-bottom: var(--space-10); }
.mt-3 { margin-top: var(--space-3); }
.mt-4 { margin-top: var(--space-4); }
.mt-16 { margin-top: var(--space-16); }
.mr-2 { margin-right: var(--space-2); }

.p-4 { padding: var(--space-4); }
.p-6 { padding: var(--space-6); }
.px-4 { padding-left: var(--space-4); padding-right: var(--space-4); }
.py-8 { padding-top: var(--space-8); padding-bottom: var(--space-8); }
.py-12 { padding-top: var(--space-12); padding-bottom: var(--space-12); }
.pt-4 { padding-top: var(--space-4); }

.space-y-1 > * + * { margin-top: var(--space-1); }
.space-y-2 > * + * { margin-top: var(--space-2); }
.space-y-4 > * + * { margin-top: var(--space-4); }

/* ==========================================================================
   Borders & Dividers
   ========================================================================== */

.border-t {
    border-top: 1px solid var(--color-border);
}

.rounded-sm { border-radius: var(--radius-sm); }
.rounded-md { border-radius: var(--radius-md); }
.rounded-lg { border-radius: var(--radius-lg); }
.rounded-xl { border-radius: var(--radius-xl); }
.rounded-2xl { border-radius: var(--radius-2xl); }
.rounded-full { border-radius: var(--radius-full); }

/* ==========================================================================
   Header Component
   ========================================================================== */

.site-header {
    background: var(--color-bg-card);
    border: 1px solid var(--color-border-muted);
    border-radius: var(--radius-2xl);
    padding: var(--space-6);
    box-shadow: var(--shadow-card);
    position: relative;
    overflow: hidden;
}

@media (min-width: 640px) {
    .site-header { padding: 1.75rem; }
}

.site-header::before {
    content: '';
    position: absolute;
    inset: 0;
    background: linear-gradient(to bottom right, var(--color-bg-card) 0%, var(--color-bg-card) 50%, rgba(255,255,255,0.8));
    pointer-events: none;
}

@media (prefers-color-scheme: dark) {
    .site-header::before {
        background: linear-gradient(to bottom right, var(--color-bg-card) 0%, var(--color-bg-card) 50%, rgba(30,41,59,0.7));
    }
}

.site-header::after {
    content: '';
    position: absolute;
    inset: 0;
    opacity: 0.03;
    background: radial-gradient(circle at top, rgba(0,0,0,0.6) 0%, transparent 60%);
    pointer-events: none;
}

@media (prefers-color-scheme: dark) {
    .site-header::after {
        background: radial-gradient(circle at top, rgba(255,255,255,0.35) 0%, transparent 60%);
    }
}

.header-content {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
}

@media (min-width: 640px) {
    .header-content {
        flex-direction: row;
        align-items: center;
        justify-content: space-between;
    }
}

.header-brand {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
    text-align: center;
}

@media (min-width: 640px) {
    .header-brand {
        flex-direction: row;
        gap: var(--space-5);
        text-align: left;
    }
}

.header-logo {
    width: 3rem;
    height: 3rem;
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
    flex-shrink: 0;
}

@media (min-width: 640px) {
    .header-logo {
        width: 2.75rem;
        height: 2.75rem;
    }
}

.header-title {
    font-size: var(--text-3xl);
    font-weight: 600;
    letter-spacing: -0.025em;
    color: var(--color-brand);
    margin: 0;
}

@media (min-width: 640px) {
    .header-title { font-size: 2.35rem; }
}

.header-subtitle {
    color: var(--color-text-muted);
    font-size: var(--text-sm);
    margin: 0;
}

@media (min-width: 640px) {
    .header-subtitle { font-size: var(--text-base); }
}

/* ==========================================================================
   Navigation
   ========================================================================== */

.nav {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: center;
    gap: var(--space-3);
    font-size: var(--text-sm);
    font-weight: 500;
    color: var(--color-text-muted);
}

@media (min-width: 640px) {
    .nav { justify-content: flex-end; }
}

.nav-link {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: 0.4375rem var(--space-3);
    border-radius: var(--radius-lg);
    border: 1px solid var(--color-border);
    background: var(--color-bg-card);
    color: inherit;
    text-decoration: none;
    transition: border-color var(--transition-fast), color var(--transition-fast), box-shadow var(--transition-fast);
}

.nav-link:hover {
    border-color: var(--color-brand);
    color: var(--color-brand);
    box-shadow: var(--shadow-sm);
}

/* ==========================================================================
   Section Titles
   ========================================================================== */

.section-title {
    font-size: var(--text-2xl);
    font-weight: 600;
    margin: 0 0 var(--space-5) 0;
    color: var(--color-text-secondary);
}

.section-label {
    font-size: var(--text-sm);
    font-weight: 600;
    margin-bottom: var(--space-2);
    color: var(--color-text-secondary);
}

/* ==========================================================================
   Cards
   ========================================================================== */

.card {
    background: var(--color-bg-card);
    border-radius: var(--radius-xl);
    box-shadow: var(--shadow-card);
    overflow: hidden;
    transition: box-shadow var(--transition-base), transform var(--transition-base);
}

.card:hover {
    box-shadow: var(--shadow-card-hover);
}

.card-header {
    padding: var(--space-6);
    color: #ffffff;
}

.card-header-brand {
    background-color: var(--color-brand);
}

.card-header-slate {
    background-color: var(--color-header-slate);
}

.card-header .card-title-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.card-header h3 {
    font-size: var(--text-xl);
    font-weight: 700;
    margin: 0;
}

.card-body {
    padding: var(--space-6);
}

/* ==========================================================================
   Stat Cards
   ========================================================================== */

.stat-card {
    background: var(--color-bg-card);
    border-radius: var(--radius-xl);
    box-shadow: var(--shadow-card);
    padding: var(--space-4);
    text-align: center;
    transition: box-shadow var(--transition-base), transform var(--transition-base);
}

@media (min-width: 640px) {
    .stat-card { padding: var(--space-6); }
}

.stat-card:hover {
    box-shadow: var(--shadow-card-hover);
    transform: translateY(-4px);
}

.stat-value {
    font-size: var(--text-3xl);
    font-weight: 700;
    margin-bottom: var(--space-2);
}

@media (min-width: 640px) {
    .stat-value { font-size: var(--text-5xl); }
}

.stat-label {
    font-size: var(--text-sm);
    font-weight: 500;
    color: var(--color-text-secondary);
}

@media (min-width: 640px) {
    .stat-label { font-size: var(--text-lg); }
}

/* ==========================================================================
   Badges
   ========================================================================== */

.badge {
    display: inline-block;
    padding: 0.375rem 0.875rem;
    border-radius: var(--radius-full);
    font-size: var(--text-sm);
    box-shadow: inset 0 0 0 1px currentColor;
    transition: background-color var(--transition-fast);
}

.badge-input {
    background: var(--color-bg-subtle);
    color: var(--color-text-secondary);
    font-weight: 500;
    box-shadow: inset 0 0 0 1px var(--color-ring);
}

.badge-input:hover {
    background: var(--color-border);
}

.badge-brand {
    background: var(--color-brand-15);
    color: var(--color-brand);
    font-weight: 600;
    box-shadow: inset 0 0 0 1px var(--color-brand-30);
}

.badge-brand:hover {
    background: var(--color-brand-20);
}

.badge-type {
    padding: 0.375rem 1rem;
    background: rgba(255, 255, 255, 0.2);
    color: #ffffff;
    font-weight: 500;
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.4), 0 1px 2px rgba(0,0,0,0.1);
    backdrop-filter: blur(4px);
    -webkit-backdrop-filter: blur(4px);
}

@media (prefers-color-scheme: dark) {
    .badge-type {
        background: rgba(255, 255, 255, 0.1);
        box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.3), 0 1px 2px rgba(0,0,0,0.2);
    }
}

/* ==========================================================================
   Status Indicators
   ========================================================================== */

.status-dot {
    display: inline-block;
    width: 0.75rem;
    height: 0.75rem;
    border-radius: var(--radius-full);
    margin-right: var(--space-2);
}

.bg-success { background-color: var(--color-success); }
.bg-warning { background-color: var(--color-warning); }
.bg-danger { background-color: var(--color-danger); }

/* ==========================================================================
   Content Boxes
   ========================================================================== */

.content-box {
    background: var(--color-bg-subtle);
    padding: var(--space-4);
    border-radius: var(--radius-lg);
}

.content-box-bordered {
    background: var(--color-bg-subtle);
    padding: var(--space-4);
    border-radius: var(--radius-lg);
    border: 1px solid var(--color-border);
}

/* ==========================================================================
   Footer
   ========================================================================== */

.site-footer {
    margin-top: var(--space-16);
    border-top: 1px solid var(--color-border);
}

.footer-content {
    max-width: 80rem;
    margin: 0 auto;
    padding: var(--space-8) var(--space-4);
}

.footer-brand h3 {
    font-weight: 600;
    color: var(--color-text-primary);
    margin: 0 0 var(--space-1) 0;
    text-align: center;
}

@media (min-width: 768px) {
    .footer-brand h3 { text-align: left; }
}

.connection-indicator {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    justify-content: center;
}

@media (min-width: 768px) {
    .connection-indicator { justify-content: flex-start; }
}

.plug-icon {
    width: 0.75rem;
    height: 0.75rem;
    transition: all var(--transition-slow);
    color: var(--color-text-muted);
}

.connection-status {
    font-size: var(--text-xs);
    color: var(--color-text-muted);
}

.footer-links {
    text-align: center;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
}

@media (min-width: 768px) {
    .footer-links { text-align: right; }
}

.footer-links-row {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-3);
    font-size: var(--text-sm);
}

@media (min-width: 768px) {
    .footer-links-row { justify-content: flex-end; }
}

.footer-links a {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    color: var(--color-text-tertiary);
    text-decoration: none;
    transition: color var(--transition-fast);
}

.footer-links a:hover {
    color: var(--color-brand);
}

.footer-links svg {
    width: 1rem;
    height: 1rem;
}

.footer-divider {
    color: var(--color-border);
}

.footer-copyright {
    font-size: var(--text-xs);
    color: var(--color-text-faint);
}

/* ==========================================================================
   Animations
   ========================================================================== */

@keyframes flash {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.6; transform: scale(0.98); }
}

@keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
}

.animate-flash {
    animation: flash 0.5s ease-in-out;
}

.animate-pulse {
    animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
}

/* ==========================================================================
   Utility Classes (used by JavaScript)
   ========================================================================== */

.hidden { display: none; }
.hidden-mobile { display: none; }
@media (min-width: 768px) {
    .hidden-mobile { display: block; }
}
.inline-block { display: inline-block; }
.overflow-hidden { overflow: hidden; }
.w-3 { width: 0.75rem; }
.h-3 { height: 0.75rem; }
.w-4 { width: 1rem; }
.h-4 { height: 1rem; }

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
        let reconnectDelay = 1000;
        const maxReconnectDelay = 30000;

        // HTML Generation Helpers
        function escapeHtml(text) {
            const div = document.createElement('div');
            div.textContent = text;
            return div.innerHTML;
        }

        function createLabeledField(label, value, labelClass, valueClass) {
            labelClass = labelClass || 'text-muted-dark';
            valueClass = valueClass || 'font-semibold';
            return '<div class="mb-2"><span class="' + labelClass + '">' + label + ':</span> <span class="' + valueClass + '">' + escapeHtml(value) + '</span></div>';
        }

        function createBadge(text, classes) {
            return '<span class="badge ' + classes + '">' + escapeHtml(text) + '</span>';
        }

        function createMetadataCard(name, type, headerClass, content, hasChanged) {
            return '<div class="card" data-changed="' + hasChanged + '">' +
                '<div class="card-header ' + headerClass + '">' +
                    '<div class="card-title-row">' +
                        '<h3>' + escapeHtml(name) + '</h3>' +
                        createBadge(type, 'badge-type') +
                    '</div>' +
                '</div>' +
                '<div class="card-body">' + content + '</div>' +
            '</div>';
        }

        function createStatusBadge(status, statusConfig) {
            const config = statusConfig[status] || statusConfig.default;
            return '<div class="flex items-center mb-3">' +
                '<span class="status-dot ' + config.dot + '"></span>' +
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
            input: 'badge-input',
            formatter: 'badge-brand',
            filter: 'badge-brand',
            type: 'badge-type'
        };

        const CARD_CLASSES = {
            container: 'card',
            inputHeader: 'card-header-brand',
            outputHeader: 'card-header-slate'
        };

        // Data Management Helpers
        function updateContainerWithCards(container, html) {
            container.innerHTML = html;
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
                reconnectDelay = 1000;
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
                updateConnectionStatus('disconnected');

                if (reconnectTimeout) clearTimeout(reconnectTimeout);
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

            switch(status) {
                case 'connected':
                    statusText.textContent = 'Connected';
                    break;
                case 'disconnected':
                    statusText.textContent = 'Disconnected';
                    break;
                case 'connecting':
                    statusText.textContent = 'Connecting';
                    plugIcon.classList.add('animate-pulse');
                    statusText.classList.add('animate-pulse');
                    break;
            }
        }

        function formatDisplayTime(timestamp, useRelative) {
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

        function animateCardChange(element) {
            element.classList.add('animate-flash');
            setTimeout(() => {
                element.classList.remove('animate-flash');
            }, 500);
        }

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

        function updateInputCards(inputs) {
            const container = document.getElementById('inputs-grid');

            const html = inputs.map(input => {
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
                        metadataHtml = '<div class="content-box-bordered mt-4 font-mono text-sm break-all">' +
                                      fields.join('') +
                                      '</div>';
                    }
                }

                let prefixSuffixHtml = '';
                const parts = [];
                if (input.prefix && input.prefix !== 'undefined') {
                    parts.push(createLabeledField('Prefix', input.prefix, 'text-muted-light', 'font-mono'));
                }
                if (input.suffix && input.suffix !== 'undefined') {
                    parts.push(createLabeledField('Suffix', input.suffix, 'text-muted-light', 'font-mono'));
                }
                if (parts.length > 0) {
                    prefixSuffixHtml = '<div class="mt-3 space-y-1">' +
                        parts.map(p => '<div class="text-sm">' + p + '</div>').join('') +
                        '</div>';
                }

                let filterHtml = '';
                if (input.filters && input.filters.length > 0) {
                    const filterTags = input.filters
                        .map(filter => createBadge(filter, TAG_CLASSES.filter))
                        .join(' ');
                    filterHtml = '<div class="mt-3">' +
                        '<div class="section-label">Filters</div>' +
                        '<div class="flex flex-wrap gap-2">' + filterTags + '</div>' +
                        '</div>';
                }

                const prevInput = previousData.inputs[input.name];
                const hasChanged = hasDataChanged(input, prevInput, ['status', 'metadata']);

                const content = createStatusBadge(input.status, STATUS_CONFIG) +
                    prefixSuffixHtml +
                    filterHtml +
                    metadataHtml +
                    '<div class="text-muted-light text-sm mt-4 pt-4 border-t">' +
                        '<div>Updated: <span class="' + (input.status === 'available' ? 'font-medium' : '') + '">' +
                        formatDisplayTime(input.updatedAt, input.status === 'available') + '</span></div>' +
                        (input.expiresAt ? '<div>Expires: ' + formatDisplayTime(input.expiresAt) + '</div>' : '') +
                    '</div>';

                const card = createMetadataCard(input.name, input.type, CARD_CLASSES.inputHeader, content, hasChanged);
                return card.replace('data-changed=', 'data-input-name="' + input.name + '" data-changed=');
            }).join('');

            updateContainerWithCards(container, html);

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
                const inputTags = (output.inputs || [])
                    .map(input => createBadge(input, TAG_CLASSES.input))
                    .join(' ');

                const formatterTags = (output.formatters || [])
                    .map(formatter => createBadge(formatter, TAG_CLASSES.formatter))
                    .join(' ');

                const prevOutput = previousData.outputs[output.name];
                const hasChanged = hasDataChanged(output, prevOutput, ['currentInput']);

                const content =
                    '<div class="content-box grid-2-col mb-4">' +
                        '<div>' +
                            '<div class="text-muted-dark text-sm">Delay</div>' +
                            '<div class="font-bold text-lg">' + output.delay + 's</div>' +
                        '</div>' +
                        '<div>' +
                            '<div class="text-muted-dark text-sm">Current Input</div>' +
                            '<div class="font-bold text-lg ' + (output.currentInput ? 'text-success' : 'text-faint') + '">' +
                            escapeHtml(output.currentInput || 'None') + '</div>' +
                        '</div>' +
                    '</div>' +
                    '<div class="space-y-4">' +
                        '<div>' +
                            '<div class="section-label">Inputs (priority order)</div>' +
                            '<div class="flex flex-wrap gap-2">' + inputTags + '</div>' +
                        '</div>' +
                        (formatterTags ? '<div><div class="section-label">Formatters</div><div class="flex flex-wrap gap-2">' + formatterTags + '</div></div>' : '') +
                    '</div>';

                const card = createMetadataCard(output.name, output.type, CARD_CLASSES.outputHeader, content, hasChanged);
                return card.replace('data-changed=', 'data-output-name="' + output.name + '" data-changed=');
            }).join('');

            updateContainerWithCards(container, html);

            outputs.forEach(output => {
                previousData.outputs[output.name] = {
                    currentInput: output.currentInput
                };
            });
        }

        function processDashboardUpdate(data) {
            if (!data) return;

            updateStatistics(data);
            updateInputCards(data.inputs);
            updateOutputCards(data.outputs);
        }

        // Initialize
        updateConnectionStatus('connecting');
        establishWebSocketConnection();

        window.addEventListener('beforeunload', () => {
            if (reconnectTimeout) clearTimeout(reconnectTimeout);
            if (ws) ws.close();
        });
    `
}
