document.addEventListener('DOMContentLoaded', () => {
    const state = { sessionToken: '', currentPath: '', libraries: [], readingDirection: 'rtl' };
    const $ = id => document.getElementById(id);
    const loginScreen = $('login-screen'), appContainer = $('app-container'), loginForm = $('login-form');
    const fileBrowser = $('file-browser'), folderHeader = $('folder-header');

    function showLoading(text) { $('loading-text').textContent = text; $('loading-overlay').style.display = 'flex'; }
    function hideLoading() { $('loading-overlay').style.display = 'none'; }

    function toast(text, type = 'error') {
        const el = document.createElement('div');
        el.className = `toast toast-${type}`;
        el.textContent = text;
        document.body.appendChild(el);
        setTimeout(() => el.remove(), 4000);
    }

    // Search
    $('search-input').addEventListener('input', e => {
        const q = e.target.value.toLowerCase();
        document.querySelectorAll('.file-item').forEach(el => {
            el.style.display = el.querySelector('.item-name')?.textContent.toLowerCase().includes(q) ? '' : 'none';
        });
    });

    // Login
    loginForm.addEventListener('submit', async e => {
        e.preventDefault(); showLoading('Signing in...');
        try {
            const resp = await fetch('/api/v1/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ password: $('password-input').value }) });
            if (!resp.ok) throw new Error('Incorrect password');
            state.sessionToken = (await resp.json()).token;
            loginScreen.style.display = 'none'; appContainer.style.display = 'block';
            hideLoading(); browseFiles('');
        } catch (err) { hideLoading(); $('login-error').textContent = err.message; }
    });

    async function fetchWithAuth(url, opts = {}) {
        const headers = { ...opts.headers, Authorization: `Bearer ${state.sessionToken}` };
        const resp = await fetch(url, { ...opts, headers });
        if (resp.status === 401) { toast('Session expired'); window.location.reload(); }
        return resp;
    }

    // Browse
    async function browseFiles(path) {
        state.currentPath = path; showLoading('Loading...');
        $('search-input').value = '';
        try {
            const data = await (await fetchWithAuth(`/api/v1/browse?path=${encodeURIComponent(path)}`)).json();
            const items = data.items || [];
            if (!path || path === '.') state.libraries = items.map(i => ({ friendly_name: i.friendly_name, path: i.path }));
            if (data.current_folder?.reading_direction) state.readingDirection = data.current_folder.reading_direction;
            renderBreadcrumb(path);
            renderFolderHeader(path, data.current_folder);
            renderFiles(items);
            hideLoading();
        } catch (err) { hideLoading(); fileBrowser.innerHTML = `<p style="color:#ef5350">Error: ${err.message}</p>`; }
    }
    // Expose for logo click
    window.suikaApp = { browseFiles };

    function getBackPath() {
        if (!state.currentPath || state.currentPath === '.') return null;
        const lib = state.libraries.find(l => state.currentPath === l.path);
        if (lib) return '';
        const parent = state.currentPath.substring(0, state.currentPath.lastIndexOf('/'));
        return parent || '';
    }

    function renderFolderHeader(path, folder) {
        const backPath = getBackPath();
        if (backPath === null) {
            folderHeader.innerHTML = '';
            return;
        }
        const name = folder?.name || path.split('/').pop() || '';
        folderHeader.innerHTML = `<div class="folder-header"><a href="#" class="back-btn" id="back-btn">&#8592;</a><h2>${name}</h2></div>`;
        $('back-btn')?.addEventListener('click', e => { e.preventDefault(); browseFiles(backPath); });
    }

    function renderFiles(items) {
        fileBrowser.innerHTML = items.map(item => {
            const id = `icon-${item.path.replace(/[^a-zA-Z0-9]/g, '-')}`;
            const m = item.metadata;
            const name = m?.title || item.friendly_name || item.name;
            let detail = '';
            if (item.page_count) detail += `<div class="item-pages">${item.page_count} pages</div>`;
            if (m?.writer) detail += `<div class="item-pages">${m.writer}</div>`;
            let comicDir = '';
            if (m?.manga === 'Yes' || m?.manga === 'YesAndRightToLeft') comicDir = 'rtl';
            else if (m?.manga === 'No') comicDir = 'ltr';
            let poster;
            if (item.name === '..') poster = `<div class="item-poster" id="${id}">&#8617;</div>`;
            else if (item.is_dir) poster = `<div class="item-poster" id="${id}">&#128193;</div>`;
            else poster = `<div class="item-poster" id="${id}">&#128214;</div>`;
            return `<div class="file-item${item.name === '..' ? ' back-item' : ''}" data-path="${item.path}" data-is-dir="${item.is_dir}" data-is-archive="${item.is_archive || false}" data-comicdir="${comicDir}">
                ${poster}
                <div class="item-info">
                    <div class="item-name" title="${name}">${name}</div>
                    ${detail}
                </div>
            </div>`;
        }).join('');

        items.forEach(item => {
            const src = item.icon;
            if (!src) return;
            const el = document.getElementById(`icon-${item.path.replace(/[^a-zA-Z0-9]/g, '-')}`);
            if (!el) return;
            fetchWithAuth(src).then(r => r.blob()).then(b => {
                el.innerHTML = `<img src="${URL.createObjectURL(b)}">`;
            }).catch(() => {});
        });
    }

    function renderBreadcrumb(path) {
        const bc = $('breadcrumb');
        if (!path || path === '.') { bc.innerHTML = '<a href="#" data-path="">Home</a>'; return; }
        let html = '<a href="#" data-path="">Home</a>';
        const lib = state.libraries.find(l => path.startsWith(l.path));
        if (lib) {
            html += ` / <a href="#" data-path="${lib.path}">${lib.friendly_name}</a>`;
            const sub = path.substring(lib.path.length).replace(/^\/|\/$/g, '');
            if (sub) { let cur = lib.path; sub.split('/').forEach(p => { cur += `/${p}`; html += ` / <a href="#" data-path="${cur}">${p}</a>`; }); }
        }
        bc.innerHTML = html;
    }

    // Manga Viewer
    let viewerGeneration = 0;

    async function openManga(archivePath, title, direction = 'rtl') {
        const myGeneration = ++viewerGeneration;
        document.querySelector('#suika-next-btn')?.remove();

        showLoading('Loading manga...');
        try {
            viewerClosed = false;
            const archiveId = btoa(archivePath).replace(/\+/g, '-').replace(/\//g, '_');

            // Fetch pages and siblings in parallel
            const [pagesResp, siblingsResp] = await Promise.all([
                fetchWithAuth(`/api/v1/manga/${archiveId}/pages`),
                fetchWithAuth(`/api/v1/manga/${archiveId}/siblings`)
            ]);
            const pagesData = await pagesResp.json();
            const siblings = await siblingsResp.json();

            const pageUrls = [];
            for (let i = 0; i < pagesData.count; i++) {
                pageUrls.push(`/api/v1/manga/${archiveId}/page/${i}?token=${encodeURIComponent(state.sessionToken)}`);
            }

            const container = $('manga-viewer-container');
            container.style.display = 'block';
            container.innerHTML = '<div id="mv-inner"></div>';

            hideLoading();

            console.log(`[suika] opening: ${title}, pages: ${pagesData.count}, urls: ${pageUrls.length}, direction: ${direction}`);

            const viewerOpts = {
                container: '#mv-inner',
                pages: pageUrls,
                title: title,
                direction: direction,
                showHeader: true,
                showFooter: true,
                storageKey: `suika_${archiveId}`,
                backUrl: '#close-manga'
            };

            // Track page changes, show next chapter button on last page
            const totalPages = pagesData.count;
            const nextChapter = siblings.next;
            viewerOpts.onPageChange = (cur, total) => {
                if (viewerClosed || myGeneration !== viewerGeneration) return;
                console.log(`[suika] page ${cur}/${total} (server: ${totalPages})`);
                if (!nextChapter) return;
                const isLastPage = cur >= total - 1 || cur >= totalPages - 1;
                const btn = document.querySelector('#suika-next-btn');
                if (isLastPage && !btn) {
                    const b = document.createElement('button');
                    b.id = 'suika-next-btn';
                    b.textContent = `Next: ${nextChapter.name} \u2192`;
                    b.style.cssText = 'position:fixed;bottom:80px;left:50%;transform:translateX(-50%);z-index:99999;padding:12px 24px;border-radius:8px;border:none;background:rgba(46,125,50,0.5);color:rgba(255,255,255,0.85);font-size:15px;font-weight:600;cursor:pointer;box-shadow:0 4px 16px rgba(0,0,0,0.3);backdrop-filter:blur(4px);transition:background 0.2s;';
                    b.onmouseenter = () => b.style.background = 'rgba(46,125,50,0.95)';
                    b.onmouseleave = () => b.style.background = 'rgba(46,125,50,0.5)';
                    b.onclick = () => {
                        b.remove();
                        closeMangaViewer();
                        openManga(nextChapter.path, nextChapter.name, direction);
                    };
                    document.body.appendChild(b);
                } else if (!isLastPage && btn) {
                    btn.remove();
                }
            };

            new window.MangaViewer(viewerOpts);

            // Listen for close
            const onHash = () => {
                if (window.location.hash === '#close-manga') {
                    closeMangaViewer();
                    window.removeEventListener('hashchange', onHash);
                }
            };
            window.addEventListener('hashchange', onHash);

        } catch (err) { hideLoading(); toast('Failed to open manga: ' + err.message); }
    }

    let viewerClosed = false;
    function closeMangaViewer() {
        viewerClosed = true;
        viewerGeneration++;
        const container = $('manga-viewer-container');
        container.style.display = 'none';
        container.innerHTML = '';
        document.querySelector('#suika-next-btn')?.remove();
        window.location.hash = '';
    }

    // Events
    fileBrowser.addEventListener('click', e => {
        const item = e.target.closest('.file-item');
        if (!item) return;
        if (item.dataset.isDir === 'true') {
            browseFiles(item.dataset.path);
        } else if (item.dataset.isArchive === 'true') {
            const name = item.querySelector('.item-name')?.textContent || '';
            // Priority: ComicInfo.xml manga field > library default
            const comicDir = item.dataset.comicdir;
            const direction = comicDir || state.readingDirection;
            openManga(item.dataset.path, name, direction);
        }
    });

    $('breadcrumb').addEventListener('click', e => { e.preventDefault(); if (e.target.tagName === 'A') browseFiles(e.target.dataset.path); });

    // View toggle
    window.setView = function(mode) {
        const grid = $('file-browser');
        $('view-grid').classList.toggle('active', mode === 'grid');
        $('view-list').classList.toggle('active', mode === 'list');
        grid.classList.toggle('list-view', mode === 'list');
        localStorage.setItem('suika-view', mode);
    };
    if (localStorage.getItem('suika-view') === 'list') {
        $('file-browser').classList.add('list-view');
        $('view-list')?.classList.add('active');
        $('view-grid')?.classList.remove('active');
    }

    // ESC to close manga viewer
    document.addEventListener('keydown', e => {
        if (e.key === 'Escape' && $('manga-viewer-container').style.display === 'block') {
            closeMangaViewer();
        }
    });
});
