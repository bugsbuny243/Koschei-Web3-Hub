'use strict';

const assert = require('node:assert/strict');
const test = require('node:test');
const { expectedNavigationMarker } = require('./expected-unified-scan-navigation-marker.cjs');

test('versioned bootstrap produces the exact deployed asset marker', () => {
  assert.equal(
    expectedNavigationMarker("script.src='/js/unified-scan-navigation.js?v=2';"),
    "script.src='/js/unified-scan-navigation.js?v=2'"
  );
  assert.equal(
    expectedNavigationMarker('script.src="/js/unified-scan-navigation.js?v=12";'),
    'script.src="/js/unified-scan-navigation.js?v=12"'
  );
});

test('missing, unversioned, and duplicate bootstraps fail closed', () => {
  assert.throws(() => expectedNavigationMarker(''), /exactly one/);
  assert.throws(() => expectedNavigationMarker("script.src='/js/unified-scan-navigation.js';"), /exactly one/);
  const duplicate = "script.src='/js/unified-scan-navigation.js?v=1';\nscript.src='/js/unified-scan-navigation.js?v=2';";
  assert.throws(() => expectedNavigationMarker(duplicate), /found 2/);
});
