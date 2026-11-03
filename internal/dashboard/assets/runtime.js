/* isekai dashboard runtime. Fills a look's markup with the stats data.
   Data is only ever inserted as text (textContent) or as numbers in CSS
   variables: commit messages are untrusted. See docs/skins.md. */
(function () {
  'use strict';
  var DATA = JSON.parse(document.getElementById('isekai-data').textContent);
  var app = document.getElementById('app');
  var looks = Array.prototype.map.call(document.querySelectorAll('template[data-look]'), function (t) {
    return { id: t.getAttribute('data-look'), name: t.getAttribute('data-name'), desc: t.getAttribute('data-desc'), tpl: t };
  });
  var SAFE_ATTRS = { title: 1, 'aria-label': 1, alt: 1, 'aria-valuenow': 1 };

  function get(obj, path) {
    if (path === '' || path == null) return obj;
    var parts = String(path).split('.');
    var v = obj;
    for (var i = 0; i < parts.length; i++) {
      if (v == null) return undefined;
      v = v[parts[i]];
    }
    return v;
  }
  function resolve(path, scope) {
    path = String(path || '').trim();
    return path.charAt(0) === '.' ? get(scope, path.slice(1)) : get(DATA, path);
  }
  function fmt(v) {
    if (v == null) return '—';
    if (Array.isArray(v)) return String(v.length);
    if (typeof v === 'object') return '';
    if (typeof v === 'boolean') return v ? 'yes' : 'no';
    return String(v);
  }
  function truthy(v) {
    if (Array.isArray(v)) return v.length > 0;
    return !!v && v !== '0';
  }
  function token(v) {
    return String(v == null ? '' : v).toLowerCase().replace(/[^a-z0-9-]+/g, '-').slice(0, 40);
  }
  function pairs(spec) {
    return String(spec).split(';').map(function (p) {
      var i = p.indexOf(':');
      return i < 0 ? null : [p.slice(0, i).trim(), p.slice(i + 1).trim()];
    }).filter(Boolean);
  }
  function each(nodes, fn) { Array.prototype.forEach.call(nodes, fn); }

  function bind(root, scope) {
    each(root.querySelectorAll('[data-if]'), function (el) {
      var p = el.getAttribute('data-if'), neg = p.charAt(0) === '!';
      var v = truthy(resolve(neg ? p.slice(1) : p, scope));
      el.hidden = neg ? v : !v;
    });
    each(root.querySelectorAll('[data-bind]'), function (el) {
      el.textContent = fmt(resolve(el.getAttribute('data-bind'), scope));
    });
    each(root.querySelectorAll('[data-bind-attr]'), function (el) {
      pairs(el.getAttribute('data-bind-attr')).forEach(function (kv) {
        if (SAFE_ATTRS[kv[0]]) el.setAttribute(kv[0], fmt(resolve(kv[1], scope)));
      });
    });
    each(root.querySelectorAll('[data-bind-var]'), function (el) {
      pairs(el.getAttribute('data-bind-var')).forEach(function (kv) {
        var v = resolve(kv[1], scope);
        if (typeof v === 'boolean') v = v ? 1 : 0;
        if (kv[0].indexOf('--') === 0 && typeof v === 'number' && isFinite(v)) el.style.setProperty(kv[0], String(v));
      });
    });
    each(root.querySelectorAll('[data-bind-state]'), function (el) {
      el.setAttribute('data-state', token(resolve(el.getAttribute('data-bind-state'), scope)));
    });
    each(root.querySelectorAll('isekai-icon[data-name-from]'), function (el) {
      el.setAttribute('data-name', token(resolve(el.getAttribute('data-name-from'), scope)));
    });
    each(root.querySelectorAll('template[data-each]'), function (t) {
      var list = resolve(t.getAttribute('data-each'), scope);
      if (!Array.isArray(list)) return;
      var limit = parseInt(t.getAttribute('data-limit') || '0', 10) || list.length;
      var frag = document.createDocumentFragment();
      list.slice(0, limit).forEach(function (item, i) {
        var c = t.content.cloneNode(true);
        var wrap = document.createElement('div');
        wrap.appendChild(c);
        bind(wrap, typeof item === 'object' && item !== null ? Object.assign({ $index: i + 1 }, item) : { value: item, $index: i + 1 });
        while (wrap.firstChild) frag.appendChild(wrap.firstChild);
      });
      t.parentNode.insertBefore(frag, t);
    });
  }

  var SVGNS = 'http://www.w3.org/2000/svg';
  function svg(tag, attrs, parent) {
    var e = document.createElementNS(SVGNS, tag);
    for (var k in attrs) e.setAttribute(k, attrs[k]);
    if (parent) parent.appendChild(e);
    return e;
  }

  function radar(el) {
    var vals = resolve(el.getAttribute('data-values') || 'stats.radar', DATA) || [];
    var names = resolve(el.getAttribute('data-names') || 'stats.names', DATA) || [];
    var n = vals.length || 6, cx = 110, cy = 105, r = 80;
    var s = svg('svg', { viewBox: '0 0 220 215', role: 'img', 'aria-label': 'Attributes: ' + names.map(function (nm, i) { return nm + ' ' + vals[i]; }).join(', ') });
    function pt(i, f) {
      var a = (-90 + i * 360 / n) * Math.PI / 180;
      return (cx + r * f * Math.cos(a)).toFixed(1) + ',' + (cy + r * f * Math.sin(a)).toFixed(1);
    }
    [1, 0.66, 0.33].forEach(function (f) {
      var p = []; for (var i = 0; i < n; i++) p.push(pt(i, f));
      svg('polygon', { points: p.join(' '), class: 'radar-grid' }, s);
    });
    var p = []; for (var i = 0; i < n; i++) p.push(pt(i, Math.max(0.04, (vals[i] || 0) / 100)));
    svg('polygon', { points: p.join(' '), class: 'radar-value' }, s);
    names.forEach(function (nm, i) {
      var a = (-90 + i * 360 / n) * Math.PI / 180;
      var t = svg('text', { x: (cx + (r + 16) * Math.cos(a)).toFixed(1), y: (cy + (r + 16) * Math.sin(a) + 4).toFixed(1), 'text-anchor': 'middle', class: 'radar-label' }, s);
      t.textContent = nm;
    });
    el.textContent = '';
    el.appendChild(s);
  }

  var ICONS = {
    helm: 'M4 14a8 8 0 0 1 16 0v4H4z M8 14h8 M12 6v8',
    mail: 'M7 3 4 6v5l3 1v9h10v-9l3-1V6l-3-3-2 2H9z M12 5v16',
    blade: 'M14.5 3H21v6.5L10 20.5 3.5 14z M6 17l-3 3 M8 13l3 3',
    shield: 'M12 3 20 6v6c0 5-4 8-8 9-4-1-8-4-8-9V6z M9 9l6 6 M15 9l-6 6',
    amulet: 'M5 3c0 5 3 8 7 8s7-3 7-8 M12 11l5 5-5 6-5-6z',
    ring: 'M6 15a6 6 0 1 0 12 0a6 6 0 1 0-12 0 M9 6l3-3 3 3-3 3z',
    boots: 'M7 3h6v9l6 3v5H5V3z M5 16h14',
    raven: 'M3 12c4-1 6-5 10-5 3 0 5 2 5 4l3 1-3 1c0 4-4 6-8 6l-3 3v-4c-2-1-3-3-4-6z',
    recall: 'M4 5c3-1 6-1 8 1 2-2 5-2 8-1v14c-3-1-6-1-8 1-2-2-5-2-8-1z M12 6v14',
    never: 'M4 20c4-8 8-8 12-16 M14 14l6 6 M20 14l-6 6',
    quest: 'M6 3h11a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6z M9 12l2 2 4-4',
    oath: 'M6 9a6 6 0 1 0 12 0a6 6 0 1 0-12 0 M9 14l-2 7 5-3 5 3-2-7',
    summon: 'M3 4h18v16H3z M9 4v16 M15 4v16',
    star: 'M12 2l2.9 6.6 7.1.6-5.4 4.7 1.6 7-6.2-3.7-6.2 3.7 1.6-7L2 9.2l7.1-.6z'
  };
  function icon(el) {
    var d = ICONS[el.getAttribute('data-name')] || ICONS.star;
    var s = svg('svg', { viewBox: '0 0 24 24', 'aria-hidden': 'true', fill: 'none', stroke: 'currentColor', 'stroke-width': '1.8', 'stroke-linecap': 'round', 'stroke-linejoin': 'round' });
    svg('path', { d: d }, s);
    el.textContent = '';
    el.appendChild(s);
  }

  function hero(el) {
    var tpl = document.getElementById('isekai-hero-art');
    if (tpl) { el.textContent = ''; el.appendChild(tpl.content.cloneNode(true)); }
  }

  function prompt(desc) {
    return 'Change the isekai dashboard look for this repository.\n\nWhat I want: ' + (desc.trim() || '[DESCRIBE THE LOOK]') +
      '\n\nFollow docs/skins.md from why-isekai step by step: get the data with `isekai stats --format json`, start from a built-in skin with `isekai ui new`, bind data with data-* attributes (no scripts, no network, no copied game or brand art), preview at 1280 and 390 px, fix everything `isekai ui check` reports, then apply it with `isekai ui use` and commit the skin with a Why trailer quoting my request.';
  }

  function lookButton(el) {
    var b = document.createElement('button');
    b.type = 'button';
    b.className = 'isekai-look-button';
    b.textContent = 'Change look';
    b.addEventListener('click', openLooks);
    el.textContent = '';
    el.appendChild(b);
  }

  function openLooks() {
    var d = document.createElement('dialog');
    d.className = 'isekai-looks';
    var h = document.createElement('h2'); h.textContent = 'Change look'; d.appendChild(h);
    var p = document.createElement('p'); p.textContent = 'Same data, any look. Pick one, or describe your own and give the prompt to your agent.'; d.appendChild(p);
    var list = document.createElement('div'); list.className = 'isekai-looks-list';
    looks.forEach(function (l) {
      var b = document.createElement('button');
      b.type = 'button';
      b.setAttribute('aria-pressed', String(l.id === current));
      var n = document.createElement('strong'); n.textContent = l.name; b.appendChild(n);
      var s = document.createElement('span'); s.textContent = l.desc || ''; b.appendChild(s);
      b.addEventListener('click', function () { d.close(); d.remove(); render(l.id); });
      list.appendChild(b);
    });
    d.appendChild(list);
    var lab = document.createElement('label'); lab.textContent = 'Describe your own';
    var ta = document.createElement('textarea'); ta.rows = 3;
    ta.placeholder = 'e.g. dark fantasy MOBA HUD: hero portrait at the bottom, skills on Q W E R, items in a 3x2 grid';
    lab.appendChild(ta); d.appendChild(lab);
    var pre = document.createElement('pre'); pre.textContent = prompt(''); d.appendChild(pre);
    ta.addEventListener('input', function () { pre.textContent = prompt(ta.value); });
    var row = document.createElement('div'); row.className = 'isekai-looks-actions';
    var copy = document.createElement('button'); copy.type = 'button'; copy.textContent = 'Copy prompt';
    copy.addEventListener('click', function () {
      try { navigator.clipboard.writeText(pre.textContent); copy.textContent = 'Copied'; } catch (e) { copy.textContent = 'Select the text and copy it'; }
    });
    var close = document.createElement('button'); close.type = 'button'; close.textContent = 'Close';
    close.addEventListener('click', function () { d.close(); d.remove(); });
    row.appendChild(copy); row.appendChild(close); d.appendChild(row);
    document.body.appendChild(d);
    if (d.showModal) d.showModal(); else d.setAttribute('open', '');
  }

  var current = null;
  function render(id) {
    var look = looks.filter(function (l) { return l.id === id; })[0] || looks[0];
    if (!look) return;
    current = look.id;
    app.textContent = '';
    app.appendChild(look.tpl.content.cloneNode(true));
    document.documentElement.setAttribute('data-look', look.id);
    bind(app, DATA);
    each(app.querySelectorAll('isekai-radar'), radar);
    each(app.querySelectorAll('isekai-icon'), icon);
    each(app.querySelectorAll('isekai-hero'), hero);
    each(app.querySelectorAll('isekai-look-button'), lookButton);
    try { localStorage.setItem('isekai-look', look.id); } catch (e) {}
  }
  var saved = null;
  try { saved = localStorage.getItem('isekai-look'); } catch (e) {}
  var forced = document.documentElement.getAttribute('data-default-look');
  var hasSaved = looks.some(function (l) { return l.id === saved; });
  render(document.documentElement.hasAttribute('data-force-look') || !hasSaved ? forced : saved);
})();
