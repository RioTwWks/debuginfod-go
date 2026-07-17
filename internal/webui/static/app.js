(function () {
  const statsGrid = document.getElementById("stats-grid");
  const scanInfo = document.getElementById("scan-info");
  const uptimeEl = document.getElementById("uptime");
  const searchForm = document.getElementById("search-form");
  const searchInput = document.getElementById("search-input");
  const searchHint = document.getElementById("search-hint");
  const searchStatus = document.getElementById("search-status");
  const resultsTable = document.getElementById("results-table");
  const resultsBody = document.getElementById("results-body");
  const loadMoreBtn = document.getElementById("load-more");
  const modeButtons = document.querySelectorAll(".mode-btn");
  const mainTabs = document.querySelectorAll(".main-tab");
  const tabDashboard = document.getElementById("tab-dashboard");
  const tabScans = document.getElementById("tab-scans");
  const indexSummary = document.getElementById("index-summary");
  const dedupSummary = document.getElementById("dedup-summary");
  const dedupStatus = document.getElementById("dedup-status");
  const scanRunsBody = document.getElementById("scan-runs-body");
  const dedupProjectsBody = document.getElementById("dedup-projects-body");
  const dedupRunsBody = document.getElementById("dedup-runs-body");

  let searchKey = "buildid";
  let nextOffset = 0;
  let lastSearchValue = "";
  let scansLoaded = false;

  const hints = {
    buildid:
      "Пустой запрос — первые 50 build-id. Ссылки скачивания только для типов, реально присутствующих в индексе.",
    glob: "Шаблон fnmatch, как в /metadata: /usr/bin/*, /usr/lib/debug/**",
    file: "Точный путь файла, как в /metadata?key=file&value=…",
  };

  const placeholders = {
    buildid: "Префикс build-id (hex), например deadbeef",
    glob: "Шаблон пути, например /usr/bin/*",
    file: "Абсолютный путь, например /usr/bin/ls",
  };

  function formatNumber(n) {
    return new Intl.NumberFormat("ru-RU").format(n);
  }

  function formatBytes(bytes) {
    if (!bytes || bytes === 0) return "0 B";
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

  function formatMs(ms) {
    if (ms < 1000) return ms + " ms";
    return (ms / 1000).toFixed(1) + " с";
  }

  function formatDate(iso) {
    if (!iso) return "—";
    try {
      return new Date(iso).toLocaleString("ru-RU");
    } catch (_) {
      return iso;
    }
  }

  function escapeHtml(s) {
    const div = document.createElement("div");
    div.textContent = s;
    return div.innerHTML;
  }

  function artifactLinks(buildid, types) {
    const available = (types || []).filter(function (t) {
      return t === "debuginfo" || t === "executable";
    });
    if (!available.length) {
      return '<span class="muted">—</span>';
    }
    return available
      .map(function (t) {
        return (
          '<a class="type-badge ' +
          t +
          '" href="/buildid/' +
          encodeURIComponent(buildid) +
          "/" +
          t +
          '" download>' +
          escapeHtml(t) +
          "</a>"
        );
      })
      .join("");
  }

  function setMainTab(tab) {
    mainTabs.forEach(function (btn) {
      const active = btn.dataset.tab === tab;
      btn.classList.toggle("active", active);
      btn.setAttribute("aria-selected", active ? "true" : "false");
    });
    tabDashboard.classList.toggle("active", tab === "dashboard");
    tabDashboard.hidden = tab !== "dashboard";
    tabScans.classList.toggle("active", tab === "scans");
    tabScans.hidden = tab !== "scans";
    if (tab === "scans" && !scansLoaded) {
      loadScans();
    }
  }

  function setSearchMode(key) {
    searchKey = key;
    nextOffset = 0;
    modeButtons.forEach(function (btn) {
      const active = btn.dataset.key === key;
      btn.classList.toggle("active", active);
      btn.setAttribute("aria-selected", active ? "true" : "false");
    });
    searchInput.placeholder = placeholders[key] || "";
    searchHint.textContent = hints[key] || "";
    searchInput.value = "";
    loadMoreBtn.hidden = true;
    doSearch("", false);
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

  async function loadScans() {
    try {
      const res = await fetch("/ui/api/scans?limit=50");
      if (!res.ok) throw new Error("HTTP " + res.status);
      const data = await res.json();
      renderScans(data);
      scansLoaded = true;
    } catch (err) {
      indexSummary.textContent = "Ошибка: " + err.message;
      dedupSummary.textContent = "Ошибка загрузки";
      if (dedupStatus) dedupStatus.textContent = "";
      scanRunsBody.innerHTML =
        '<tr><td colspan="8" class="muted">Ошибка загрузки</td></tr>';
      dedupRunsBody.innerHTML =
        '<tr><td colspan="9" class="muted">Ошибка загрузки</td></tr>';
      if (dedupProjectsBody) {
        dedupProjectsBody.innerHTML =
          '<tr><td colspan="7" class="muted">Ошибка загрузки</td></tr>';
      }
    }
  }

  function renderScans(data) {
    const idx = data.index_summary || {};
    indexSummary.innerHTML = [
      summaryItem(formatNumber(idx.artifacts_total), "артефактов"),
      summaryItem(formatNumber(idx.artifacts_executable), "executable"),
      summaryItem(formatNumber(idx.artifacts_debuginfo), "debuginfo"),
      summaryItem(formatNumber(idx.scanned_files_total), "файлов"),
      summaryItem(formatBytes(idx.bytes_on_disk), "на диске"),
    ].join("");

    const dedupEnabled = !!data.dedup_enabled;
    const t = data.dedup_totals || {};
    const savedPct =
      t.saved_percent > 0 ? t.saved_percent.toFixed(1) + "%" : "—";

    if (!dedupEnabled) {
      dedupSummary.innerHTML =
        '<div class="summary-item muted"><span>Dedup выключен (DEBUGINFOD_DEDUP_ENABLED=false)</span></div>';
      if (dedupStatus) {
        dedupStatus.textContent =
          "Включите DEBUGINFOD_DEDUP_ENABLED=true для сжатия .debug через xdelta.";
      }
    } else {
      dedupSummary.innerHTML = [
        summaryItem(formatNumber(t.files_done), "файлов dedup"),
        summaryItem(formatNumber(t.files_delta), "delta-файлов"),
        summaryItem(formatBytes(t.bytes_original), "исходный объём"),
        summaryItem(formatBytes(t.bytes_on_disk), "на диске сейчас"),
        summaryItem(formatBytes(t.bytes_saved) + " (" + savedPct + ")", "сэкономлено"),
      ].join("");
      if (dedupStatus) dedupStatus.textContent = "";
    }

    renderDedupByProject(data, dedupEnabled);

    if (!data.index_scans || data.index_scans.length === 0) {
      scanRunsBody.innerHTML =
        '<tr><td colspan="8" class="muted">Нет записей (ожидается после первого scan)</td></tr>';
    } else {
      scanRunsBody.innerHTML = data.index_scans
        .map(function (r) {
          return (
            "<tr>" +
            "<td>" + escapeHtml(formatDate(r.finished_at)) + "</td>" +
            "<td>" + escapeHtml(formatMs(r.duration_ms)) + "</td>" +
            "<td>" + formatNumber(r.indexed) + "</td>" +
            "<td>" + formatNumber(r.skipped) + "</td>" +
            "<td>" + formatNumber(r.errors) + "</td>" +
            "<td>" + formatNumber(r.artifacts_total) + "</td>" +
            "<td>" + formatNumber(r.scanned_files) + "</td>" +
            "<td>" + escapeHtml(formatBytes(r.bytes_on_disk)) + "</td>" +
            "</tr>"
          );
        })
        .join("");
    }

    if (!dedupEnabled) {
      dedupRunsBody.innerHTML =
        '<tr><td colspan="9" class="muted">Dedup выключен</td></tr>';
    } else if (!data.dedup_runs || data.dedup_runs.length === 0) {
      dedupRunsBody.innerHTML =
        '<tr><td colspan="9" class="muted">Нет записей (ожидается после первого dedup-прогона)</td></tr>';
    } else {
      dedupRunsBody.innerHTML = data.dedup_runs
        .map(function (r) {
          const saved =
            r.bytes_saved > 0
              ? formatBytes(r.bytes_saved) +
                " (" +
                (r.saved_percent || 0).toFixed(1) +
                "%)"
              : "—";
          return (
            "<tr>" +
            "<td>" + escapeHtml(formatDate(r.finished_at)) + "</td>" +
            "<td>" + escapeHtml(formatMs(r.duration_ms)) + "</td>" +
            "<td>" + escapeHtml(r.project || "все") + "</td>" +
            "<td>" + formatNumber(r.files_compressed) + "</td>" +
            "<td>" + formatNumber(r.files_skipped) + "</td>" +
            "<td>" + formatNumber(r.errors) + "</td>" +
            "<td>" + formatBytes(r.bytes_before) + "</td>" +
            "<td>" + formatBytes(r.bytes_after) + "</td>" +
            "<td>" + saved + "</td>" +
            "</tr>"
          );
        })
        .join("");
    }
  }

  function renderDedupByProject(data, dedupEnabled) {
    if (!dedupProjectsBody) return;
    if (!dedupEnabled) {
      dedupProjectsBody.innerHTML =
        '<tr><td colspan="7" class="muted">Dedup выключен</td></tr>';
      return;
    }
    const rows = (data.dedup_by_project || []).slice().sort(function (a, b) {
      return String(a.project).localeCompare(String(b.project), "ru");
    });
    if (!rows.length) {
      dedupProjectsBody.innerHTML =
        '<tr><td colspan="7" class="muted">Нет данных (ожидается после scan с каталогами build_*)</td></tr>';
      return;
    }
    dedupProjectsBody.innerHTML = rows
      .map(function (r) {
        const saved =
          r.bytes_saved > 0
            ? formatBytes(r.bytes_saved) +
              " (" +
              (r.saved_percent || 0).toFixed(1) +
              "%)"
            : "—";
        return (
          "<tr>" +
          '<td class="mono">' + escapeHtml(r.project) + "</td>" +
          "<td>" + formatNumber(r.build_dirs) + "</td>" +
          "<td>" + formatNumber(r.files_done) + "</td>" +
          "<td>" + formatNumber(r.files_delta) + "</td>" +
          "<td>" + formatBytes(r.bytes_original) + "</td>" +
          "<td>" + formatBytes(r.bytes_on_disk) + "</td>" +
          "<td>" + saved + "</td>" +
          "</tr>"
        );
      })
      .join("");
  }

  function summaryItem(value, label) {
    return (
      '<div class="summary-item"><strong>' +
      escapeHtml(String(value)) +
      "</strong><span>" +
      escapeHtml(label) +
      "</span></div>"
    );
  }

  function buildSearchParams(value, append) {
    const params = new URLSearchParams();
    params.set("key", searchKey);
    if (searchKey === "buildid") {
      if (value) params.set("q", value);
    } else {
      params.set("value", value);
      if (append && nextOffset > 0) {
        params.set("offset", String(nextOffset));
      }
    }
    return params;
  }

  async function doSearch(query, append) {
    if (!append) {
      nextOffset = 0;
      lastSearchValue = query;
      searchStatus.textContent = "Поиск…";
      searchStatus.classList.remove("error");
      resultsTable.hidden = true;
      loadMoreBtn.hidden = true;
    } else {
      loadMoreBtn.disabled = true;
      loadMoreBtn.textContent = "Загрузка…";
    }

    try {
      const params = buildSearchParams(append ? lastSearchValue : query, append);
      const res = await fetch("/ui/api/search?" + params.toString());
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || "HTTP " + res.status);
      }
      const data = await res.json();
      renderResults(data, append);
    } catch (err) {
      searchStatus.textContent = "Ошибка поиска: " + err.message;
      searchStatus.classList.add("error");
      loadMoreBtn.hidden = true;
    } finally {
      loadMoreBtn.disabled = false;
      loadMoreBtn.textContent = "Ещё результаты";
    }
  }

  function renderGroupedRow(row) {
    const types = row.types || [];
    const typesLabel = types.map(escapeHtml).join(", ");
    const file = row.file || "—";
    return (
      "<tr>" +
      '<td class="mono">' + escapeHtml(row.buildid) + "</td>" +
      "<td>" + typesLabel + "</td>" +
      '<td class="mono">' + escapeHtml(file) + "</td>" +
      "<td>" + escapeHtml(row.buildid_kind || "—") + "</td>" +
      '<td class="links">' + artifactLinks(row.buildid, types) + "</td>" +
      "</tr>"
    );
  }

  function renderFlatRow(row) {
    const types = [row.type];
    const file = row.archive ? row.archive + " → " + row.file : row.file;
    return (
      "<tr>" +
      '<td class="mono">' + escapeHtml(row.buildid) + "</td>" +
      "<td>" + escapeHtml(row.type) + "</td>" +
      '<td class="mono">' + escapeHtml(file) + "</td>" +
      "<td>" + escapeHtml(row.buildid_kind || "—") + "</td>" +
      '<td class="links">' + artifactLinks(row.buildid, types) + "</td>" +
      "</tr>"
    );
  }

  function renderResults(data, append) {
    let label = "";
    if (searchKey === "buildid") {
      label = data.query ? ' по «' + data.query + '»' : "";
    } else {
      label = data.value ? " (" + searchKey + ": «" + data.value + "»)" : "";
    }

    const rows = data.grouped && data.grouped.length ? data.grouped : data.results || [];
    const totalShown = append
      ? resultsBody.querySelectorAll("tr").length + rows.length
      : rows.length;

    let status = "Найдено: " + formatNumber(append ? totalShown : data.count) + label;
    if (!data.complete) {
      status += " (есть ещё — нажмите «Ещё результаты»)";
    }
    searchStatus.textContent = status;

    if (!rows.length) {
      if (!append) resultsTable.hidden = true;
      loadMoreBtn.hidden = true;
      return;
    }

    const html = rows
      .map(function (row) {
        return data.grouped ? renderGroupedRow(row) : renderFlatRow(row);
      })
      .join("");

    if (append) {
      resultsBody.insertAdjacentHTML("beforeend", html);
    } else {
      resultsBody.innerHTML = html;
    }

    resultsTable.hidden = false;
    nextOffset = data.next_offset || 0;
    loadMoreBtn.hidden = data.complete || !nextOffset;
  }

  mainTabs.forEach(function (btn) {
    btn.addEventListener("click", function () {
      setMainTab(btn.dataset.tab);
    });
  });

  modeButtons.forEach(function (btn) {
    btn.addEventListener("click", function () {
      setSearchMode(btn.dataset.key);
    });
  });

  searchForm.addEventListener("submit", function (e) {
    e.preventDefault();
    doSearch(searchInput.value.trim(), false);
  });

  loadMoreBtn.addEventListener("click", function () {
    doSearch(lastSearchValue, true);
  });

  let debounce;
  searchInput.addEventListener("input", function () {
    clearTimeout(debounce);
    debounce = setTimeout(function () {
      doSearch(searchInput.value.trim(), false);
    }, 350);
  });

  loadStats();
  setInterval(loadStats, 30000);
  setMainTab("dashboard");
  doSearch("", false);
})();
