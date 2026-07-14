(function () {
  const statsGrid = document.getElementById("stats-grid");
  const scanInfo = document.getElementById("scan-info");
  const uptimeEl = document.getElementById("uptime");
  const searchForm = document.getElementById("search-form");
  const searchInput = document.getElementById("search-input");
  const searchStatus = document.getElementById("search-status");
  const resultsTable = document.getElementById("results-table");
  const resultsBody = document.getElementById("results-body");

  function formatNumber(n) {
    return new Intl.NumberFormat("ru-RU").format(n);
  }

  function formatBytes(bytes) {
    if (bytes === 0) return "0 B";
    const units = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(1) + " " + units[i];
  }

  function formatDuration(seconds) {
    if (seconds < 60) return seconds + " с";
    if (seconds < 3600) return Math.floor(seconds / 60) + " мин";
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    return h + " ч " + m + " мин";
  }

  function escapeHtml(s) {
    const div = document.createElement("div");
    div.textContent = s;
    return div.innerHTML;
  }

  async function loadStats() {
    try {
      const res = await fetch("/ui/api/stats");
      if (!res.ok) throw new Error("HTTP " + res.status);
      const data = await res.json();
      renderStats(data);
    } catch (err) {
      statsGrid.innerHTML =
        '<div class="stat-card loading"><span class="stat-label">Ошибка загрузки статистики</span></div>';
    }
  }

  function renderStats(data) {
    uptimeEl.textContent = "uptime " + formatDuration(data.uptime_seconds);

    const cards = [
      { label: "Артефакты", value: data.artifacts_total, highlight: true },
      { label: "Executable", value: data.artifacts_executable },
      { label: "Debuginfo", value: data.artifacts_debuginfo },
      { label: "Исходники", value: data.sources_total },
      { label: "Просканировано файлов", value: data.scanned_files_total },
      { label: "HTTP запросов", value: data.http_requests_total },
      { label: "Кэш", value: formatBytes(data.cache_bytes) },
    ];

    statsGrid.innerHTML = cards
      .map(function (c) {
        const cls = c.highlight ? "stat-card highlight" : "stat-card";
        const val =
          typeof c.value === "number" ? formatNumber(c.value) : c.value;
        return (
          '<div class="' +
          cls +
          '"><span class="stat-value">' +
          escapeHtml(String(val)) +
          '</span><span class="stat-label">' +
          escapeHtml(c.label) +
          "</span></div>"
        );
      })
      .join("");

    const scanParts = [
      "<span class='scan-item'><strong>" +
        formatNumber(data.last_scan_indexed) +
        "</strong> <span>проиндексировано</span></span>",
      "<span class='scan-item'><strong>" +
        formatNumber(data.last_scan_skipped) +
        "</strong> <span>пропущено</span></span>",
      "<span class='scan-item'><strong>" +
        formatNumber(data.last_scan_errors) +
        "</strong> <span>ошибок</span></span>",
      "<span class='scan-item'><strong>" +
        formatNumber(data.last_scan_duration_ms) +
        " ms</strong> <span>длительность</span></span>",
    ];
    if (data.last_scan_finished_at) {
      scanParts.push(
        "<span class='scan-item'><strong>" +
          escapeHtml(data.last_scan_finished_at) +
          "</strong> <span>завершено</span></span>"
      );
    }
    scanInfo.innerHTML = scanParts.join("");
  }

  async function doSearch(query) {
    searchStatus.textContent = "Поиск…";
    searchStatus.classList.remove("error");
    resultsTable.hidden = true;

    try {
      const params = new URLSearchParams();
      if (query) params.set("q", query);
      const res = await fetch("/ui/api/search?" + params.toString());
      if (!res.ok) throw new Error("HTTP " + res.status);
      const data = await res.json();
      renderResults(data);
    } catch (err) {
      searchStatus.textContent = "Ошибка поиска: " + err.message;
      searchStatus.classList.add("error");
    }
  }

  function renderResults(data) {
    const q = data.query ? ' по «' + data.query + '»' : "";
    searchStatus.textContent =
      "Найдено: " + formatNumber(data.count) + q;

    if (!data.results || data.results.length === 0) {
      resultsTable.hidden = true;
      return;
    }

    resultsBody.innerHTML = data.results
      .map(function (row) {
        const typeCls =
          row.type === "executable" ? "executable" : "debuginfo";
        const file = row.archive
          ? row.archive + " → " + row.file
          : row.file;
        const links =
          '<a href="/buildid/' +
          encodeURIComponent(row.buildid) +
          '/debuginfo">debuginfo</a>' +
          '<a href="/buildid/' +
          encodeURIComponent(row.buildid) +
          '/executable">executable</a>';
        return (
          "<tr>" +
          '<td class="mono">' +
          escapeHtml(row.buildid) +
          "</td>" +
          '<td><span class="type-badge ' +
          typeCls +
          '">' +
          escapeHtml(row.type) +
          "</span></td>" +
          '<td class="mono">' +
          escapeHtml(file) +
          "</td>" +
          "<td>" +
          escapeHtml(row.buildid_kind || "—") +
          "</td>" +
          '<td class="links">' +
          links +
          "</td>" +
          "</tr>"
        );
      })
      .join("");

    resultsTable.hidden = false;
  }

  searchForm.addEventListener("submit", function (e) {
    e.preventDefault();
    doSearch(searchInput.value.trim());
  });

  let debounce;
  searchInput.addEventListener("input", function () {
    clearTimeout(debounce);
    debounce = setTimeout(function () {
      doSearch(searchInput.value.trim());
    }, 350);
  });

  loadStats();
  setInterval(loadStats, 30000);
  doSearch("");
})();
