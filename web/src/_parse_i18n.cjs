
const fs = require('fs');
const content = fs.readFileSync('D:/Projeler/aurapanel-ols/web/src/i18n.js', 'utf8');

// Extract the object between 'const messages = ' and the next top-level assignment
const match = content.match(/const messages = (\{[\s\S]*?\n\})/);
if (!match) { console.error('No match'); process.exit(1); }

// eval the object (it's JS object literal, not JSON)
const messages = eval('(' + match[1] + ')');

// Now merge duplicate keys - but JS eval already took last-wins.
// We need to parse BOTH occurrences and merge them.
// Let's do it differently: find all namespace blocks and merge manually.

console.log(JSON.stringify(messages, null, 2));
