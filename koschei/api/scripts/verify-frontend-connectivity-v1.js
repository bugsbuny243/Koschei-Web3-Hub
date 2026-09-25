'use strict';

const fs = require('node:fs');
const path = require('node:path');

const apiRoot = path.resolve(__dirname, '..');
const publicRoot = path.join(apiRoot, 'public');
const httpRoot = path.join(apiRoot, 'internal', 'http');

function walk(dir) {
  const out = [];
  for (const entry of fs.readdirSync(dir, {withFileTypes: true})) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) out.push(...walk(full));
    else out.push(full);
  }
  return out;
}

function rel(file) {
  return path.relative(apiRoot, file).split(path.sep).join('/');
}

function stripURL(raw) {
  return String(raw || '').trim().split('#')[0].split('?')[0];
}

function assetPathFromURL(raw) {
  const u = stripURL(raw);
  if (!u.startsWith('/')) return '';
  if (/^\/(?:js|css|sdk)\//.test(u) || /^\/(?:widget|agent-widget)\.js$/.test(u)) {
    return 'public' + u;
  }
  if (/\.(?:js|css|json|svg|png|webp|ico)$/i.test(u)) return 'public' + u;
  return '';
}

function pagePathFromRoute(raw, publicFiles, registeredRoutes) {
  const u = stripURL(raw);
  if (!u.startsWith('/') || u.startsWith('/api/') || u.startsWith('/fabric/')) return '';
  if (u === '/') return 'public/index.html';
  if (registeredRoutes.has(u)) return '__registered__';
  if (/\.[a-z0-9]+$/i.test(u)) {
    const candidate = 'public' + u;
    return publicFiles.has(candidate) ? candidate : '';
  }
  const clean = u.replace(/^\//, '').replace(/\/$/, '');
  const candidate = 'public/' + clean + '.html';
  return publicFiles.has(candidate) ? candidate : '';
}

function quotedStrings(source) {
  const out = [];
  for (const match of source.matchAll(/(["'`])((?:\\.|(?!\1).)*)\1/gs)) out.push(match[2]);
  return out;
}

const allPublic = walk(publicRoot)
  .filter(file => !file.includes(path.join('js', '__fixtures__')))
  .map(file => ({file, rel: rel(file), source: fs.readFileSync(file, 'utf8')}));

const publicFiles = new Set(allPublic.map(x => x.rel));
const html = allPublic.filter(x => x.rel.endsWith('.html'));
const js = allPublic.filter(x => x.rel.endsWith('.js'));
const css = allPublic.filter(x => x.rel.endsWith('.css'));
const runtimeAssets = [...js, ...css];

const goFiles = walk(httpRoot).filter(file => file.endsWith('.go') && !file.endsWith('_test.go'));
const goSource = goFiles.map(file => fs.readFileSync(file, 'utf8')).join('\n');
const registeredRoutes = new Set();
for (const match of goSource.matchAll(/["'`]((?:\/api\/|\/fabric\/|\/owner(?:\/|$)|\/docs\/)[^"'`\s]*)["'`]/g)) {
  const value = stripURL(match[1]);
  if (value) registeredRoutes.add(value);
}
for (const match of goSource.matchAll(/(?:HandleFunc|registerStaticFileAlias|registerCanonicalRedirect|registerScanModeRedirect)\([^\n]*?["'`]([^"'`]+)["'`]/g)) {
  const value = stripURL(match[1]);
  if (value.startsWith('/')) registeredRoutes.add(value);
}

const staticAliases = fs.readFileSync(path.join(httpRoot, 'static_aliases.go'), 'utf8');
for (const match of staticAliases.matchAll(/["'`]([^"'`]+)["'`]/g)) {
  const value = stripURL(match[1]);
  if (value.startsWith('/')) registeredRoutes.add(value);
}

const missingAssets = [];
const missingPages = [];
const apiRefsMissing = [];
const incoming = new Map(runtimeAssets.map(x => [x.rel, []]));

const runtimeGoFiles = walk(apiRoot).filter(file => file.endsWith('.go') && !file.endsWith('_test.go') && !file.startsWith(publicRoot));
for (const file of runtimeGoFiles) {
  const source = fs.readFileSync(file, 'utf8');
  for (const match of source.matchAll(/\/(?:js|css|sdk)\/[A-Za-z0-9_.\/-]+\.(?:js|css)|\/(?:widget|agent-widget)\.js/g)) {
    const raw = match[0];
    const asset = assetPathFromURL(raw);
    if (!asset) continue;
    if (!publicFiles.has(asset)) missingAssets.push({from: rel(file), ref: raw, expected: asset});
    else if (incoming.has(asset)) incoming.get(asset).push('go:' + rel(file));
  }
}

const backendRoutes = [...registeredRoutes].filter(x => x.startsWith('/api/') || x.startsWith('/fabric/'));
function backendRouteExists(raw) {
  let u = stripURL(raw);
  if (!u) return true;
  const marker = u.search(/[\${}:*]/);
  if (marker >= 0) u = u.slice(0, marker);
  if (!u) return true;
  return backendRoutes.some(route => {
    if (route === u) return true;
    if (route.endsWith('/') && u.startsWith(route)) return true;
    if (u.endsWith('/') && route.startsWith(u)) return true;
    return false;
  });
}

for (const item of allPublic) {
  const strings = quotedStrings(item.source);
  for (const raw of strings) {
    const asset = assetPathFromURL(raw);
    if (asset) {
      if (!publicFiles.has(asset)) missingAssets.push({from: item.rel, ref: raw, expected: asset});
      else if (incoming.has(asset)) incoming.get(asset).push(item.rel);
      continue;
    }

    const clean = stripURL(raw);
    if (clean.startsWith('/api/') || clean.startsWith('/fabric/')) {
      if (!backendRouteExists(clean)) apiRefsMissing.push({from: item.rel, ref: clean});
      continue;
    }

    if (item.rel.endsWith('.html') && clean.startsWith('/') && !clean.startsWith('//')) {
      const page = pagePathFromRoute(clean, publicFiles, registeredRoutes);
      if (!page && !/\.(?:txt|xml|map)$/i.test(clean)) missingPages.push({from: item.rel, ref: clean});
    }
  }

  if (item.rel.endsWith('.html')) {
    const ids = [...item.source.matchAll(/\bid=["']([^"']+)["']/g)].map(m => m[1]);
    const seen = new Set();
    for (const id of ids) {
      if (seen.has(id)) throw new Error(`Duplicate DOM id ${id} in ${item.rel}`);
      seen.add(id);
    }
  }
}

const explicitRuntimeEntrypoints = new Set([
  'public/widget.js',
  'public/agent-widget.js',
  'public/sdk/koschei-shield.js'
]);

const orphans = [];
for (const asset of runtimeAssets) {
  if (explicitRuntimeEntrypoints.has(asset.rel)) continue;
  if ((incoming.get(asset.rel) || []).length === 0) orphans.push(asset.rel);
}

const koscheiCSS = fs.readFileSync(path.join(publicRoot, 'css', 'koschei.css'), 'utf8');
if (koscheiCSS.includes('.page{display:none}.page.active{display:block}')) {
  throw new Error('Shared CSS still contains unscoped .page visibility ownership');
}

function uniqueRows(rows) {
  const seen = new Set();
  return rows.filter(row => {
    const key = JSON.stringify(row);
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

const report = {
  html_files: html.length,
  js_files: js.length,
  css_files: css.length,
  missing_assets: uniqueRows(missingAssets),
  missing_page_routes: uniqueRows(missingPages),
  missing_backend_routes: uniqueRows(apiRefsMissing),
  orphan_runtime_assets: orphans.sort()
};

console.log(JSON.stringify(report, null, 2));

const failures = [];
if (report.missing_assets.length) failures.push(`${report.missing_assets.length} missing local assets`);
if (report.missing_page_routes.length) failures.push(`${report.missing_page_routes.length} dead internal page links`);
if (report.missing_backend_routes.length) failures.push(`${report.missing_backend_routes.length} frontend backend-route mismatches`);
if (report.orphan_runtime_assets.length) failures.push(`${report.orphan_runtime_assets.length} orphan JS/CSS assets`);

if (failures.length) {
  throw new Error('Frontend connectivity audit failed: ' + failures.join(', '));
}
console.log('Frontend connectivity audit passed: every runtime asset and static contract is connected.');
