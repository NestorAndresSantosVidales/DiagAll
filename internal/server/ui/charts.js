/* ─── charts.js — DiagAll chart utilities ─────────────────────────────────── */

const CHART_DEFAULTS = {
    animation: false,
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
        legend: { display: false },
        tooltip: {
            backgroundColor: '#161b22',
            borderColor: '#30363d',
            borderWidth: 1,
            titleColor: '#e6edf3',
            bodyColor: '#8b949e',
        }
    },
    scales: {
        x: {
            grid: { color: 'rgba(48,54,61,.4)' },
            ticks: { color: '#8b949e', font: { size: 10, family: 'JetBrains Mono' } },
        },
        y: {
            grid: { color: 'rgba(48,54,61,.4)' },
            ticks: { color: '#8b949e', font: { size: 10, family: 'JetBrains Mono' } },
        }
    }
};

/* Rolling time-series line chart */
class TimeSeriesChart {
    constructor(canvasId, color = '#2f81f7', maxPoints = 60) {
        const ctx = document.getElementById(canvasId);
        this.maxPoints = maxPoints;
        this.labels = [];
        this.data = [];
        this.index = 0;

        this.chart = new Chart(ctx, {
            type: 'line',
            data: {
                labels: this.labels,
                datasets: [{
                    data: this.data,
                    borderColor: color,
                    backgroundColor: color.replace(')', ', 0.1)').replace('rgb', 'rgba'),
                    fill: true,
                    tension: 0.4,
                    pointRadius: 2,
                    pointBackgroundColor: color,
                    borderWidth: 2,
                }]
            },
            options: {
                ...CHART_DEFAULTS,
                scales: {
                    ...CHART_DEFAULTS.scales,
                    y: { ...CHART_DEFAULTS.scales.y, beginAtZero: true }
                }
            }
        });
    }

    addValue(val) {
        this.index++;
        this.labels.push(this.index);
        this.data.push(+val.toFixed(2));
        if (this.labels.length > this.maxPoints) {
            this.labels.shift();
            this.data.shift();
        }
        this.chart.update('none');
    }

    reset() {
        this.labels.length = 0;
        this.data.length = 0;
        this.index = 0;
        this.chart.update();
    }

    updateYLabel(label) {
        this.chart.options.scales.y.title = { display: true, text: label, color: '#8b949e', font: { size: 10 } };
        this.chart.update('none');
    }
}

/* Multi-stream performance chart */
class PerfChart {
    constructor(canvasId, maxPoints = 60) {
        const ctx = document.getElementById(canvasId);
        this.maxPoints = maxPoints;
        this.labels = [];
        this.index = 0;
        this.streams = {};

        const COLORS = ['#2f81f7', '#3fb950', '#a371f7', '#f85149', '#d29922', '#58a6ff', '#7ee787', '#ff9a3c'];
        this.COLORS = COLORS;

        this.chart = new Chart(ctx, {
            type: 'line',
            data: { labels: [], datasets: [] },
            options: {
                ...CHART_DEFAULTS,
                plugins: {
                    ...CHART_DEFAULTS.plugins,
                    legend: { display: true, labels: { color: '#8b949e', font: { size: 10 } } }
                },
                scales: {
                    ...CHART_DEFAULTS.scales,
                    y: { ...CHART_DEFAULTS.scales.y, beginAtZero: true }
                }
            }
        });
    }

    addInterval(mbps, streamId = 'total') {
        this.index++;
        if (this.labels.length >= this.maxPoints) {
            this.labels.shift();
            for (const ds of this.chart.data.datasets) {
                ds.data.shift();
            }
        }
        this.labels.push(this.index);

        // Ensure dataset for this stream
        if (!this.streams[streamId]) {
            const i = Object.keys(this.streams).length;
            const color = this.COLORS[i % this.COLORS.length];
            const ds = {
                label: streamId === 'total' ? 'Total' : `Stream ${streamId}`,
                data: Array(this.labels.length - 1).fill(null),
                borderColor: color,
                backgroundColor: 'transparent',
                fill: false,
                tension: 0.4,
                pointRadius: 2,
                borderWidth: 2,
            };
            this.chart.data.datasets.push(ds);
            this.streams[streamId] = ds;
        }

        // Pad all datasets to current length
        for (const [id, ds] of Object.entries(this.streams)) {
            if (id === streamId) {
                ds.data.push(+mbps.toFixed(2));
            } else if (ds.data.length < this.labels.length) {
                ds.data.push(null);
            }
        }

        this.chart.data.labels = this.labels;
        this.chart.update('none');
    }

    reset() {
        this.labels = [];
        this.index = 0;
        this.streams = {};
        this.chart.data.labels = [];
        this.chart.data.datasets = [];
        this.chart.update();
    }
}
